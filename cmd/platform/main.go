package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/techlane/techlane/internal/appversion"
	"github.com/techlane/techlane/internal/audit"
	"github.com/techlane/techlane/internal/commerce"
	"github.com/techlane/techlane/internal/identity"
	"github.com/techlane/techlane/internal/inventory"
	"github.com/techlane/techlane/internal/loyalty"
	"github.com/techlane/techlane/internal/notify"
	"github.com/techlane/techlane/internal/payments"
	"github.com/techlane/techlane/internal/platform"
	"github.com/techlane/techlane/internal/realtime"
	"github.com/techlane/techlane/internal/receipts"
	"github.com/techlane/techlane/internal/repair"
	"github.com/techlane/techlane/internal/reporting"
	"github.com/techlane/techlane/internal/sales"
	"github.com/techlane/techlane/internal/search"
	"github.com/techlane/techlane/internal/storefrontcms"
	"github.com/techlane/techlane/internal/sync"
	"github.com/techlane/techlane/internal/whatsapp"
	"github.com/techlane/techlane/internal/worker"
	"github.com/techlane/techlane/packages/pkg/config"
	"github.com/techlane/techlane/packages/pkg/db"
	"github.com/techlane/techlane/packages/pkg/events"
	"github.com/techlane/techlane/packages/pkg/httpx"
	"github.com/techlane/techlane/packages/pkg/mailer"
	"github.com/techlane/techlane/packages/pkg/objectstore"
	"github.com/techlane/techlane/packages/pkg/otel"
)

func main() {
	ctx := context.Background()
	otel.SetupJSONLogger()
	databaseURL := config.Env("DATABASE_URL", "postgres://techlane:techlane@localhost:5432/techlane?sslmode=disable")
	jwtSecret := config.Env("JWT_SECRET", "dev-change-me-in-production-32chars")
	addr := config.Env("HTTP_ADDR", ":8080")

	if err := platform.ValidateProductionSecrets(jwtSecret); err != nil {
		log.Fatalf("security: %v", err)
	}

	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	if err := platform.EnsureSchemas(ctx, pool); err != nil {
		log.Fatalf("schemas: %v", err)
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		log.Fatalf("repo root: %v", err)
	}
	if err := platform.RunMigrations(ctx, pool, repoRoot); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	bus := events.NewBus()
	auditSvc := audit.NewService(pool)
	httpx.SetErrorSink(auditSvc)
	bus.Subscribe("*", func(e events.Envelope) {
		actor := e.ActorID
		_ = auditSvc.Record(context.Background(), e.TenantID, actor, e.EventType, "event", &e.EventID, e.Payload, e.CorrelationID)
	})

	realtimeHub := realtime.NewHub(jwtSecret)
	bus.Subscribe("*", realtimeHub.Broadcast)

	idSvc := identity.NewService(pool, jwtSecret)
	idSvc.SetAuditSink(auditSvc)
	if err := idSvc.Start(ctx); err != nil {
		log.Fatalf("identity seed: %v", err)
	}

	invSvc := inventory.NewService(pool)
	invSvc.SetEventBus(bus)
	invSvc.SetAlertHook(auditSvc)
	paySvc := payments.NewService(pool)
	paySvc.SetEventBus(bus)
	paySvc.SetRiskHook(audit.PaymentsRiskAdapter{Svc: auditSvc})
	repairSvc := repair.NewService(pool, bus)
	repairSvc.SetPasscodeKey([]byte(config.Env("DEVICE_PASSCODE_KEY", jwtSecret)))
	repairSvc.SetCommissionHook(identity.RepairCommissionAdapter{Svc: idSvc})
	repairSvc.SetCompletionHook(audit.RepairCompletionAdapter{Svc: auditSvc})
	repairSvc.SetStockDeductor(inventory.RepairStockAdapter{Svc: invSvc})
	// Parts consumed on a job book their cost onto that job, whether they came from
	// a supplier or off the shop's own shelf.
	invSvc.SetJobCostHook(repair.PartCostAdapter{Svc: repairSvc})
	searchSvc := search.NewService(pool)
	searchHandler := search.NewHandler(searchSvc)
	appVersionHandler := appversion.NewHandler(appversion.NewService(pool))
	loyaltySvc := loyalty.NewService(pool)
	loyaltySvc.SetEventBus(bus)
	loyaltyHandler := loyalty.NewHandler(loyaltySvc)
	receiptSvc := receipts.NewService(pool)
	storefrontSvc := storefrontcms.NewService(pool)
	objCfg := objectstore.ConfigFromEnv()
	if store, err := objectstore.New(objCfg); err != nil {
		log.Printf("object storage: %v (attachments fall back to database)", err)
	} else if store != nil {
		repairSvc.SetObjectStore(store)
		receiptSvc.SetObjectStore(store)
		storefrontSvc.SetObjectStore(store)
		invSvc.SetObjectStore(store)
		log.Printf("object storage: enabled bucket=%s endpoint=%s", objCfg.Bucket, objCfg.Endpoint)
	} else {
		log.Printf("object storage: not configured (set OBJECT_STORAGE_* for Cloudflare R2 / MinIO)")
	}
	if apiKey := config.Env("BLESSEDTEXTS_API_KEY", ""); apiKey != "" {
		senderID := config.Env("BLESSEDTEXTS_SENDER_ID", "")
		baseURL := config.Env("BLESSEDTEXTS_BASE_URL", "")
		repairSvc.SetSMSSender(repair.NewBlessedTextsSMSSender(apiKey, senderID, baseURL))
		log.Printf("sms: BlessedTexts env fallback configured (sender_id=%s)", senderID)
	} else {
		appEnv := strings.ToLower(config.Env("APP_ENV", "development"))
		if appEnv == "production" {
			log.Printf("sms: WARNING — no BLESSEDTEXTS_* env; OTP requires tenant Settings → SMS (OTP codes are never logged)")
		} else {
			log.Printf("sms: tenant Settings → SMS (OTP) or BLESSEDTEXTS_* env required (OTP codes are never logged)")
		}
	}
	salesSvc := sales.NewService(pool, invSvc)
	salesSvc.SetPaymentTaker(payments.SalePaymentAdapter{Svc: paySvc})
	paySvc.SetQuickSaleCreator(payments.SalesQuickSaleAdapter{Svc: salesSvc})
	syncSvc := sync.NewService(pool, repairSvc, invSvc, paySvc, idSvc)
	commerceSvc := commerce.NewService(pool, invSvc, storefrontSvc)
	commerceSvc.SetPaymentTaker(payments.OrderPaymentAdapter{Svc: paySvc})
	notifySvc := notify.NewService(pool)
	commerceSvc.SetOrderPlacedNotifier(commerce.NotifyAdapter{Svc: notifySvc})
	paySvc.SetOrderPaidHook(payments.CommercePaidAdapter{Svc: commerceSvc})
	paySvc.SetRepairPaidHook(payments.RepairSettledAdapter{Svc: repairSvc})
	paySvc.SetOutstandingResolver(func(ctx context.Context, tenantID uuid.UUID, payableType string, payableID uuid.UUID) (float64, bool, error) {
		if payableType != "repair" {
			return 0, false, nil
		}
		return repairSvc.RepairOutstanding(ctx, tenantID, payableID)
	})

	notifySvc.SetSMSResolver(func(ctx context.Context, tenantID uuid.UUID) notify.SMSSender {
		return repairSvc.ResolveSMSSender(ctx, tenantID)
	})
	waClient := whatsapp.NewClient(config.Env("WHATSAPP_SERVICE_URL", ""), config.Env("WHATSAPP_SERVICE_SECRET", ""))
	waSvc := whatsapp.NewService(pool, waClient)
	waSvc.SetRepairActions(repairSvc)
	waSvc.SetSupplierActions(invSvc)
	notifySvc.SetWhatsApp(waSvc)
	if waClient.Configured() {
		log.Printf("whatsapp: sidecar configured at %s", waClient.BaseURL)
	} else {
		log.Printf("whatsapp: not configured (set WHATSAPP_SERVICE_URL + WHATSAPP_SERVICE_SECRET)")
	}
	notifySvc.Wire(bus, repair.NotifyAdapter{Svc: repairSvc})

	// Seed inventory supplier for default tenant
	if err := seedInventory(ctx, pool, invSvc); err != nil {
		log.Printf("inventory seed: %v", err)
	}

	// Periodic risk scans + STK reconciliation for all tenants
	go func() {
		scanner := worker.NewRiskScanner(pool, invSvc, auditSvc)
		run := func() {
			rows, err := pool.Query(context.Background(), `SELECT id FROM identity.tenants ORDER BY created_at`)
			if err != nil {
				return
			}
			defer rows.Close()
			for rows.Next() {
				var tenantID uuid.UUID
				if err := rows.Scan(&tenantID); err != nil {
					continue
				}
				_, _ = scanner.ScanAll(context.Background(), tenantID)
				_, _ = paySvc.ReconcilePendingSTK(context.Background(), tenantID)
				_, _ = repairSvc.SendCreditReminders(context.Background(), tenantID)
			}
		}
		run()
		t := time.NewTicker(2 * time.Minute)
		defer t.Stop()
		for range t.C {
			run()
		}
	}()

	// Notification outbox drain
	go func() {
		drain := func() {
			_, _, _, _ = notifySvc.DrainPending(context.Background())
		}
		drain()
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for range t.C {
			drain()
		}
	}()

	mailCfg := mailer.ConfigFromEnv()
	mailSvc := mailer.New(mailCfg)
	webBaseURL := strings.TrimRight(config.Env("WEB_OPS_BASE_URL", "http://localhost:5173"), "/")
	if mailCfg.Configured() {
		log.Printf("email: SMTP configured (host=%s) for password resets", mailCfg.Host)
	} else {
		log.Printf("email: SMTP_HOST/SMTP_USERNAME not set — forgot-password links are logged, not emailed")
	}

	idHandler := identity.NewHandler(idSvc)
	idHandler.SetSignupEnabled(strings.ToLower(config.Env("SIGNUP_ENABLED", "true")) != "false")
	idHandler.SetPasswordResetNotifier(func(email, displayName, token string) {
		link := fmt.Sprintf("%s/reset-password?token=%s", webBaseURL, token)
		body := fmt.Sprintf(
			"Hi %s,\n\nUse the link below to reset your TechLane password. It expires in 30 minutes.\n\n%s\n\nIf you didn't request this, you can safely ignore this email.",
			displayName, link,
		)
		if err := mailSvc.Send(email, "Reset your TechLane password", body); err != nil {
			slog.Error("password reset email failed", "err", err, "email", email)
		}
	})
	repairHandler := repair.NewHandler(repairSvc)
	repairHandler.SetReceiptRenderer(receiptSvc)
	receiptHandler := receipts.NewHandler(receiptSvc)
	invHandler := inventory.NewHandler(invSvc)
	invHandler.SetReceiptRenderer(receiptSvc)
	payHandler := payments.NewHandler(paySvc)
	payHandler.SetCustomerRepairGateway(payments.RepairCustomerAdapter{
		DefaultTenant: repairSvc.DefaultTenantID,
		Authenticate: func(ctx context.Context, tenantID uuid.UUID, token string) (uuid.UUID, *string, error) {
			c, err := repairSvc.AuthenticateCustomer(ctx, tenantID, token)
			if err != nil {
				return uuid.Nil, nil, err
			}
			return c.ID, c.Phone, nil
		},
		AssertOwnsRepair: repairSvc.AssertCustomerOwnsRepair,
		PaymentContext:   repairSvc.RepairPaymentContext,
	})
	salesHandler := sales.NewHandler(salesSvc)
	salesHandler.SetReceiptRenderer(receiptSvc)
	auditHandler := audit.NewHandler(auditSvc)
	syncHandler := sync.NewHandler(syncSvc)
	commerceHandler := commerce.NewHandler(commerceSvc)
	storefrontHandler := storefrontcms.NewHandler(storefrontSvc)
	reportHandler := reporting.NewHandler(reporting.NewService(pool))
	notifyHandler := notify.NewHandler(notifySvc)
	notifyHandler.SetRepairLookup(repair.NotifyAdapter{Svc: repairSvc})
	waHandler := whatsapp.NewHandler(waSvc)

	auth := httpx.AuthMiddleware(jwtSecret)

	api := http.NewServeMux()
	api.HandleFunc("GET /health", healthHandler)
	api.HandleFunc("GET /ready", readyHandler(pool))
	api.HandleFunc("GET /metrics", httpx.MetricsHandler)
	api.HandleFunc("GET /events/stream", realtimeHub.ServeSSE)
	idHandler.Register(api, auth)
	repairHandler.Register(api, auth)
	searchHandler.Register(api, auth)
	appVersionHandler.Register(api, auth)
	invHandler.Register(api, auth)
	payHandler.Register(api, auth)
	salesHandler.Register(api, auth)
	auditHandler.Register(api, auth)
	syncHandler.Register(api, auth)
	commerceHandler.Register(api, auth)
	storefrontHandler.Register(api, auth)
	reportHandler.Register(api, auth)
	notifyHandler.Register(api, auth)
	loyaltyHandler.Register(api, auth)
	receiptHandler.Register(api, auth)
	waHandler.Register(api, auth)

	root := http.NewServeMux()
	root.HandleFunc("GET /health", healthHandler)
	root.HandleFunc("GET /ready", readyHandler(pool))
	root.HandleFunc("GET /metrics", httpx.MetricsHandler)
	root.Handle("/api/v1/", http.StripPrefix("/api/v1", api))
	root.Handle("/api/v1", http.StripPrefix("/api/v1", api))

	handler := platform.CORSMiddleware(httpx.MetricsMiddleware(httpx.CorrelationMiddleware(root)))

	srv := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		slog.Info("platform listening", "addr", addr, "app_env", config.Env("APP_ENV", "development"))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func readyHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "not_ready", "error": "database"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	}
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

func seedInventory(ctx context.Context, pool *pgxpool.Pool, inv *inventory.Service) error {
	var tenantID uuid.UUID
	err := pool.QueryRow(ctx, `SELECT id FROM identity.tenants ORDER BY created_at LIMIT 1`).Scan(&tenantID)
	if err != nil {
		return err
	}
	return inv.Start(ctx, tenantID)
}
