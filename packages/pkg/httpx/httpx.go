package httpx

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/techlane/techlane/packages/pkg/apierrors"
	"github.com/techlane/techlane/packages/pkg/authz"
)

type corrKey string

const CorrelationKey corrKey = "correlation_id"

func CorrelationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cid := r.Header.Get("X-Correlation-ID")
		if cid == "" {
			cid = uuid.NewString()
		}
		w.Header().Set("X-Correlation-ID", cid)
		ctx := context.WithValue(r.Context(), CorrelationKey, cid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func CorrelationID(ctx context.Context) string {
	if v, ok := ctx.Value(CorrelationKey).(string); ok {
		return v
	}
	return ""
}

func AuthMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := authz.BearerToken(r.Header.Get("Authorization"))
			if token == "" {
				apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing bearer token", CorrelationID(r.Context()))
				return
			}
			claims, err := authz.ParseAccessToken(secret, token)
			if err != nil {
				apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid token", CorrelationID(r.Context()))
				return
			}
			ctx := authz.WithClaims(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequirePermission(perm string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := authz.FromContext(r.Context())
			if !ok || !claims.HasPermission(perm) {
				apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "permission denied", CorrelationID(r.Context()))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
