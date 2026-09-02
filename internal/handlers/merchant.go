package handlers

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/ezrantn/quickshelf/internal/httpx"
	"github.com/ezrantn/quickshelf/internal/idgen"
	"github.com/ezrantn/quickshelf/internal/middleware"
	"github.com/ezrantn/quickshelf/internal/models"
)

type MerchantHandler struct {
	DB *sql.DB
}

func NewMerchantHandler(db *sql.DB) *MerchantHandler {
	return &MerchantHandler{DB: db}
}

type registerMerchantReq struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Register creates a new merchant account and returns its API key.
// The key is only ever returned here — store it securely on the client
// side, since there's no "show again" endpoint in this template.
func (h *MerchantHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerMerchantReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Name == "" || req.Email == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "name and email are required")
		return
	}

	m := models.Merchant{
		ID:     idgen.New("mch"),
		Name:   req.Name,
		Email:  req.Email,
		APIKey: idgen.APIKey(),
	}

	_, err := h.DB.ExecContext(r.Context(),
		`INSERT INTO merchants (id, name, email, api_key) VALUES (?, ?, ?, ?)`,
		m.ID, m.Name, m.Email, m.APIKey,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			httpx.Error(w, http.StatusConflict, "email already registered")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "failed to create merchant")
		return
	}

	err = h.DB.QueryRowContext(r.Context(),
		`SELECT created_at FROM merchants WHERE id = ?`, m.ID,
	).Scan(&m.CreatedAt)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to load merchant")
		return
	}

	httpx.JSON(w, http.StatusCreated, m)
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