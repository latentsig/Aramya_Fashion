package config

import (
	"encoding/base64"
	"testing"
)

func setBaseConfig(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://localhost/aramya")
	t.Setenv("CSRF_SECRET", base64.RawURLEncoding.EncodeToString(make([]byte, 32)))
	t.Setenv("CHECKOUT_PRICING_FINAL", "false")
	for _, name := range []string{"CLOUDFLARE_R2_ACCOUNT_ID", "CLOUDFLARE_R2_ACCESS_KEY_ID", "CLOUDFLARE_R2_SECRET_ACCESS_KEY", "CLOUDFLARE_R2_UPLOAD_BUCKET", "CLOUDFLARE_R2_APPROVED_BUCKET", "CLOUDFLARE_R2_PUBLIC_BASE_URL"} {
		t.Setenv(name, "")
	}
	for _, name := range []string{"RAZORPAY_KEY_ID", "RAZORPAY_KEY_SECRET", "RAZORPAY_WEBHOOK_SECRET"} {
		t.Setenv(name, "")
	}
}

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

func TestLoadR2IsOptionalButMustBeCompleteAndValid(t *testing.T) {
	setBaseConfig(t)
	cfg, err := Load()
	if err != nil || cfg.R2 != nil {
		t.Fatalf("unset R2 should be disabled: cfg=%+v err=%v", cfg.R2, err)
	}
	t.Setenv("CLOUDFLARE_R2_ACCOUNT_ID", "0123456789abcdef0123456789abcdef")
	if _, err := Load(); err == nil {
		t.Fatal("partial R2 config must fail")
	}
	setBaseConfig(t)
	t.Setenv("CLOUDFLARE_R2_ACCOUNT_ID", "0123456789abcdef0123456789abcdef")
	t.Setenv("CLOUDFLARE_R2_ACCESS_KEY_ID", "SYNTHETIC_ACCESS_KEY")
	t.Setenv("CLOUDFLARE_R2_SECRET_ACCESS_KEY", "synthetic-test-secret-key-value-1234567890")
	t.Setenv("CLOUDFLARE_R2_UPLOAD_BUCKET", "aramya-staging")
	t.Setenv("CLOUDFLARE_R2_APPROVED_BUCKET", "aramya-approved")
	t.Setenv("CLOUDFLARE_R2_PUBLIC_BASE_URL", "https://media.example.test")
	cfg, err = Load()
	if err != nil || cfg.R2 == nil || cfg.R2.UploadBucket != "aramya-staging" {
		t.Fatalf("valid R2 config rejected: cfg=%+v err=%v", cfg.R2, err)
	}
	t.Setenv("CLOUDFLARE_R2_APPROVED_BUCKET", "aramya-staging")
	if _, err := Load(); err == nil {
		t.Fatal("same upload and approved buckets must fail")
	}
	t.Setenv("CLOUDFLARE_R2_APPROVED_BUCKET", "aramya-approved")
	t.Setenv("CLOUDFLARE_R2_PUBLIC_BASE_URL", "http://media.example.test")
	if _, err := Load(); err == nil {
		t.Fatal("non-HTTPS media domain must fail")
	}
}

func TestLoadRazorpayAllowsOnlyCompleteTestModeCredentials(t *testing.T) {
	setBaseConfig(t)
	t.Setenv("CHECKOUT_PRICING_FINAL", "false")
	cfg, err := Load()
	if err != nil || cfg.Razorpay != nil {
		t.Fatalf("unset Razorpay must leave payments disabled: config=%+v err=%v", cfg.Razorpay, err)
	}
	t.Setenv("RAZORPAY_KEY_ID", "rzp_test_syntheticKey")
	if _, err := Load(); err == nil {
		t.Fatal("partial Razorpay credentials must fail")
	}
	setBaseConfig(t)
	t.Setenv("RAZORPAY_KEY_ID", "rzp_test_syntheticKey")
	t.Setenv("RAZORPAY_KEY_SECRET", "synthetic-test-api-secret")
	t.Setenv("RAZORPAY_WEBHOOK_SECRET", "synthetic-test-hook-secret")
	cfg, err = Load()
	if err != nil || cfg.Razorpay == nil || cfg.Razorpay.KeyID != "rzp_test_syntheticKey" {
		t.Fatalf("complete Razorpay test configuration rejected: config=%+v err=%v", cfg.Razorpay, err)
	}
	t.Setenv("RAZORPAY_KEY_ID", "rzp_live_syntheticKey")
	if _, err := Load(); err == nil {
		t.Fatal("live-mode key ID must be rejected")
	}
}

func TestCheckoutPricingCannotEnableWithoutRazorpayTestCredentials(t *testing.T) {
	setBaseConfig(t)
	t.Setenv("CHECKOUT_PRICING_FINAL", "true")
	if _, err := Load(); err == nil {
		t.Fatal("final checkout pricing must not enable payment without Razorpay test credentials")
	}
	setBaseConfig(t)
	t.Setenv("CHECKOUT_PRICING_FINAL", "invalid")
	if _, err := Load(); err == nil {
		t.Fatal("invalid checkout pricing flag must fail")
	}
}

func TestCheckoutPricingIsEnabledOnlyByExplicitOperatorSetting(t *testing.T) {
	setBaseConfig(t)
	t.Setenv("RAZORPAY_KEY_ID", "rzp_test_syntheticKey")
	t.Setenv("RAZORPAY_KEY_SECRET", "synthetic-test-api-secret")
	t.Setenv("RAZORPAY_WEBHOOK_SECRET", "synthetic-test-hook-secret")
	t.Setenv("CHECKOUT_PRICING_FINAL", "true")
	cfg, err := Load()
	if err != nil || !cfg.CheckoutPricingFinal || cfg.Razorpay == nil {
		t.Fatalf("explicit test pricing configuration rejected: final=%v razorpay=%v err=%v", cfg.CheckoutPricingFinal, cfg.Razorpay != nil, err)
	}
}
