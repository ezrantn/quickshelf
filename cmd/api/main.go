package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ezrantn/quickshelf/internal/config"
	"github.com/ezrantn/quickshelf/internal/db"
	"github.com/ezrantn/quickshelf/internal/docs"
	"github.com/ezrantn/quickshelf/internal/handlers"
	"github.com/ezrantn/quickshelf/internal/middleware"
)

func main() {
	cfg := config.Load()

	conn, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer conn.Close()

	if err := db.Migrate(conn); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	mux := http.NewServeMux()
	registerRoutes(mux, conn)

	handler := middleware.Chain(mux, middleware.Logging)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("quickshelf listening on :%s (env=%s, db=%s)", cfg.Port, cfg.Env, cfg.DBPath)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}

func registerRoutes(mux *http.ServeMux, conn *sql.DB) {
	merchantH := handlers.NewMerchantHandler(conn)
	productH := handlers.NewProductHandler(conn)
	orderH := handlers.NewOrderHandler(conn)

	auth := middleware.RequireMerchant(conn)

	docs.RegisterRoutes(mux)

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// --- Merchant onboarding ---
	mux.HandleFunc("POST /api/v1/merchants", merchantH.Register)
	mux.Handle("GET /api/v1/merchants/me", auth(http.HandlerFunc(merchantH.Me)))

	// --- Products ---
	// Get-by-id is public so it can power a storefront/checkout page.
	// Everything else requires the merchant's API key.
	mux.Handle("POST /api/v1/products", auth(http.HandlerFunc(productH.Create)))
	mux.Handle("GET /api/v1/products", auth(http.HandlerFunc(productH.List)))
	mux.HandleFunc("GET /api/v1/products/{id}", productH.Get)
	mux.Handle("PUT /api/v1/products/{id}", auth(http.HandlerFunc(productH.Update)))
	mux.Handle("DELETE /api/v1/products/{id}", auth(http.HandlerFunc(productH.Delete)))

	// --- Orders / checkout ---
	// Checkout is public (buyer-facing). Settlement endpoints are
	// merchant-authenticated here — swap for a payment-provider webhook
	// in production (see the comment on OrderHandler.Complete).
	mux.HandleFunc("POST /api/v1/checkout", orderH.Checkout)
	mux.Handle("GET /api/v1/orders", auth(http.HandlerFunc(orderH.List)))
	mux.Handle("GET /api/v1/orders/{id}", auth(http.HandlerFunc(orderH.Get)))
	mux.Handle("POST /api/v1/orders/{id}/complete", auth(http.HandlerFunc(orderH.Complete)))
	mux.Handle("POST /api/v1/orders/{id}/fail", auth(http.HandlerFunc(orderH.Fail)))
	mux.Handle("POST /api/v1/orders/{id}/refund", auth(http.HandlerFunc(orderH.Refund)))
}
