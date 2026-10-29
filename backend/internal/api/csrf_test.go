package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestCSRFGuardRequiresDoubleSubmitToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	secret := []byte("01234567890123456789012345678901")
	router.Use(csrfGuard(nil, secret))
	router.POST("/mutate", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	request := httptest.NewRequest(http.MethodPost, "/mutate", nil)
	valid, err := newAnonymousCSRF(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: valid})
	request.Header.Set("X-CSRF-Token", "different-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("mismatched token status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/mutate", nil)
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: valid})
	request.Header.Set("X-CSRF-Token", valid)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("matching token status = %d", response.Code)
	}
}

func TestCSRFGuardSkipsSafeMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(csrfGuard(nil, []byte("01234567890123456789012345678901")))
	router.GET("/read", func(c *gin.Context) { c.Status(http.StatusOK) })
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/read", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("safe request status = %d", response.Code)
	}
}

func TestCSRFGuardRejectsUnissuedAnonymousToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	secret := []byte("01234567890123456789012345678901")
	router.Use(csrfGuard(nil, secret))
	router.POST("/mutate", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodPost, "/mutate", nil)
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "forged-token"})
	request.Header.Set("X-CSRF-Token", "forged-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("forged token status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestAnonymousCSRFTokenIsSignedAndExpires(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	now := time.Now()
	token, err := newAnonymousCSRF(secret, now)
	if err != nil {
		t.Fatal(err)
	}
	if !verifyAnonymousCSRF(token, secret, now.Add(time.Minute)) {
		t.Fatal("fresh signed CSRF token was rejected")
	}
	if verifyAnonymousCSRF(token, []byte("different secret with at least 32 bytes"), now) {
		t.Fatal("CSRF token signed with a different secret was accepted")
	}
	if verifyAnonymousCSRF(token, secret, now.Add(16*time.Minute)) {
		t.Fatal("expired CSRF token was accepted")
	}
	if verifyAnonymousCSRF("forged-token", secret, now) {
		t.Fatal("unsigned CSRF token was accepted")
	}
	secondToken, err := newAnonymousCSRF(secret, now)
	if err != nil {
		t.Fatal(err)
	}
	if token == secondToken {
		t.Fatal("anonymous CSRF tokens should have unique random nonces")
	}
}
