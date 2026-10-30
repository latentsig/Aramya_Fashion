package payments

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCheckoutPaymentCaptureUsesProviderStateAndIsIdempotent(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ARAMYA_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("set ARAMYA_TEST_DATABASE_URL to a disposable local migrated database to run PostgreSQL integration checks")
	}
	if os.Getenv("ARAMYA_TEST_DATABASE_CONFIRM") != "ephemeral-local" {
		t.Fatal("set ARAMYA_TEST_DATABASE_CONFIRM=ephemeral-local only for an isolated local test database")
	}
	parsed, err := url.Parse(dsn)
	if err != nil || (parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1") {
		t.Fatal("integration database must use a loopback host")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open integration database: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping integration database: %v", err)
	}

	unique := fmt.Sprintf("%d", time.Now().UnixNano())
	guestCartID := "8bc6d910-2785-4be1-8a86-1b9cbef20001"
	var orderID string
	if err := pool.QueryRow(ctx, `INSERT INTO orders(order_number,guest_email,guest_phone,status,currency,subtotal_paise,total_paise,shipping_address,idempotency_key)
		VALUES($1,'payment-test@example.invalid','9876500000','pending_payment','INR',125050,125050,'{}'::jsonb,$2) RETURNING id::text`, "AR-PAY-"+unique, "guest:"+guestCartID+":"+unique).Scan(&orderID); err != nil {
		t.Fatalf("insert payment test order: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM order_status_events WHERE order_id=$1`, orderID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM payment_events WHERE payment_attempt_id IN (SELECT id FROM payment_attempts WHERE order_id=$1)`, orderID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM refunds WHERE payment_attempt_id IN (SELECT id FROM payment_attempts WHERE order_id=$1)`, orderID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM payment_attempts WHERE order_id=$1`, orderID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM orders WHERE id=$1`, orderID)
	})

	var createCalls atomic.Int32
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost && r.URL.Path == "/v1/orders" {
			createCalls.Add(1)
			var request OrderRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode provider order: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if request.Amount != 125050 || request.Currency != "INR" {
				t.Errorf("provider amount/currency = %d/%s", request.Amount, request.Currency)
			}
			_, _ = io.WriteString(w, fmt.Sprintf(`{"id":"order_test_123456","entity":"order","amount":%d,"currency":"INR","receipt":%q,"status":"created"}`, request.Amount, request.Receipt))
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/v1/payments/pay_test_123456" {
			_, _ = io.WriteString(w, `{"id":"pay_test_123456","entity":"payment","order_id":"order_test_123456","amount":125050,"currency":"INR","status":"captured","captured":true}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer providerServer.Close()
	provider, err := NewRazorpayClientAt(testCredentials, providerServer.Client(), providerServer.URL+"/v1")
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewCheckoutPaymentService(pool, provider, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.Start(ctx, orderID, "", "wrong-cart-token-identifier-000000", "client-payment-key-0001"); err != ErrOrderUnavailable {
		t.Fatalf("unowned order start error = %v", err)
	}
	session, err := service.Start(ctx, orderID, "", guestCartID, "client-payment-key-0001")
	if err != nil {
		t.Fatalf("start payment: %v", err)
	}
	if session.ProviderOrderID != "order_test_123456" || session.AmountPaise != 125050 || session.Status != "pending" {
		t.Fatalf("session = %+v", session)
	}
	replayed, err := service.Start(ctx, orderID, "", guestCartID, "client-payment-key-0001")
	if err != nil || replayed.AttemptID != session.AttemptID || createCalls.Load() != 1 {
		t.Fatalf("idempotent start replay = %+v, err=%v, provider create calls=%d", replayed, err, createCalls.Load())
	}

	signature := CreateCheckoutSignature(session.ProviderOrderID, "pay_test_123456", testCredentials.KeySecret)
	result, err := service.Verify(ctx, orderID, "", guestCartID, session.ProviderOrderID, "pay_test_123456", signature)
	if err != nil {
		t.Fatalf("verify captured payment: %v", err)
	}
	if result.OrderStatus != "paid" || result.PaymentStatus != "captured" {
		t.Fatalf("verified result = %+v", result)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM orders WHERE id=$1`, orderID).Scan(&status); err != nil || status != "paid" {
		t.Fatalf("database order status=%q err=%v", status, err)
	}
	var paidEvents int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM order_status_events WHERE order_id=$1 AND to_status='paid'`, orderID).Scan(&paidEvents); err != nil || paidEvents != 1 {
		t.Fatalf("paid transition count=%d err=%v", paidEvents, err)
	}
	if _, err := service.Verify(ctx, orderID, "", guestCartID, session.ProviderOrderID, "pay_test_123456", signature); err != nil {
		t.Fatalf("duplicate callback: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM order_status_events WHERE order_id=$1 AND to_status='paid'`, orderID).Scan(&paidEvents); err != nil || paidEvents != 1 {
		t.Fatalf("duplicate callback added a paid transition: count=%d err=%v", paidEvents, err)
	}
}
