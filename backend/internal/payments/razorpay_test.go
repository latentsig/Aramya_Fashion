package payments

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var testCredentials = Credentials{KeyID: "rzp_test_exampleKey", KeySecret: "synthetic-test-api-secret", WebhookSecret: "synthetic-test-webhook-secret"}

func TestRazorpayClientCreatesOrderWithServerAmountAndTestCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/orders" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		user, password, ok := r.BasicAuth()
		if !ok || user != testCredentials.KeyID || password != testCredentials.KeySecret {
			t.Fatal("test credentials were not sent using HTTP Basic auth")
		}
		var got OrderRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		if got.Amount != 125050 || got.Currency != "INR" || got.Receipt != "AR-TEST-RECEIPT" {
			t.Fatalf("provider request=%+v", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"order_test_123456","entity":"order","amount":125050,"currency":"INR","receipt":"AR-TEST-RECEIPT","status":"created"}`)
	}))
	defer server.Close()
	client, err := NewRazorpayClientAt(testCredentials, server.Client(), server.URL+"/v1")
	if err != nil {
		t.Fatal(err)
	}
	order, err := client.CreateOrder(context.Background(), OrderRequest{Amount: 125050, Currency: "INR", Receipt: "AR-TEST-RECEIPT"})
	if err != nil {
		t.Fatal(err)
	}
	if order.ID != "order_test_123456" || order.Amount != 125050 {
		t.Fatalf("provider order=%+v", order)
	}
}

func TestRazorpayClientFetchesPaymentAndRejectsProviderStatusMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/payments/pay_test_123456" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"id":"pay_test_123456","entity":"payment","order_id":"order_test_123456","amount":125050,"currency":"INR","status":"captured","captured":true}`)
	}))
	defer server.Close()
	client, err := NewRazorpayClientAt(testCredentials, server.Client(), server.URL+"/v1")
	if err != nil {
		t.Fatal(err)
	}
	payment, err := client.FetchPayment(context.Background(), "pay_test_123456")
	if err != nil {
		t.Fatal(err)
	}
	if payment.Status != "captured" || !payment.Captured {
		t.Fatalf("payment=%+v", payment)
	}
	if _, err := client.FetchPayment(context.Background(), "pay_test_123456/../other"); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("invalid payment ID err=%v", err)
	}
}

func TestRazorpayClientSanitizesProviderErrorsAndRejectsAmountMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"description":"sensitive provider diagnostic"}}`)
	}))
	defer server.Close()
	client, err := NewRazorpayClientAt(testCredentials, server.Client(), server.URL+"/v1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.CreateOrder(context.Background(), OrderRequest{Amount: 100, Currency: "INR", Receipt: "AR-TEST-RECEIPT"})
	if err == nil || strings.Contains(err.Error(), "sensitive provider diagnostic") {
		t.Fatalf("provider error was not safely sanitized: %v", err)
	}
}

func TestRazorpayClientRejectsOrderResponseWithWrongAmount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"id":"order_test_123456","entity":"order","amount":99,"currency":"INR","receipt":"AR-TEST-RECEIPT","status":"created"}`)
	}))
	defer server.Close()
	client, err := NewRazorpayClientAt(testCredentials, server.Client(), server.URL+"/v1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.CreateOrder(context.Background(), OrderRequest{Amount: 100, Currency: "INR", Receipt: "AR-TEST-RECEIPT"})
	if !errors.Is(err, ErrProviderResponse) {
		t.Fatalf("mismatched provider order err=%v", err)
	}
}

func TestRazorpaySignatureVerificationUsesExactMessages(t *testing.T) {
	client, err := NewRazorpayClientAt(testCredentials, nil, DefaultAPIBaseURL)
	if err != nil {
		t.Fatal(err)
	}
	orderID, paymentID := "order_test_123456", "pay_test_123456"
	checkoutSignature := CreateCheckoutSignature(orderID, paymentID, testCredentials.KeySecret)
	if !client.VerifyCheckoutSignature(orderID, paymentID, checkoutSignature) {
		t.Fatal("valid checkout signature rejected")
	}
	if client.VerifyCheckoutSignature(orderID, paymentID, strings.Repeat("0", 64)) {
		t.Fatal("invalid checkout signature accepted")
	}
	raw := []byte("{\"event\":\"payment.captured\", \"amount\":125050}")
	webhookSignature := SignWebhookBody(raw, testCredentials.WebhookSecret)
	if !client.VerifyWebhookSignature(raw, webhookSignature) {
		t.Fatal("valid webhook raw-body signature rejected")
	}
	if client.VerifyWebhookSignature([]byte(`{"event":"payment.captured","amount":125050}`), webhookSignature) {
		t.Fatal("reserialized webhook payload incorrectly validated")
	}
	if client.VerifyWebhookSignature(raw, "bad-signature") {
		t.Fatal("malformed webhook signature accepted")
	}
}

func TestRazorpayClientOnlyAllowsTestCredentialsAndFixedProductionEndpoint(t *testing.T) {
	bad := testCredentials
	bad.KeyID = "rzp_live_secret"
	if _, err := NewRazorpayClient(bad, nil); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("live key was accepted: %v", err)
	}
	if _, err := NewRazorpayClientAt(testCredentials, nil, "https://attacker.example/v1"); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("untrusted endpoint was accepted: %v", err)
	}
	if _, err := NewRazorpayClientAt(testCredentials, nil, "http://razorpay.com/v1"); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("non-HTTPS endpoint was accepted: %v", err)
	}
}
