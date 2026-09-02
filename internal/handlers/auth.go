package handlers

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/ezrantn/quickshelf/internal/httpx"
	"github.com/ezrantn/quickshelf/internal/idgen"
	"github.com/ezrantn/quickshelf/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	DB *sql.DB
}

func NewAuthHandler(db *sql.DB) *AuthHandler {
	return &AuthHandler{DB: db}
}

type registerReq struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Register creates a new merchant account secured by a password and
// returns the merchant profile plus an API key. The key is only ever
// returned here — store it securely on the client side, since there's
// no "show again" endpoint in this template.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
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
	if len(req.Password) < 8 {
		httpx.Error(w, http.StatusUnprocessableEntity, "password must be at least 8 characters")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	m := models.Merchant{
		ID:           idgen.New("mch"),
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: string(hash),
		APIKey:       idgen.APIKey(),
	}

	_, err = h.DB.ExecContext(r.Context(),
		`INSERT INTO merchants (id, name, email, password_hash, api_key) VALUES (?, ?, ?, ?, ?)`,
		m.ID, m.Name, m.Email, m.PasswordHash, m.APIKey,
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

// Login verifies the merchant's email/password and returns their profile
// and API key. There's no separate session token in this template — the
// API key returned here (and at registration) doubles as the bearer
// credential for every X-API-Key-authenticated endpoint.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "email and password are required")
		return
	}

	var m models.Merchant
	err := h.DB.QueryRowContext(r.Context(),
		`SELECT id, name, email, password_hash, api_key, created_at FROM merchants WHERE email = ?`,
		req.Email,
	).Scan(&m.ID, &m.Name, &m.Email, &m.PasswordHash, &m.APIKey, &m.CreatedAt)

	switch {
	case err == sql.ErrNoRows:
		httpx.Error(w, http.StatusUnauthorized, "invalid email or password")
		return
	case err != nil:
		httpx.Error(w, http.StatusInternalServerError, "login lookup failed")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(m.PasswordHash), []byte(req.Password)); err != nil {
		httpx.Error(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	httpx.JSON(w, http.StatusOK, m)
}
