package commerce

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/techlane/techlane/packages/pkg/apierrors"
	"github.com/techlane/techlane/packages/pkg/authz"
	"github.com/techlane/techlane/packages/pkg/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.HandleFunc("GET /commerce/public/bootstrap", h.publicBootstrap)
	mux.HandleFunc("GET /commerce/public/catalog", h.publicCatalog)
	mux.HandleFunc("GET /commerce/public/storefront-content", h.publicStorefrontContent)
	mux.HandleFunc("POST /commerce/public/newsletter-subscribe", h.publicNewsletterSubscribe)
	mux.HandleFunc("POST /commerce/public/checkout", h.publicCheckout)
	mux.HandleFunc("GET /commerce/public/orders/{id}", h.publicGetOrder)
	mux.HandleFunc("GET /commerce/public/track", h.publicTrack)
	mux.HandleFunc("POST /commerce/public/accounts/register", h.publicAccountRegister)
	mux.HandleFunc("POST /commerce/public/accounts/login", h.publicAccountLogin)
	mux.HandleFunc("GET /commerce/public/account", h.publicAccount)
	mux.HandleFunc("GET /commerce/public/account/orders", h.publicAccountOrders)
	mux.HandleFunc("GET /commerce/public/account/repairs", h.publicAccountRepairs)
	mux.HandleFunc("POST /commerce/public/products/{variantID}/view", h.publicRecordView)
	mux.HandleFunc("GET /commerce/public/products/{productID}/reviews", h.publicListReviews)
	mux.HandleFunc("POST /commerce/public/products/{productID}/reviews", h.publicSubmitReview)
	mux.HandleFunc("GET /commerce/public/fx-rates", h.publicFXRates)

	mux.Handle("GET /commerce/catalog", auth(http.HandlerFunc(h.catalog)))
	mux.Handle("POST /commerce/checkout", auth(http.HandlerFunc(h.checkout)))
	mux.Handle("GET /commerce/orders", auth(http.HandlerFunc(h.listOrders)))
	mux.Handle("GET /commerce/orders/{id}", auth(http.HandlerFunc(h.getOrder)))
	mux.Handle("POST /commerce/orders/{id}/confirm-paid", auth(httpx.RequirePermission("orders.confirm_paid")(http.HandlerFunc(h.confirmPaid))))
	mux.Handle("POST /commerce/collect", auth(http.HandlerFunc(h.collect)))
	mux.Handle("POST /commerce/expire-holds", auth(http.HandlerFunc(h.expireHolds)))
	mux.Handle("POST /commerce/products/{id}/publish", auth(http.HandlerFunc(h.publish)))
	mux.Handle("GET /commerce/delivery-locations", auth(http.HandlerFunc(h.listDeliveryLocations)))
	mux.Handle("POST /commerce/delivery-locations", auth(http.HandlerFunc(h.createDeliveryLocation)))
	mux.Handle("PUT /commerce/delivery-locations/{id}", auth(http.HandlerFunc(h.updateDeliveryLocation)))
	mux.Handle("DELETE /commerce/delivery-locations/{id}", auth(http.HandlerFunc(h.deleteDeliveryLocation)))
}

func (h *Handler) catalog(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	var loc *uuid.UUID
	if s := r.URL.Query().Get("location_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid location_id", httpx.CorrelationID(r.Context()))
			return
		}
		loc = &id
	}
	items, err := h.svc.ListOnlineCatalog(r.Context(), claims.TenantID, loc)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) checkout(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	var req CheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid body", httpx.CorrelationID(r.Context()))
		return
	}
	req.ActorID = claims.UserID
	req.CorrID = corrID(r)
	result, err := h.svc.StartCheckout(r.Context(), claims.TenantID, req)
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "CHECKOUT_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, result)
}

func (h *Handler) listOrders(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	items, err := h.svc.ListOrders(r.Context(), claims.TenantID, r.URL.Query().Get("status"))
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) getOrder(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	order, err := h.svc.GetOrder(r.Context(), claims.TenantID, id)
	if err != nil {
		apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, order)
}

func (h *Handler) confirmPaid(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.ManualConfirmPaid(r.Context(), claims.TenantID, id, claims.UserID); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "CONFIRM_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	order, _ := h.svc.GetOrder(r.Context(), claims.TenantID, id)
	httpx.JSON(w, http.StatusOK, order)
}

func (h *Handler) collect(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	var body struct {
		CollectionCode string `json:"collection_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.CollectionCode == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "collection_code required", httpx.CorrelationID(r.Context()))
		return
	}
	order, err := h.svc.CollectInBranch(r.Context(), claims.TenantID, body.CollectionCode, claims.UserID)
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "COLLECT_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, order)
}

func (h *Handler) expireHolds(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	if !claims.HasPermission("inventory.adjust") && !claims.HasPermission("sales.create") && !claims.HasPermission("*") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "permission required", httpx.CorrelationID(r.Context()))
		return
	}
	n, err := h.svc.ExpireHolds(r.Context(), claims.TenantID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"released": n})
}

func (h *Handler) publish(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	published := true
	var body struct {
		Published *bool `json:"published"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.Published != nil {
		published = *body.Published
	}
	if err := h.svc.SetProductPublished(r.Context(), claims.TenantID, id, published); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "PUBLISH_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	status := "unpublished"
	if published {
		status = "published"
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": status})
}

func (h *Handler) publicBootstrap(w http.ResponseWriter, r *http.Request) {
	boot, err := h.svc.PublicBootstrap(r.Context())
	if err != nil {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, boot)
}

func (h *Handler) publicCatalog(w http.ResponseWriter, r *http.Request) {
	boot, err := h.svc.PublicBootstrap(r.Context())
	if err != nil {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	loc := boot.LocationID
	if s := r.URL.Query().Get("location_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid location_id", httpx.CorrelationID(r.Context()))
			return
		}
		loc = id
	}
	items, err := h.svc.ListOnlineCatalog(r.Context(), boot.TenantID, &loc)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) publicStorefrontContent(w http.ResponseWriter, r *http.Request) {
	boot, err := h.svc.PublicBootstrap(r.Context())
	if err != nil {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	loc := boot.LocationID
	if s := r.URL.Query().Get("location_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid location_id", httpx.CorrelationID(r.Context()))
			return
		}
		loc = id
	}
	content, err := h.svc.PublicStorefrontContent(r.Context(), boot.TenantID, &loc)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, content)
}

func (h *Handler) publicNewsletterSubscribe(w http.ResponseWriter, r *http.Request) {
	boot, err := h.svc.PublicBootstrap(r.Context())
	if err != nil {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid body", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.SubscribeNewsletter(r.Context(), boot.TenantID, req.Email); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "SUBSCRIBE_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "subscribed"})
}

func (h *Handler) publicCheckout(w http.ResponseWriter, r *http.Request) {
	boot, err := h.svc.PublicBootstrap(r.Context())
	if err != nil {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	var req CheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid body", httpx.CorrelationID(r.Context()))
		return
	}
	if req.BranchID == uuid.Nil {
		req.BranchID = boot.BranchID
	}
	if req.LocationID == uuid.Nil {
		req.LocationID = boot.LocationID
	}
	if req.Method == "" {
		req.Method = "mpesa_stk"
	}
	// Ensure the customer can only reserve stock at a real branch for this shop.
	branchOK := false
	for _, br := range boot.Branches {
		if br.ID == req.BranchID && br.LocationID == req.LocationID {
			branchOK = true
			break
		}
	}
	if !branchOK && req.BranchID == boot.BranchID && req.LocationID == boot.LocationID {
		branchOK = true
	}
	if !branchOK {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid branch or location", httpx.CorrelationID(r.Context()))
		return
	}
	if token := bearerToken(r); token != "" {
		account, authErr := h.svc.AuthenticateCustomer(r.Context(), boot.TenantID, token)
		if authErr != nil {
			apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", authErr.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		req.CustomerID = &account.ID
		if req.Phone == "" && account.Phone != nil {
			req.Phone = *account.Phone
		}
	} else {
		// Guest checkout must not be able to attach an arbitrary customer record.
		req.CustomerID = nil
	}
	// Fulfilment comes from the client (branch_pickup | delivery); StartCheckout validates.
	req.ActorID = uuid.Nil
	req.CorrID = corrID(r)
	result, err := h.svc.StartCheckout(r.Context(), boot.TenantID, req)
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "CHECKOUT_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, result)
}

func (h *Handler) publicGetOrder(w http.ResponseWriter, r *http.Request) {
	boot, err := h.svc.PublicBootstrap(r.Context())
	if err != nil {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	order, err := h.svc.GetOrder(r.Context(), boot.TenantID, id)
	if err != nil {
		apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", "order not found", httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, order)
}

func (h *Handler) publicTrack(w http.ResponseWriter, r *http.Request) {
	boot, err := h.svc.PublicBootstrap(r.Context())
	if err != nil {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		q = strings.TrimSpace(r.URL.Query().Get("query"))
	}
	if q == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "enter an order number, job code, or phone number", httpx.CorrelationID(r.Context()))
		return
	}
	result, err := h.svc.PublicTrack(r.Context(), boot.TenantID, q)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "TRACK_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (h *Handler) publicAccountRegister(w http.ResponseWriter, r *http.Request) {
	boot, err := h.svc.PublicBootstrap(r.Context())
	if err != nil {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		FullName string `json:"full_name"`
		Email    string `json:"email"`
		Phone    string `json:"phone"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid body", httpx.CorrelationID(r.Context()))
		return
	}
	session, err := h.svc.RegisterCustomerAccount(
		r.Context(), boot.TenantID, req.FullName, req.Email, req.Phone, req.Password,
	)
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "REGISTER_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, session)
}

func (h *Handler) publicAccountLogin(w http.ResponseWriter, r *http.Request) {
	boot, err := h.svc.PublicBootstrap(r.Context())
	if err != nil {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid body", httpx.CorrelationID(r.Context()))
		return
	}
	session, err := h.svc.LoginCustomerAccount(r.Context(), boot.TenantID, req.Email, req.Password)
	if err != nil {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, session)
}

func (h *Handler) publicAccount(w http.ResponseWriter, r *http.Request) {
	account, ok := h.authenticatedPublicAccount(w, r)
	if !ok {
		return
	}
	httpx.JSON(w, http.StatusOK, account)
}

func (h *Handler) publicAccountOrders(w http.ResponseWriter, r *http.Request) {
	boot, err := h.svc.PublicBootstrap(r.Context())
	if err != nil {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	account, err := h.svc.AuthenticateCustomer(r.Context(), boot.TenantID, bearerToken(r))
	if err != nil {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	items, err := h.svc.ListCustomerOrders(r.Context(), boot.TenantID, account.ID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) publicAccountRepairs(w http.ResponseWriter, r *http.Request) {
	boot, err := h.svc.PublicBootstrap(r.Context())
	if err != nil {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	account, err := h.svc.AuthenticateCustomer(r.Context(), boot.TenantID, bearerToken(r))
	if err != nil {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	phone := ""
	if account.Phone != nil {
		phone = *account.Phone
	}
	items, err := h.svc.ListAccountRepairs(r.Context(), boot.TenantID, account.ID, phone)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) publicRecordView(w http.ResponseWriter, r *http.Request) {
	boot, err := h.svc.PublicBootstrap(r.Context())
	if err != nil {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	variantID, err := uuid.Parse(r.PathValue("variantID"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid variant id", httpx.CorrelationID(r.Context()))
		return
	}
	// Fire-and-forget: a tracking failure must never break the product page.
	_ = h.svc.RecordProductView(r.Context(), boot.TenantID, variantID)
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "recorded"})
}

func (h *Handler) publicListReviews(w http.ResponseWriter, r *http.Request) {
	boot, err := h.svc.PublicBootstrap(r.Context())
	if err != nil {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	productID, err := uuid.Parse(r.PathValue("productID"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid product id", httpx.CorrelationID(r.Context()))
		return
	}
	items, err := h.svc.ListProductReviews(r.Context(), boot.TenantID, productID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) publicSubmitReview(w http.ResponseWriter, r *http.Request) {
	boot, err := h.svc.PublicBootstrap(r.Context())
	if err != nil {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	productID, err := uuid.Parse(r.PathValue("productID"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid product id", httpx.CorrelationID(r.Context()))
		return
	}
	account, err := h.svc.AuthenticateCustomer(r.Context(), boot.TenantID, bearerToken(r))
	if err != nil {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		Rating int    `json:"rating"`
		Title  string `json:"title"`
		Body   string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid body", httpx.CorrelationID(r.Context()))
		return
	}
	review, err := h.svc.SubmitProductReview(r.Context(), boot.TenantID, account.ID, productID, req.Rating, req.Title, req.Body)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "collected") {
			status = http.StatusForbidden
		}
		apierrors.Write(w, status, "REVIEW_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, review)
}

func (h *Handler) publicFXRates(w http.ResponseWriter, r *http.Request) {
	boot, err := h.svc.PublicBootstrap(r.Context())
	if err != nil {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	rates, err := h.svc.FXRates(r.Context(), boot.TenantID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, rates)
}

func (h *Handler) listDeliveryLocations(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	items, err := h.svc.ListDeliveryLocations(r.Context(), claims.TenantID, false)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) createDeliveryLocation(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	var in DeliveryLocationInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid body", httpx.CorrelationID(r.Context()))
		return
	}
	loc, err := h.svc.CreateDeliveryLocation(r.Context(), claims.TenantID, in)
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, loc)
}

func (h *Handler) updateDeliveryLocation(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var in DeliveryLocationInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid body", httpx.CorrelationID(r.Context()))
		return
	}
	loc, err := h.svc.UpdateDeliveryLocation(r.Context(), claims.TenantID, id, in)
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, loc)
}

func (h *Handler) deleteDeliveryLocation(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.DeleteDeliveryLocation(r.Context(), claims.TenantID, id); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) authenticatedPublicAccount(w http.ResponseWriter, r *http.Request) (*CustomerAccount, bool) {
	boot, err := h.svc.PublicBootstrap(r.Context())
	if err != nil {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", err.Error(), httpx.CorrelationID(r.Context()))
		return nil, false
	}
	account, err := h.svc.AuthenticateCustomer(r.Context(), boot.TenantID, bearerToken(r))
	if err != nil {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error(), httpx.CorrelationID(r.Context()))
		return nil, false
	}
	return account, true
}

func bearerToken(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(value) > 7 && strings.EqualFold(value[:7], "Bearer ") {
		return strings.TrimSpace(value[7:])
	}
	return ""
}

func corrID(r *http.Request) uuid.UUID {
	if id := httpx.CorrelationID(r.Context()); id != "" {
		if u, err := uuid.Parse(id); err == nil {
			return u
		}
	}
	return uuid.New()
}
