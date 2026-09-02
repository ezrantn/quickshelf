package middleware

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/ezrantn/quickshelf/internal/models"

	"github.com/ezrantn/quickshelf/internal/httpx"
)

type ctxKey string

const merchantCtxKey ctxKey = "merchant"

// RequireMerchant validates the X-API-Key header against the merchants
// table and injects the merchant into the request context. Wrap any
// handler that should only be callable by the merchant themselves
// (product management, order listing, settlement endpoints, etc).
func RequireMerchant(conn *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-API-Key")
			if key == "" {
				httpx.Error(w, http.StatusUnauthorized, "missing X-API-Key header")
				return
			}

			var m models.Merchant
			err := conn.QueryRowContext(r.Context(),
				`SELECT id, name, email, password_hash, api_key, created_at FROM merchants WHERE api_key = ?`,
				key,
			).Scan(&m.ID, &m.Name, &m.Email, &m.PasswordHash, &m.APIKey, &m.CreatedAt)

			switch {
			case err == sql.ErrNoRows:
				httpx.Error(w, http.StatusUnauthorized, "invalid API key")
				return
			case err != nil:
				httpx.Error(w, http.StatusInternalServerError, "auth lookup failed")
				return
			}

			ctx := context.WithValue(r.Context(), merchantCtxKey, m)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// MerchantFromContext retrieves the merchant injected by RequireMerchant.
// Only call this inside handlers mounted behind RequireMerchant.
func MerchantFromContext(r *http.Request) (models.Merchant, bool) {
	m, ok := r.Context().Value(merchantCtxKey).(models.Merchant)
	return m, ok
}
