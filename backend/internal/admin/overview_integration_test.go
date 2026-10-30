package admin

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

func TestOverviewSalesCountOnlyCapturedAttempts(t *testing.T) {
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
	var firstOrderID, secondOrderID string
	orderIDs := make([]string, 0, 2)
	t.Cleanup(func() {
		for _, orderID := range orderIDs {
			_, _ = pool.Exec(context.Background(), `DELETE FROM payment_events WHERE payment_attempt_id IN (SELECT id FROM payment_attempts WHERE order_id=$1)`, orderID)
			_, _ = pool.Exec(context.Background(), `DELETE FROM refunds WHERE payment_attempt_id IN (SELECT id FROM payment_attempts WHERE order_id=$1)`, orderID)
			_, _ = pool.Exec(context.Background(), `DELETE FROM payment_attempts WHERE order_id=$1`, orderID)
			_, _ = pool.Exec(context.Background(), `DELETE FROM orders WHERE id=$1`, orderID)
		}
	})

	service, err := NewOverviewRepository(pool, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := service.Get(ctx)
	if err != nil {
		t.Fatalf("read overview baseline: %v", err)
	}
	createOrder := func(orderNumber, status, idem string, createdAt time.Time) string {
		t.Helper()
		var id string
		if err := pool.QueryRow(ctx, `INSERT INTO orders(order_number,guest_email,guest_phone,status,currency,subtotal_paise,total_paise,shipping_address,idempotency_key,created_at)
			VALUES($1,'overview-test@example.invalid','9876500000',$2,'INR',1234,1234,'{}'::jsonb,$3,$4) RETURNING id::text`, orderNumber, status, idem, createdAt).Scan(&id); err != nil {
			t.Fatalf("create overview test order: %v", err)
		}
		orderIDs = append(orderIDs, id)
		return id
	}
	firstOrderID = createOrder("AR-OV-CAPTURED-"+unique, "paid", "overview-captured-"+unique, time.Now().Add(time.Minute))
	secondOrderID = createOrder("AR-OV-PENDING-"+unique, "pending_payment", "overview-pending-"+unique, time.Now().Add(2*time.Minute))
	var capturedAttemptID string
	if err := pool.QueryRow(ctx, `INSERT INTO payment_attempts(order_id,provider,status,amount_paise,idempotency_key)
		VALUES($1,'razorpay','captured',1234,$2) RETURNING id::text`, firstOrderID, "overview-payment-captured-"+unique).Scan(&capturedAttemptID); err != nil {
		t.Fatalf("create overview test payment attempts: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO payment_attempts(order_id,provider,status,amount_paise,idempotency_key)
		VALUES($1,'razorpay','pending',9876,$2)`, secondOrderID, "overview-payment-pending-"+unique); err != nil {
		t.Fatalf("create pending test payment attempt: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO refunds(payment_attempt_id,amount_paise,status,idempotency_key)
		VALUES($1,234,'processed',$2)`, capturedAttemptID, "overview-refund-"+unique); err != nil {
		t.Fatalf("create processed test refund: %v", err)
	}

	actual, err := service.Get(ctx)
	if err != nil {
		t.Fatalf("read overview after fixtures: %v", err)
	}
	if actual.GrossCapturedPaise-baseline.GrossCapturedPaise != 1234 || actual.ProcessedRefundsPaise-baseline.ProcessedRefundsPaise != 234 || actual.NetSalesPaise-baseline.NetSalesPaise != 1000 || actual.CapturedPayments-baseline.CapturedPayments != 1 {
		t.Fatalf("gross captured delta=%d processed refunds delta=%d net sales delta=%d captured payment delta=%d", actual.GrossCapturedPaise-baseline.GrossCapturedPaise, actual.ProcessedRefundsPaise-baseline.ProcessedRefundsPaise, actual.NetSalesPaise-baseline.NetSalesPaise, actual.CapturedPayments-baseline.CapturedPayments)
	}
	if actual.PendingPaymentOrders-baseline.PendingPaymentOrders != 1 {
		t.Fatalf("pending orders delta=%d", actual.PendingPaymentOrders-baseline.PendingPaymentOrders)
	}
	found := false
	for _, item := range actual.RecentOrders {
		if item.OrderNumber == "AR-OV-PENDING-"+unique {
			found = true
			if item.PaymentStatus != "pending" {
				t.Fatalf("recent order payment status=%q", item.PaymentStatus)
			}
		}
	}
	if !found {
		t.Fatal("most recent test order was missing from the overview")
	}
}
