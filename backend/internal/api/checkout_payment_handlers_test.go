package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCheckoutPaymentRoutesRejectMalformedOrderIDsBeforeDatabaseAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerCheckoutPaymentRoutes(router.Group("/api/v1"), nil, nil, nil, nil)

	start := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/orders/not-a-uuid/payment", nil)
	start.Header.Set("Idempotency-Key", "checkout-payment-key-0001")
	startResponse := httptest.NewRecorder()
	router.ServeHTTP(startResponse, start)
	if startResponse.Code != http.StatusBadRequest {
		t.Fatalf("start with malformed order ID status=%d body=%s", startResponse.Code, startResponse.Body.String())
	}

	verify := httptest.NewRequest(http.MethodPost, "/api/v1/payments/razorpay/verify", strings.NewReader(`{"orderId":"not-a-uuid"}`))
	verify.Header.Set("Content-Type", "application/json")
	verifyResponse := httptest.NewRecorder()
	router.ServeHTTP(verifyResponse, verify)
	if verifyResponse.Code != http.StatusBadRequest {
		t.Fatalf("verify with malformed order ID status=%d body=%s", verifyResponse.Code, verifyResponse.Body.String())
	}
}
