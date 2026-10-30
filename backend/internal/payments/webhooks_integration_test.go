package payments

import (
	"context"
	"fmt"
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
	var orderStatus, paymentStatus string
	if err := pool.QueryRow(ctx, `SELECT o.status,pa.status FROM orders o JOIN payment_attempts pa ON pa.order_id=o.id WHERE o.id=$1`, orderID).Scan(&orderStatus, &paymentStatus); err != nil {
		t.Fatal(err)
	}
	if orderStatus != "paid" || paymentStatus != "captured" {
		t.Fatalf("out-of-order event regressed state: order=%s payment=%s", orderStatus, paymentStatus)
	}
	var paidEvents, inboxEvents int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM order_status_events WHERE order_id=$1 AND to_status='paid'`, orderID).Scan(&paidEvents); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM payment_events WHERE payment_attempt_id=$1 AND processed_at IS NOT NULL`, attemptID).Scan(&inboxEvents); err != nil {
		t.Fatal(err)
	}
	if paidEvents != 1 || inboxEvents != 2 {
		t.Fatalf("paid transition=%d processed inbox events=%d", paidEvents, inboxEvents)
	}
}
