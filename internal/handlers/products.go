package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"github.com/ezrantn/quickshelf/internal/httpx"
	"github.com/ezrantn/quickshelf/internal/idgen"
	"github.com/ezrantn/quickshelf/internal/middleware"
	"github.com/ezrantn/quickshelf/internal/models"
)

type ProductHandler struct {
	DB *sql.DB
}

func NewProductHandler(db *sql.DB) *ProductHandler {
	return &ProductHandler{DB: db}
}

type upsertProductReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceCents  int64  `json:"price_cents"`
	Currency    string `json:"currency"`
	IsActive    *bool  `json:"is_active"`
}

func (h *ProductHandler) Create(w http.ResponseWriter, r *http.Request) {
	merchant, _ := middleware.MerchantFromContext(r)

	var req upsertProductReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "name is required")
		return
	}
	if req.PriceCents <= 0 {
		httpx.Error(w, http.StatusUnprocessableEntity, "price_cents must be greater than 0")
		return
	}
	if req.Currency == "" {
		req.Currency = "USD"
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	p := models.Product{
		ID:          idgen.New("prod"),
		MerchantID:  merchant.ID,
		Name:        req.Name,
		Description: req.Description,
		PriceCents:  req.PriceCents,
		Currency:    strings.ToUpper(req.Currency),
		IsActive:    isActive,
	}

	_, err := h.DB.ExecContext(r.Context(),
		`INSERT INTO products (id, merchant_id, name, description, price_cents, currency, is_active)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.MerchantID, p.Name, p.Description, p.PriceCents, p.Currency, boolToInt(p.IsActive),
	)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to create product")
		return
	}

	loaded, err := h.getByID(r.Context(), p.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to load product")
		return
	}
	httpx.JSON(w, http.StatusCreated, loaded)
}

func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
	merchant, _ := middleware.MerchantFromContext(r)

	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT id, merchant_id, name, description, price_cents, currency, is_active, created_at, updated_at
		 FROM products WHERE merchant_id = ? ORDER BY created_at DESC`, merchant.ID,
	)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to list products")
		return
	}
	defer rows.Close()

	products := []models.Product{}
	for rows.Next() {
		var p models.Product
		var active int
		if err := rows.Scan(&p.ID, &p.MerchantID, &p.Name, &p.Description, &p.PriceCents, &p.Currency, &active, &p.CreatedAt, &p.UpdatedAt); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "failed to scan product")
			return
		}
		p.IsActive = active == 1
		products = append(products, p)
	}
	httpx.JSON(w, http.StatusOK, products)
}

// Get is public — used by the storefront / checkout page. No auth required.
func (h *ProductHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := h.getByID(r.Context(), id)
	if err == sql.ErrNoRows {
		httpx.Error(w, http.StatusNotFound, "product not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to load product")
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}

func (h *ProductHandler) Update(w http.ResponseWriter, r *http.Request) {
	merchant, _ := middleware.MerchantFromContext(r)
	id := r.PathValue("id")

	existing, err := h.getByID(r.Context(), id)
	if err == sql.ErrNoRows {
		httpx.Error(w, http.StatusNotFound, "product not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to load product")
		return
	}
	if existing.MerchantID != merchant.ID {
		httpx.Error(w, http.StatusForbidden, "not your product")
		return
	}

	var req upsertProductReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.Name) != "" {
		existing.Name = strings.TrimSpace(req.Name)
	}
	existing.Description = req.Description
	if req.PriceCents > 0 {
		existing.PriceCents = req.PriceCents
	}
	if req.Currency != "" {
		existing.Currency = strings.ToUpper(req.Currency)
	}
	if req.IsActive != nil {
		existing.IsActive = *req.IsActive
	}

	_, err = h.DB.ExecContext(r.Context(),
		`UPDATE products
		 SET name = ?, description = ?, price_cents = ?, currency = ?, is_active = ?,
		     updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		 WHERE id = ?`,
		existing.Name, existing.Description, existing.PriceCents, existing.Currency, boolToInt(existing.IsActive), existing.ID,
	)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to update product")
		return
	}

	loaded, err := h.getByID(r.Context(), existing.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to reload product")
		return
	}
	httpx.JSON(w, http.StatusOK, loaded)
}

func (h *ProductHandler) Delete(w http.ResponseWriter, r *http.Request) {
	merchant, _ := middleware.MerchantFromContext(r)
	id := r.PathValue("id")

	existing, err := h.getByID(r.Context(), id)
	if err == sql.ErrNoRows {
		httpx.Error(w, http.StatusNotFound, "product not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to load product")
		return
	}
	if existing.MerchantID != merchant.ID {
		httpx.Error(w, http.StatusForbidden, "not your product")
		return
	}

	if _, err := h.DB.ExecContext(r.Context(), `DELETE FROM products WHERE id = ?`, id); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to delete product")
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *ProductHandler) getByID(ctx context.Context, id string) (models.Product, error) {
	var p models.Product
	var active int
	err := h.DB.QueryRowContext(ctx,
		`SELECT id, merchant_id, name, description, price_cents, currency, is_active, created_at, updated_at
		 FROM products WHERE id = ?`, id,
	).Scan(&p.ID, &p.MerchantID, &p.Name, &p.Description, &p.PriceCents, &p.Currency, &active, &p.CreatedAt, &p.UpdatedAt)
	p.IsActive = active == 1
	return p, err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
