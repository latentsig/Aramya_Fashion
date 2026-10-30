package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/latentsig/aramya-fashion/backend/internal/payments"
)

type webhookGatewayStub struct {
	eventID, signature, body string
	result                   payments.WebhookResult
	err                      error
}

func (s *webhookGatewayStub) Start(context.Context, string, string, string, string) (payments.CheckoutSession, error) {
	return payments.CheckoutSession{}, nil
}
func (s *webhookGatewayStub) Verify(context.Context, string, string, string, string, string, string) (payments.PaymentResult, error) {
	return payments.PaymentResult{}, nil
}
func (s *webhookGatewayStub) ProcessWebhook(_ context.Context, eventID, signature string, body []byte) (payments.WebhookResult, error) {
	s.eventID, s.signature, s.body = eventID, signature, string(body)
	return s.result, s.err
}

func TestRazorpayWebhookRouteForwardsExactBodyAndHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &webhookGatewayStub{result: payments.WebhookResult{Duplicate: true}}
	router := gin.New()
	registerRazorpayWebhookRoute(router, stub, nil)
	body := `{"event":"payment.captured","payload":{}}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/razorpay", strings.NewReader(body))
	request.Header.Set("x-razorpay-event-id", "evt_test_123456")
	request.Header.Set("X-Razorpay-Signature", "signed-raw-body")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || stub.eventID != "evt_test_123456" || stub.signature != "signed-raw-body" || stub.body != body {
		t.Fatalf("response=%d event=%q signature=%q body=%q", response.Code, stub.eventID, stub.signature, stub.body)
	}
}

func TestRazorpayWebhookRouteRejectsMissingHeadersWithoutCallingGateway(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &webhookGatewayStub{}
	router := gin.New()
	registerRazorpayWebhookRoute(router, stub, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/razorpay", strings.NewReader(`{}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || stub.body != "" {
		t.Fatalf("response=%d gateway body=%q", response.Code, stub.body)
	}
}
