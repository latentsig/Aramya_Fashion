package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRateLimiterLimitsByRemoteIPAndResetsWindow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := newRateLimiter(1, time.Hour)
	router := gin.New()
	router.Use(limiter.Middleware())
	router.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/health", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("first request status = %d", first.Code)
	}
	second := httptest.NewRecorder()
	router.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/health", nil))
	if second.Code != http.StatusTooManyRequests || second.Header().Get("Retry-After") == "" {
		t.Fatalf("second request status = %d, retry-after=%q", second.Code, second.Header().Get("Retry-After"))
	}
	if remoteClientIP("[::1]:9000") != "::1" {
		t.Fatal("failed to parse IPv6 client address")
	}
}
