package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthAndReadiness(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		ready  func() error
		status int
	}{
		{name: "health", path: "/health", status: http.StatusOK},
		{name: "readiness rejects missing database check", path: "/ready", status: http.StatusServiceUnavailable},
		{name: "readiness when database is reachable", path: "/ready", ready: func() error { return nil }, status: http.StatusOK},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := NewRouter(RouterConfig{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Ready: test.ready})
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			router.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
			if response.Header().Get("X-Request-ID") == "" {
				t.Fatal("expected request ID header")
			}
		})
	}
}

func TestCorsAllowlist(t *testing.T) {
	router := NewRouter(RouterConfig{AllowedOrigins: []string{"https://shop.example"}})
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set("Origin", "https://shop.example")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "https://shop.example" {
		t.Fatalf("allow origin = %q", got)
	}
	request = httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set("Origin", "https://untrusted.example")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("untrusted origin was allowed: %q", got)
	}
}

func TestBoundedInteger(t *testing.T) {
	for _, value := range []string{"0", "101", "abc"} {
		if _, err := parseBoundedInt(value, 24, 1, 100); err == nil {
			t.Errorf("parseBoundedInt(%q) unexpectedly succeeded", value)
		}
	}
	got, err := parseBoundedInt("12", 24, 1, 100)
	if err != nil || got != 12 {
		t.Fatalf("parseBoundedInt(12) = %d, %v", got, err)
	}
}
