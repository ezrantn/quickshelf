package handlers

import (
	"database/sql"
	"net/http"

	"github.com/ezrantn/quickshelf/internal/httpx"
	"github.com/ezrantn/quickshelf/internal/middleware"
)

type MerchantHandler struct {
	DB *sql.DB
}

func NewMerchantHandler(db *sql.DB) *MerchantHandler {
	return &MerchantHandler{DB: db}
}

// Me returns the authenticated merchant's profile (API key omitted —
// it's write-once and shouldn't round-trip on every call).
func (h *MerchantHandler) Me(w http.ResponseWriter, r *http.Request) {
	m, ok := middleware.MerchantFromContext(r)
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	m.APIKey = ""
	httpx.JSON(w, http.StatusOK, m)
}
