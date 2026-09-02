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

type OrderHandler struct {
	DB *sql.DB
}

func NewOrderHandler(db *sql.DB) *OrderHandler {
	return &OrderHandler{DB: db}
}

type checkoutItemReq struct {
	ProductID string `json:"product_id"`
	Quantity  int64  `json:"quantity"`
}

type checkoutReq struct {
	CustomerEmail string            `json:"customer_email"`
	CustomerName  string            `json:"customer_name"`
	Items         []checkoutItemReq `json:"items"`
}

// Checkout is public: a buyer hits this from the storefront to start an
// order. It creates the order as "pending". Wire Complete/Fail to your
// payment provider's webhook once the charge is confirmed — never let an
// unauthenticated client mark its own order as paid (see updateStatus).
func (h *OrderHandler) Checkout(w http.ResponseWriter, r *http.Request) {
	var req checkoutReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.CustomerEmail = strings.TrimSpace(strings.ToLower(req.CustomerEmail))
	if req.CustomerEmail == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "customer_email is required")
		return
	}
	if len(req.Items) == 0 {
		httpx.Error(w, http.StatusUnprocessableEntity, "at least one item is required")
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback()

	var merchantID, currency string
	var totalCents int64
	items := make([]models.OrderItem, 0, len(req.Items))

	for _, it := range req.Items {
		if it.Quantity <= 0 {
			it.Quantity = 1
		}

		var p models.Product
		var active int
		err := tx.QueryRowContext(r.Context(),
			`SELECT id, merchant_id, price_cents, currency, is_active FROM products WHERE id = ?`,
			it.ProductID,
		).Scan(&p.ID, &p.MerchantID, &p.PriceCents, &p.Currency, &active)
		if err == sql.ErrNoRows {
			httpx.Error(w, http.StatusUnprocessableEntity, "product not found: "+it.ProductID)
			return
		}
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "failed to load product")
			return
		}
		if active != 1 {
			httpx.Error(w, http.StatusUnprocessableEntity, "product not available: "+it.ProductID)
			return
		}

		if merchantID == "" {
			merchantID = p.MerchantID
			currency = p.Currency
		} else if merchantID != p.MerchantID {
			httpx.Error(w, http.StatusUnprocessableEntity, "all items in one order must belong to the same merchant")
			return
		}

		totalCents += p.PriceCents * it.Quantity
		items = append(items, models.OrderItem{
			ID:             idgen.New("itm"),
			ProductID:      p.ID,
			Quantity:       it.Quantity,
			UnitPriceCents: p.PriceCents,
		})
	}

	customerID, err := findOrCreateCustomer(r.Context(), tx, req.CustomerEmail, req.CustomerName)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to create customer")
		return
	}

	order := models.Order{
		ID:         idgen.New("ord"),
		MerchantID: merchantID,
		CustomerID: customerID,
		Status:     "pending",
		TotalCents: totalCents,
		Currency:   currency,
	}

	_, err = tx.ExecContext(r.Context(),
		`INSERT INTO orders (id, merchant_id, customer_id, status, total_cents, currency)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		order.ID, order.MerchantID, order.CustomerID, order.Status, order.TotalCents, order.Currency,
	)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to create order")
		return
	}

	for i := range items {
		items[i].OrderID = order.ID
		_, err = tx.ExecContext(r.Context(),
			`INSERT INTO order_items (id, order_id, product_id, quantity, unit_price_cents)
			 VALUES (?, ?, ?, ?, ?)`,
			items[i].ID, items[i].OrderID, items[i].ProductID, items[i].Quantity, items[i].UnitPriceCents,
		)
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "failed to create order item")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to commit order")
		return
	}

	order.Items = items
	httpx.JSON(w, http.StatusCreated, order)
}

// Complete/Fail/Refund are merchant-authenticated in this template for
// simplicity. In production, put these behind your payment provider's
// webhook instead (verify the provider's signature, look up the order by
// payment_ref) rather than trusting the merchant's own API key to settle
// orders — otherwise a compromised key could mint "paid" orders for free.
func (h *OrderHandler) Complete(w http.ResponseWriter, r *http.Request) {
	h.updateStatus(w, r, "paid")
}

func (h *OrderHandler) Fail(w http.ResponseWriter, r *http.Request) {
	h.updateStatus(w, r, "failed")
}

func (h *OrderHandler) Refund(w http.ResponseWriter, r *http.Request) {
	h.updateStatus(w, r, "refunded")
}

func (h *OrderHandler) updateStatus(w http.ResponseWriter, r *http.Request, status string) {
	merchant, _ := middleware.MerchantFromContext(r)
	id := r.PathValue("id")

	res, err := h.DB.ExecContext(r.Context(),
		`UPDATE orders SET status = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		 WHERE id = ? AND merchant_id = ?`,
		status, id, merchant.ID,
	)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to update order")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		httpx.Error(w, http.StatusNotFound, "order not found")
		return
	}

	order, err := h.getByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to load order")
		return
	}
	httpx.JSON(w, http.StatusOK, order)
}

func (h *OrderHandler) List(w http.ResponseWriter, r *http.Request) {
	merchant, _ := middleware.MerchantFromContext(r)

	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT id, merchant_id, customer_id, status, total_cents, currency, payment_provider, payment_ref, created_at, updated_at
		 FROM orders WHERE merchant_id = ? ORDER BY created_at DESC`, merchant.ID,
	)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to list orders")
		return
	}
	defer rows.Close()

	orders := []models.Order{}
	for rows.Next() {
		var o models.Order
		if err := rows.Scan(&o.ID, &o.MerchantID, &o.CustomerID, &o.Status, &o.TotalCents, &o.Currency, &o.PaymentProvider, &o.PaymentRef, &o.CreatedAt, &o.UpdatedAt); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "failed to scan order")
			return
		}
		orders = append(orders, o)
	}
	httpx.JSON(w, http.StatusOK, orders)
}

func (h *OrderHandler) Get(w http.ResponseWriter, r *http.Request) {
	merchant, _ := middleware.MerchantFromContext(r)
	id := r.PathValue("id")

	order, err := h.getByID(r.Context(), id)
	if err == sql.ErrNoRows {
		httpx.Error(w, http.StatusNotFound, "order not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to load order")
		return
	}
	if order.MerchantID != merchant.ID {
		httpx.Error(w, http.StatusForbidden, "not your order")
		return
	}
	httpx.JSON(w, http.StatusOK, order)
}

func (h *OrderHandler) getByID(ctx context.Context, id string) (models.Order, error) {
	var o models.Order
	err := h.DB.QueryRowContext(ctx,
		`SELECT id, merchant_id, customer_id, status, total_cents, currency, payment_provider, payment_ref, created_at, updated_at
		 FROM orders WHERE id = ?`, id,
	).Scan(&o.ID, &o.MerchantID, &o.CustomerID, &o.Status, &o.TotalCents, &o.Currency, &o.PaymentProvider, &o.PaymentRef, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return o, err
	}

	rows, err := h.DB.QueryContext(ctx,
		`SELECT id, order_id, product_id, quantity, unit_price_cents FROM order_items WHERE order_id = ?`, id,
	)
	if err != nil {
		return o, err
	}
	defer rows.Close()
	for rows.Next() {
		var it models.OrderItem
		if err := rows.Scan(&it.ID, &it.OrderID, &it.ProductID, &it.Quantity, &it.UnitPriceCents); err != nil {
			return o, err
		}
		o.Items = append(o.Items, it)
	}
	return o, nil
}

// findOrCreateCustomer dedupes customers by email per platform. Good
// enough for a template; if you need per-merchant customer scoping,
// change the UNIQUE constraint to (merchant_id, email) and pass
// merchant_id through here too.
func findOrCreateCustomer(ctx context.Context, tx *sql.Tx, email, name string) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, `SELECT id FROM customers WHERE email = ?`, email).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}

	id = idgen.New("cus")
	_, err = tx.ExecContext(ctx, `INSERT INTO customers (id, email, name) VALUES (?, ?, ?)`, id, email, name)
	if err != nil {
		return "", err
	}
	return id, nil
}
