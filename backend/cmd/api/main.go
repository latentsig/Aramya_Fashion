package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/latentsig/aramya-fashion/backend/internal/api"
	"github.com/latentsig/aramya-fashion/backend/internal/catalog"
	"github.com/latentsig/aramya-fashion/backend/internal/config"
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
		logger.Error("database_unavailable", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	router := api.NewRouter(api.RouterConfig{
		Catalog: catalog.NewRepository(pool, cfg.QueryTimeout),
		Ready: func() error {
			ctx, cancel := context.WithTimeout(context.Background(), cfg.QueryTimeout)
			defer cancel()
			return pool.Ping(ctx)
		},
		AllowedOrigins: cfg.AllowedOrigins,
		Logger:         logger,
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
			logger.Error("api_stopped", "error", err)
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
