package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var r2AccountIDPattern = regexp.MustCompile(`^[a-fA-F0-9]{32}$`)
var r2BucketPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

type R2Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	UploadBucket    string
	ApprovedBucket  string
	PublicBaseURL   string
}

type RazorpayConfig struct {
	KeyID         string
	KeySecret     string
	WebhookSecret string
}

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
	MetricsBearerToken    string
	R2                    *R2Config
	Razorpay              *RazorpayConfig
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
		MetricsBearerToken:    strings.TrimSpace(os.Getenv("PROMETHEUS_BEARER_TOKEN")),
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
	razorpayValues := []string{
		strings.TrimSpace(os.Getenv("RAZORPAY_KEY_ID")),
		strings.TrimSpace(os.Getenv("RAZORPAY_KEY_SECRET")),
		strings.TrimSpace(os.Getenv("RAZORPAY_WEBHOOK_SECRET")),
	}
	configuredRazorpayValues := 0
	for _, value := range razorpayValues {
		if value != "" {
			configuredRazorpayValues++
		}
	}
	if configuredRazorpayValues != 0 {
		if configuredRazorpayValues != len(razorpayValues) {
			return Config{}, errors.New("RAZORPAY_KEY_ID, RAZORPAY_KEY_SECRET, and RAZORPAY_WEBHOOK_SECRET must be configured together")
		}
		if !strings.HasPrefix(razorpayValues[0], "rzp_test_") || len(razorpayValues[0]) < len("rzp_test_x") || len(razorpayValues[1]) < 8 || len(razorpayValues[2]) < 8 {
			return Config{}, errors.New("Razorpay configuration must use a test-mode key ID and non-empty API and webhook secrets")
		}
		cfg.Razorpay = &RazorpayConfig{KeyID: razorpayValues[0], KeySecret: razorpayValues[1], WebhookSecret: razorpayValues[2]}
	}
	r2Values := []string{
		strings.TrimSpace(os.Getenv("CLOUDFLARE_R2_ACCOUNT_ID")),
		strings.TrimSpace(os.Getenv("CLOUDFLARE_R2_ACCESS_KEY_ID")),
		strings.TrimSpace(os.Getenv("CLOUDFLARE_R2_SECRET_ACCESS_KEY")),
		strings.TrimSpace(os.Getenv("CLOUDFLARE_R2_UPLOAD_BUCKET")),
		strings.TrimSpace(os.Getenv("CLOUDFLARE_R2_APPROVED_BUCKET")),
		strings.TrimSpace(os.Getenv("CLOUDFLARE_R2_PUBLIC_BASE_URL")),
	}
	configuredR2Values := 0
	for _, value := range r2Values {
		if value != "" {
			configuredR2Values++
		}
	}
	if configuredR2Values != 0 {
		if configuredR2Values != len(r2Values) {
			return Config{}, errors.New("all CLOUDFLARE_R2_* settings must be configured together")
		}
		publicURL, parseErr := url.Parse(r2Values[5])
		if !r2AccountIDPattern.MatchString(r2Values[0]) || len(r2Values[1]) < 8 || len(r2Values[2]) < 32 ||
			!r2BucketPattern.MatchString(r2Values[3]) || !r2BucketPattern.MatchString(r2Values[4]) || r2Values[3] == r2Values[4] ||
			parseErr != nil || publicURL.Scheme != "https" || publicURL.Host == "" || publicURL.User != nil || publicURL.RawQuery != "" || publicURL.Fragment != "" {
			return Config{}, errors.New("CLOUDFLARE_R2_* settings are invalid; use a 32-character account ID, restricted credentials, distinct bucket names, and an HTTPS public base URL")
		}
		cfg.R2 = &R2Config{AccountID: r2Values[0], AccessKeyID: r2Values[1], SecretAccessKey: r2Values[2], UploadBucket: r2Values[3], ApprovedBucket: r2Values[4], PublicBaseURL: strings.TrimRight(r2Values[5], "/")}
	}
	if cfg.MetricsBearerToken != "" && len(cfg.MetricsBearerToken) < 32 {
		return Config{}, errors.New("PROMETHEUS_BEARER_TOKEN must contain at least 32 characters when configured")
	}
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
