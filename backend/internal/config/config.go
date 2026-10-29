package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Address               string
	DatabaseURL           string
	AllowedOrigins        []string
	DatabaseTimeout       time.Duration
	QueryTimeout          time.Duration
	SessionTTL            time.Duration
	SessionCookieSecure   bool
	SessionCookieSameSite string
	CSRFSecret            []byte
}

func Load() (Config, error) {
	cfg := Config{
		Address:               strings.TrimSpace(os.Getenv("API_ADDR")),
		DatabaseURL:           strings.TrimSpace(os.Getenv("DATABASE_URL")),
		AllowedOrigins:        splitList(os.Getenv("CORS_ALLOWED_ORIGINS")),
		DatabaseTimeout:       5 * time.Second,
		QueryTimeout:          3 * time.Second,
		SessionTTL:            7 * 24 * time.Hour,
		SessionCookieSameSite: "lax",
	}
	if cfg.Address == "" {
		cfg.Address = ":8080"
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	csrfSecret, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(os.Getenv("CSRF_SECRET")))
	if err != nil || len(csrfSecret) < 32 {
		return Config{}, errors.New("CSRF_SECRET must be base64url encoding of at least 32 random bytes")
	}
	cfg.CSRFSecret = csrfSecret
	if value := strings.TrimSpace(os.Getenv("DATABASE_CONNECT_TIMEOUT")); value != "" {
		duration, err := time.ParseDuration(value)
		if err != nil || duration <= 0 {
			return Config{}, fmt.Errorf("DATABASE_CONNECT_TIMEOUT must be a positive duration")
		}
		cfg.DatabaseTimeout = duration
	}
	if value := strings.TrimSpace(os.Getenv("DATABASE_QUERY_TIMEOUT")); value != "" {
		duration, err := time.ParseDuration(value)
		if err != nil || duration <= 0 {
			return Config{}, fmt.Errorf("DATABASE_QUERY_TIMEOUT must be a positive duration")
		}
		cfg.QueryTimeout = duration
	}
	if value := strings.TrimSpace(os.Getenv("AUTH_SESSION_TTL")); value != "" {
		duration, err := time.ParseDuration(value)
		if err != nil || duration < time.Hour || duration > 30*24*time.Hour {
			return Config{}, errors.New("AUTH_SESSION_TTL must be between 1 hour and 30 days")
		}
		cfg.SessionTTL = duration
	}
	if value := strings.TrimSpace(os.Getenv("SESSION_COOKIE_SECURE")); value != "" {
		secure, err := strconv.ParseBool(value)
		if err != nil {
			return Config{}, errors.New("SESSION_COOKIE_SECURE must be true or false")
		}
		cfg.SessionCookieSecure = secure
	}
	if value := strings.ToLower(strings.TrimSpace(os.Getenv("SESSION_COOKIE_SAMESITE"))); value != "" {
		if value != "lax" && value != "strict" && value != "none" {
			return Config{}, errors.New("SESSION_COOKIE_SAMESITE must be lax, strict, or none")
		}
		cfg.SessionCookieSameSite = value
	}
	if cfg.SessionCookieSameSite == "none" && !cfg.SessionCookieSecure {
		return Config{}, errors.New("SESSION_COOKIE_SECURE must be true when SESSION_COOKIE_SAMESITE is none")
	}
	return cfg, nil
}

func splitList(value string) []string {
	var values []string
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}
