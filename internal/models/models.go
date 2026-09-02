package models

type Merchant struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	APIKey    string `json:"api_key,omitempty"`
	CreatedAt string `json:"created_at"`
}

type Product struct {
	ID          string `json:"id"`
	MerchantID  string `json:"merchant_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceCents  int64  `json:"price_cents"`
	Currency    string `json:"currency"`
	IsActive    bool   `json:"is_active"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type Customer struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type OrderItem struct {
	ID             string `json:"id"`
	OrderID        string `json:"order_id"`
	ProductID      string `json:"product_id"`
	Quantity       int64  `json:"quantity"`
	UnitPriceCents int64  `json:"unit_price_cents"`
}

type Order struct {
	ID              string      `json:"id"`
	MerchantID      string      `json:"merchant_id"`
	CustomerID      string      `json:"customer_id"`
	Status          string      `json:"status"`
	TotalCents      int64       `json:"total_cents"`
	Currency        string      `json:"currency"`
	PaymentProvider string      `json:"payment_provider"`
	PaymentRef      string      `json:"payment_ref"`
	CreatedAt       string      `json:"created_at"`
	UpdatedAt       string      `json:"updated_at"`
	Items           []OrderItem `json:"items,omitempty"`
}
