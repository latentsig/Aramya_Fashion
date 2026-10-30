package payments

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRazorpayWebhookInboxCaptureReplayAndOutOfOrderEvents(t *testing.T) {
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
	var orderID, attemptID string
	if err := pool.QueryRow(ctx, `INSERT INTO orders(order_number,guest_email,guest_phone,status,currency,subtotal_paise,total_paise,shipping_address,idempotency_key)
		VALUES($1,'webhook-test@example.invalid','9876500000','pending_payment','INR',125050,125050,'{}'::jsonb,$2) RETURNING id::text`, "AR-WEB-"+unique, "guest:8bc6d910-2785-4be1-8a86-1b9cbef20001:"+unique).Scan(&orderID); err != nil {
		t.Fatalf("insert webhook test order: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO payment_attempts(order_id,provider,status,amount_paise,currency,provider_order_id,idempotency_key)
		VALUES($1,'razorpay','pending',125050,'INR',$2,$3) RETURNING id::text`, orderID, "order_webhook_"+unique, "webhook_"+unique).Scan(&attemptID); err != nil {
		t.Fatalf("insert payment attempt: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_logs WHERE entity_type='order' AND entity_id=$1`, orderID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM order_status_events WHERE order_id=$1`, orderID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM payment_events WHERE payment_attempt_id=$1 OR provider_event_id LIKE $2`, attemptID, "%"+unique)
		_, _ = pool.Exec(context.Background(), `DELETE FROM payment_attempts WHERE id=$1`, attemptID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM orders WHERE id=$1`, orderID)
	})
	provider, err := NewRazorpayClientAt(testCredentials, nil, "https://example.invalid/v1")
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewCheckoutPaymentService(pool, provider, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	captured := fmt.Sprintf(`{"event":"payment.captured","payload":{"payment":{"entity":{"id":"pay_webhook_%s","order_id":"order_webhook_%s","amount":125050,"currency":"INR","status":"captured","captured":true}}}}`, unique, unique)
	const capturedEventID = "evt_webhook_capture_123456"
	if _, err := service.ProcessWebhook(ctx, capturedEventID, SignWebhookBody([]byte(captured), testCredentials.WebhookSecret), []byte(captured)); err != nil {
		t.Fatalf("process capture webhook: %v", err)
	}
	duplicate, err := service.ProcessWebhook(ctx, capturedEventID, SignWebhookBody([]byte(captured), testCredentials.WebhookSecret), []byte(captured))
	if err != nil || !duplicate.Duplicate {
		t.Fatalf("capture replay = %+v, err=%v", duplicate, err)
	}
	conflict := strings.Replace(captured, "125050", "125051", 1)
	if _, err := service.ProcessWebhook(ctx, capturedEventID, SignWebhookBody([]byte(conflict), testCredentials.WebhookSecret), []byte(conflict)); err != ErrWebhookConflict {
		t.Fatalf("conflicting replay error = %v", err)
	}
	authorized := fmt.Sprintf(`{"event":"payment.authorized","payload":{"payment":{"entity":{"id":"pay_webhook_%s","order_id":"order_webhook_%s","amount":125050,"currency":"INR","status":"authorized","captured":false}}}}`, unique, unique)
	if _, err := service.ProcessWebhook(ctx, "evt_webhook_authorized_123456", SignWebhookBody([]byte(authorized), testCredentials.WebhookSecret), []byte(authorized)); err != nil {
		t.Fatalf("process out-of-order authorization: %v", err)
	}
	additionalCapture := fmt.Sprintf(`{"event":"payment.captured","payload":{"payment":{"entity":{"id":"pay_webhook_second_%s","order_id":"order_webhook_%s","amount":125050,"currency":"INR","status":"captured","captured":true}}}}`, unique, unique)
	if _, err := service.ProcessWebhook(ctx, "evt_webhook_second_capture_123456", SignWebhookBody([]byte(additionalCapture), testCredentials.WebhookSecret), []byte(additionalCapture)); err != nil {
		t.Fatalf("process additional captured payment: %v", err)
	}
	var orderStatus, paymentStatus string
	if err := pool.QueryRow(ctx, `SELECT o.status,pa.status FROM orders o JOIN payment_attempts pa ON pa.order_id=o.id WHERE o.id=$1`, orderID).Scan(&orderStatus, &paymentStatus); err != nil {
		t.Fatal(err)
	}
	if orderStatus != "paid" || paymentStatus != "captured" {
		t.Fatalf("out-of-order event regressed state: order=%s payment=%s", orderStatus, paymentStatus)
	}
	var paidEvents, inboxEvents, additionalCaptureAudits int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM order_status_events WHERE order_id=$1 AND to_status='paid'`, orderID).Scan(&paidEvents); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM payment_events WHERE payment_attempt_id=$1 AND processed_at IS NOT NULL`, attemptID).Scan(&inboxEvents); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE action='payment.additional_capture_review_required' AND entity_id=$1`, orderID).Scan(&additionalCaptureAudits); err != nil {
		t.Fatal(err)
	}
	var storedPaymentID string
	if err := pool.QueryRow(ctx, `SELECT provider_payment_id FROM payment_attempts WHERE id=$1`, attemptID).Scan(&storedPaymentID); err != nil {
		t.Fatal(err)
	}
	if paidEvents != 1 || inboxEvents != 3 || additionalCaptureAudits != 1 || storedPaymentID != "pay_webhook_"+unique {
		t.Fatalf("paid transition=%d processed inbox events=%d extra-capture audits=%d stored payment=%s", paidEvents, inboxEvents, additionalCaptureAudits, storedPaymentID)
	}
}

func TestRazorpayReconciliationUsesProviderCapturedPayments(t *testing.T) {
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
	providerOrderID := "order_reconcile_" + unique
	var orderID, attemptID string
	if err := pool.QueryRow(ctx, `INSERT INTO orders(order_number,guest_email,guest_phone,status,currency,subtotal_paise,total_paise,shipping_address,idempotency_key)
		VALUES($1,'reconcile-test@example.invalid','9876500000','pending_payment','INR',125050,125050,'{}'::jsonb,$2) RETURNING id::text`, "AR-REC-"+unique, "guest:8bc6d910-2785-4be1-8a86-1b9cbef20001:"+unique).Scan(&orderID); err != nil {
		t.Fatalf("insert reconciliation order: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO payment_attempts(order_id,provider,status,amount_paise,currency,provider_order_id,idempotency_key,created_at,updated_at)
		VALUES($1,'razorpay','pending',125050,'INR',$2,$3,now()-interval '3 minutes',now()-interval '3 minutes') RETURNING id::text`, orderID, providerOrderID, "reconcile_"+unique).Scan(&attemptID); err != nil {
		t.Fatalf("insert reconciliation attempt: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_logs WHERE entity_type='order' AND entity_id=$1`, orderID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM order_status_events WHERE order_id=$1`, orderID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM payment_events WHERE payment_attempt_id=$1`, attemptID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM payment_attempts WHERE id=$1`, attemptID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM orders WHERE id=$1`, orderID)
	})
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/orders/"+providerOrderID+"/payments" || r.URL.Query().Get("count") != "100" {
			t.Errorf("unexpected provider request %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, fmt.Sprintf(`{"entity":"collection","count":1,"items":[{"id":"pay_reconcile_%s","order_id":%q,"amount":125050,"currency":"INR","status":"captured","captured":true}]}`, unique, providerOrderID))
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
	report, err := service.ReconcilePayments(ctx, 25)
	if err != nil || report.Checked != 1 || report.Captured != 1 || report.Errors != 0 {
		t.Fatalf("reconciliation report=%+v err=%v", report, err)
	}
	var orderStatus, attemptStatus string
	var reconciledAt, signatureVerifiedAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT o.status,pa.status,pa.last_reconciled_at,pa.signature_verified_at FROM orders o JOIN payment_attempts pa ON pa.order_id=o.id WHERE o.id=$1`, orderID).Scan(&orderStatus, &attemptStatus, &reconciledAt, &signatureVerifiedAt); err != nil {
		t.Fatal(err)
	}
	if orderStatus != "paid" || attemptStatus != "captured" || reconciledAt == nil || signatureVerifiedAt != nil {
		t.Fatalf("reconciled state order=%s attempt=%s time=%v callbackSignatureTime=%v", orderStatus, attemptStatus, reconciledAt, signatureVerifiedAt)
	}
}
