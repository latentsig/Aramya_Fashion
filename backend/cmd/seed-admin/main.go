package main

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/latentsig/aramya-fashion/backend/internal/auth"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	setupEmail := os.Getenv("ADMIN_SETUP_EMAIL")
	setupName := os.Getenv("ADMIN_SETUP_NAME")
	setupPassword := os.Getenv("ADMIN_SETUP_PASSWORD")
	_ = os.Unsetenv("ADMIN_SETUP_PASSWORD")
	email, name, err := validateSetupInput(setupEmail, setupName, setupPassword)
	if err != nil {
		return err
	}
	passwordHash, err := auth.HashPassword(setupPassword)
	setupPassword = ""
	if err != nil {
		return fmt.Errorf("validate administrator password: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("open administrator setup database: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("administrator setup database is unavailable: %w", err)
	}
	userID, err := createAdministrator(ctx, pool, email, name, passwordHash)
	if err != nil {
		return err
	}
	fmt.Printf("Administrator account created (user ID %s). Sign in through the storefront login page.\n", userID)
	return nil
}

func validateSetupInput(email, name, password string) (string, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || len(email) > 254 {
		return "", "", errors.New("ADMIN_SETUP_EMAIL must be a valid email address")
	}
	name = strings.TrimSpace(name)
	if len(name) < 1 || len(name) > 120 {
		return "", "", errors.New("ADMIN_SETUP_NAME must be between 1 and 120 characters")
	}
	if len(password) < 12 || len(password) > 128 || strings.TrimSpace(password) == "" {
		return "", "", auth.ErrInvalidPassword
	}
	return email, name, nil
}

func createAdministrator(ctx context.Context, pool *pgxpool.Pool, email, name, passwordHash string) (string, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin administrator setup: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var userID string
	err = tx.QueryRow(ctx, `INSERT INTO users(email,password_hash,display_name) VALUES($1,$2,$3) RETURNING id::text`, email, passwordHash, name).Scan(&userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", errors.New("an account with that email already exists; this setup command never elevates an existing customer")
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errors.New("administrator account was not created")
		}
		return "", fmt.Errorf("create administrator account: %w", err)
	}
	result, err := tx.Exec(ctx, `INSERT INTO user_roles(user_id,role_id) SELECT $1,id FROM roles WHERE name='admin'`, userID)
	if err != nil {
		return "", fmt.Errorf("assign administrator role: %w", err)
	}
	if result.RowsAffected() != 1 {
		return "", errors.New("admin role is missing from the database; apply the commerce migration first")
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit administrator setup: %w", err)
	}
	return userID, nil
}
