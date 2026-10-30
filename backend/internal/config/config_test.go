package config

import (
	"encoding/base64"
	"testing"
)

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected missing DATABASE_URL to fail")
	}
}

func TestLoadParsesOriginsAndDurations(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/aramya")
	t.Setenv("CSRF_SECRET", base64.RawURLEncoding.EncodeToString(make([]byte, 32)))
	t.Setenv("API_ADDR", ":9000")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://shop.example, https://admin.example")
	t.Setenv("DATABASE_CONNECT_TIMEOUT", "2s")
	t.Setenv("AUTH_SESSION_TTL", "72h")
	t.Setenv("SESSION_COOKIE_SECURE", "true")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Address != ":9000" || len(cfg.AllowedOrigins) != 2 || cfg.DatabaseTimeout.String() != "2s" || cfg.SessionTTL.Hours() != 72 || !cfg.SessionCookieSecure {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestLoadRejectsInvalidTimeout(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/aramya")
	t.Setenv("CSRF_SECRET", base64.RawURLEncoding.EncodeToString(make([]byte, 32)))
	t.Setenv("DATABASE_CONNECT_TIMEOUT", "-1s")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid timeout to fail")
	}
}

func TestLoadRejectsInvalidSessionSettings(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/aramya")
	t.Setenv("CSRF_SECRET", base64.RawURLEncoding.EncodeToString(make([]byte, 32)))
	t.Setenv("AUTH_SESSION_TTL", "5m")
	if _, err := Load(); err == nil {
		t.Fatal("expected too-short session lifetime to fail")
	}
	t.Setenv("AUTH_SESSION_TTL", "168h")
	t.Setenv("SESSION_COOKIE_SECURE", "sometimes")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid cookie security flag to fail")
	}
	t.Setenv("SESSION_COOKIE_SECURE", "false")
	t.Setenv("SESSION_COOKIE_SAMESITE", "none")
	if _, err := Load(); err == nil {
		t.Fatal("expected cross-site cookies to require Secure")
	}
}

func TestLoadRejectsShortPrometheusBearerToken(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/aramya")
	t.Setenv("CSRF_SECRET", base64.RawURLEncoding.EncodeToString(make([]byte, 32)))
	t.Setenv("PROMETHEUS_BEARER_TOKEN", "too-short")
	if _, err := Load(); err == nil {
		t.Fatal("expected short Prometheus bearer token to fail")
	}
}
