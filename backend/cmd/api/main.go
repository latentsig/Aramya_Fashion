package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/latentsig/aramya-fashion/backend/internal/admin"
	"github.com/latentsig/aramya-fashion/backend/internal/api"
	"github.com/latentsig/aramya-fashion/backend/internal/appointments"
	"github.com/latentsig/aramya-fashion/backend/internal/auth"
	"github.com/latentsig/aramya-fashion/backend/internal/cart"
	"github.com/latentsig/aramya-fashion/backend/internal/catalog"
	"github.com/latentsig/aramya-fashion/backend/internal/checkout"
	"github.com/latentsig/aramya-fashion/backend/internal/config"
	"github.com/latentsig/aramya-fashion/backend/internal/orders"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid_configuration", "error", err)
		os.Exit(1)
	}

	connectCtx, cancelConnect := context.WithTimeout(context.Background(), cfg.DatabaseTimeout)
	pool, err := pgxpool.New(connectCtx, cfg.DatabaseURL)
	if err == nil {
		err = pool.Ping(connectCtx)
	}
	cancelConnect()
	if err != nil {
		logger.Error("database_unavailable", "error_type", fmt.Sprintf("%T", err))
		os.Exit(1)
	}
	defer pool.Close()

	authService, err := auth.NewService(pool, cfg.SessionTTL, cfg.QueryTimeout)
	if err != nil {
		logger.Error("auth_initialization_failed", "error_type", fmt.Sprintf("%T", err))
		os.Exit(1)
	}
	cartService, err := cart.NewService(pool, cfg.QueryTimeout)
	if err != nil {
		logger.Error("cart_initialization_failed", "error_type", fmt.Sprintf("%T", err))
		os.Exit(1)
	}
	checkoutService, err := checkout.NewService(pool, cfg.QueryTimeout)
	if err != nil {
		logger.Error("checkout_initialization_failed", "error_type", fmt.Sprintf("%T", err))
		os.Exit(1)
	}
	orderRepository, err := orders.NewRepository(pool, cfg.QueryTimeout)
	if err != nil {
		logger.Error("orders_initialization_failed", "error_type", fmt.Sprintf("%T", err))
		os.Exit(1)
	}

	router := api.NewRouter(api.RouterConfig{
		Catalog:        catalog.NewRepository(pool, cfg.QueryTimeout),
		Auth:           authService,
		Cart:           cartService,
		Checkout:       checkoutService,
		Orders:         orderRepository,
		StudioRequests: appointments.NewRepository(pool, cfg.QueryTimeout),
		AdminStudio:    admin.NewStudioRepository(pool, cfg.QueryTimeout),
		AdminInventory: admin.NewInventoryRepository(pool, cfg.QueryTimeout),
		AdminCatalog:   admin.NewCatalogRepository(pool, cfg.QueryTimeout),
		AdminIdentity:  authService,
		Ready: func() error {
			ctx, cancel := context.WithTimeout(context.Background(), cfg.QueryTimeout)
			defer cancel()
			return pool.Ping(ctx)
		},
		AllowedOrigins:        cfg.AllowedOrigins,
		SessionCookieSecure:   cfg.SessionCookieSecure,
		SessionCookieSameSite: cfg.SessionCookieSameSite,
		CSRFSecret:            cfg.CSRFSecret,
		Logger:                logger,
	})
	server := &http.Server{Addr: cfg.Address, Handler: router, ReadHeaderTimeout: 5 * time.Second}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("api_starting", "address", cfg.Address)
		serverErrors <- server.ListenAndServe()
	}()

	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("api_stopped", "error_type", fmt.Sprintf("%T", err))
			os.Exit(1)
		}
	case <-stopCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful_shutdown_failed", "error", err)
			_ = server.Close()
		}
	}
}
