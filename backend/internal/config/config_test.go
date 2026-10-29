package config

import (
	"os"
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
	t.Setenv("API_ADDR", ":9000")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://shop.example, https://admin.example")
	t.Setenv("DATABASE_CONNECT_TIMEOUT", "2s")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Address != ":9000" || len(cfg.AllowedOrigins) != 2 || cfg.DatabaseTimeout.String() != "2s" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestLoadRejectsInvalidTimeout(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/aramya")
	t.Setenv("DATABASE_CONNECT_TIMEOUT", "-1s")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid timeout to fail")
	}
}

func TestConfigDoesNotReadUnsetEnv(t *testing.T) {
	if _, err := os.LookupEnv("ARAMYA_TEST_ENV_SHOULD_NOT_EXIST"); err {
		t.Fatal("unexpected test environment variable")
	}
}
