package sync_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/techlane/techlane/internal/identity"
	"github.com/techlane/techlane/internal/inventory"
	"github.com/techlane/techlane/internal/payments"
	"github.com/techlane/techlane/internal/platform"
	"github.com/techlane/techlane/internal/repair"
	"github.com/techlane/techlane/internal/sync"
	"github.com/techlane/techlane/packages/pkg/db"
	"github.com/techlane/techlane/packages/pkg/events"
)

type testEnv struct {
	pool     *pgxpool.Pool
	idSvc    *identity.Service
	syncSvc  *sync.Service
	tenantID uuid.UUID
	userID   uuid.UUID
	branchID uuid.UUID
	deviceID uuid.UUID
	repairID uuid.UUID
}

func setupSyncTest(t *testing.T) (*testEnv, context.Context) {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://techlane:techlane@localhost:5432/techlane?sslmode=disable"
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}
	if err := platform.EnsureSchemas(ctx, pool); err != nil {
		t.Fatalf("schemas: %v", err)
	}
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	if err := platform.RunMigrations(ctx, pool, repoRoot); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	var tenantID, userID, branchID uuid.UUID
	err = pool.QueryRow(ctx, `
		SELECT t.id, u.id, b.id
		FROM identity.tenants t
		JOIN identity.users u ON u.tenant_id = t.id
		JOIN identity.branches b ON b.tenant_id = t.id
		ORDER BY t.created_at, u.created_at, b.created_at
		LIMIT 1`).Scan(&tenantID, &userID, &branchID)
	if err != nil {
		t.Fatalf("seed data: %v", err)
	}

	idSvc := identity.NewService(pool, "test-secret-32-characters-minimum!!")
	repairSvc := repair.NewService(pool, events.NewBus())
	invSvc := inventory.NewService(pool)
	paySvc := payments.NewService(pool)
	syncSvc := sync.NewService(pool, repairSvc, invSvc, paySvc, idSvc)

	deviceID := uuid.New()
	if _, err := idSvc.RegisterDevice(ctx, tenantID, userID, identity.RegisterDeviceInput{
		ID: &deviceID, DeviceName: strPtr("test-handset"), Platform: strPtr("android"),
	}); err != nil {
		t.Fatalf("register device: %v", err)
	}

	customerDevice, err := repairSvc.CreateDevice(ctx, repair.CreateDeviceInput{
		Kind: "phone", Anonymous: true, ActorID: userID, TenantID: tenantID, CorrID: uuid.New(),
	})
	if err != nil {
		t.Fatalf("create device: %v", err)
	}

	repairJobID := uuid.New()
	job, err := repairSvc.CreateRepair(ctx, repair.CreateRepairInput{
		BranchID: branchID, DeviceID: customerDevice.ID, ProblemSummary: "sync test",
		ActorID: userID, TenantID: tenantID, ClientID: &repairJobID, CorrID: uuid.New(),
	})
	if err != nil {
		t.Fatalf("create repair: %v", err)
	}

	return &testEnv{
		pool: pool, idSvc: idSvc, syncSvc: syncSvc, tenantID: tenantID, userID: userID,
		branchID: branchID, deviceID: deviceID, repairID: job.ID,
	}, ctx
}

func strPtr(s string) *string { return &s }

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

func TestSyncIdempotentReplay(t *testing.T) {
	env, ctx := setupSyncTest(t)
	defer env.pool.Close()

	customerDevice, err := repair.NewService(env.pool, events.NewBus()).CreateDevice(ctx, repair.CreateDeviceInput{
		Kind: "phone", Anonymous: true, ActorID: env.userID, TenantID: env.tenantID, CorrID: uuid.New(),
	})
	if err != nil {
		t.Fatalf("device: %v", err)
	}

	actionID := uuid.New()
	payload := map[string]any{
		"branch_id":       env.branchID.String(),
		"device_id":       customerDevice.ID.String(),
		"problem_summary": "offline replay test",
	}
	in := sync.CommandInput{
		ActionID: actionID, TenantID: env.tenantID, BranchID: &env.branchID,
		DeviceID: &env.deviceID, UserID: env.userID, CommandType: "repair.create_draft",
		Payload: payload,
	}

	first, err := env.syncSvc.Submit(ctx, in)
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}
	if first.SyncStatus != "applied" {
		t.Fatalf("expected applied, got %s (%s)", first.SyncStatus, first.Error)
	}

	second, err := env.syncSvc.Submit(ctx, in)
	if err != nil {
		t.Fatalf("replay submit: %v", err)
	}
	if second.SyncStatus != "applied" {
		t.Fatalf("expected applied replay, got %s", second.SyncStatus)
	}
	if first.Result["repair_job_id"] != second.Result["repair_job_id"] {
		t.Fatalf("repair_job_id mismatch on replay")
	}
}

func TestSyncPayloadMismatch(t *testing.T) {
	env, ctx := setupSyncTest(t)
	defer env.pool.Close()

	actionID := uuid.New()
	base := sync.CommandInput{
		ActionID: actionID, TenantID: env.tenantID, BranchID: &env.branchID,
		DeviceID: &env.deviceID, UserID: env.userID, CommandType: "repair.add_note",
		Payload: map[string]any{
			"repair_job_id": env.repairID.String(),
			"note":          "first note",
		},
	}
	if _, err := env.syncSvc.Submit(ctx, base); err != nil {
		t.Fatalf("first submit: %v", err)
	}

	changed := base
	changed.Payload = map[string]any{
		"repair_job_id": env.repairID.String(),
		"note":          "different note",
	}
	_, err := env.syncSvc.Submit(ctx, changed)
	if err == nil {
		t.Fatal("expected payload mismatch error")
	}
	if err != sync.ErrPayloadMismatch {
		t.Fatalf("expected ErrPayloadMismatch, got %v", err)
	}
}

func TestSyncCashNoDoubleCharge(t *testing.T) {
	env, ctx := setupSyncTest(t)
	defer env.pool.Close()

	actionID := uuid.New()
	payload := map[string]any{
		"branch_id":    env.branchID.String(),
		"payable_type": "repair",
		"payable_id":   env.repairID.String(),
		"amount":       500.0,
	}
	in := sync.CommandInput{
		ActionID: actionID, TenantID: env.tenantID, BranchID: &env.branchID,
		DeviceID: &env.deviceID, UserID: env.userID, CommandType: "payments.cash_provisional",
		Payload: payload,
	}

	first, err := env.syncSvc.Submit(ctx, in)
	if err != nil {
		t.Fatalf("first cash submit: %v", err)
	}
	second, err := env.syncSvc.Submit(ctx, in)
	if err != nil {
		t.Fatalf("replay cash submit: %v", err)
	}
	if first.Result["payment_id"] != second.Result["payment_id"] {
		t.Fatalf("payment_id changed on replay")
	}
}

func TestSyncRevokedDeviceBlocked(t *testing.T) {
	env, ctx := setupSyncTest(t)
	defer env.pool.Close()

	if err := env.idSvc.RevokeDevice(ctx, env.tenantID, env.userID, env.deviceID); err != nil {
		t.Fatalf("revoke device: %v", err)
	}

	_, err := env.syncSvc.Submit(ctx, sync.CommandInput{
		ActionID: uuid.New(), TenantID: env.tenantID, BranchID: &env.branchID,
		DeviceID: &env.deviceID, UserID: env.userID, CommandType: "repair.add_note",
		Payload: map[string]any{
			"repair_job_id": env.repairID.String(),
			"note":          "should be blocked",
		},
	})
	if err == nil {
		t.Fatal("expected revoked device error")
	}
	if err != sync.ErrDeviceRevoked {
		t.Fatalf("expected ErrDeviceRevoked, got %v", err)
	}
}
