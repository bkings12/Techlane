package appversion

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

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

// Register mounts the public version-check endpoint directly on mux (no auth
// required — the app needs this before/without a signed-in session) and the
// owner-only publish/list endpoints behind auth.
func (h *Handler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.HandleFunc("GET /app-version", h.latest)
	mux.Handle("POST /app-releases", auth(http.HandlerFunc(h.publish)))
	mux.Handle("GET /app-releases", auth(http.HandlerFunc(h.list)))
}

func canManageReleases(claims *authz.Claims) bool {
	if claims == nil {
		return false
	}
	if claims.HasPermission("*") {
		return true
	}
	for _, role := range claims.Roles {
		if role == "owner" {
			return true
		}
	}
	return false
}

func (h *Handler) latest(w http.ResponseWriter, r *http.Request) {
	app := strings.TrimSpace(r.URL.Query().Get("app"))
	platform := strings.TrimSpace(r.URL.Query().Get("platform"))
	if app == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "app query param required", httpx.CorrelationID(r.Context()))
		return
	}
	if platform == "" {
		platform = "android"
	}
	rel, err := h.svc.Latest(r.Context(), app, platform)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	if rel == nil {
		httpx.JSON(w, http.StatusOK, map[string]any{
			"app": app, "platform": platform, "update_available": false, "force_update": false,
		})
		return
	}
	currentCode, _ := strconv.Atoi(r.URL.Query().Get("current_version_code"))
	updateAvailable := currentCode > 0 && rel.VersionCode > currentCode
	forceUpdate := currentCode > 0 && currentCode < rel.MinSupportedVersionCode
	httpx.JSON(w, http.StatusOK, map[string]any{
		"app":                        rel.App,
		"platform":                   rel.Platform,
		"latest_version_code":        rel.VersionCode,
		"latest_version_name":        rel.VersionName,
		"min_supported_version_code": rel.MinSupportedVersionCode,
		"download_url":               rel.DownloadURL,
		"notes":                      rel.Notes,
		"update_available":           updateAvailable,
		"force_update":               forceUpdate,
	})
}

func (h *Handler) publish(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok || !canManageReleases(claims) {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "owner role required", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		App                     string `json:"app"`
		Platform                string `json:"platform"`
		VersionCode             int    `json:"version_code"`
		VersionName             string `json:"version_name"`
		MinSupportedVersionCode int    `json:"min_supported_version_code"`
		DownloadURL             string `json:"download_url"`
		Notes                   string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.App == "" || req.VersionCode <= 0 || req.VersionName == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "app, version_code, version_name required", httpx.CorrelationID(r.Context()))
		return
	}
	if req.Platform == "" {
		req.Platform = "android"
	}
	rel, err := h.svc.Publish(r.Context(), PublishInput{
		App: req.App, Platform: req.Platform, VersionCode: req.VersionCode, VersionName: req.VersionName,
		MinSupportedVersionCode: req.MinSupportedVersionCode, DownloadURL: req.DownloadURL, Notes: req.Notes,
		ActorID: claims.UserID,
	})
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, rel)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok || !canManageReleases(claims) {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "owner role required", httpx.CorrelationID(r.Context()))
		return
	}
	items, err := h.svc.ListReleases(r.Context(), r.URL.Query().Get("app"))
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}
