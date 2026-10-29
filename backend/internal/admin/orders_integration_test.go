package admin

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAdminOrderRepositoryPostgresIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ARAMYA_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("set ARAMYA_TEST_DATABASE_URL to a disposable local migrated database to run PostgreSQL integration checks")
	}
	if os.Getenv("ARAMYA_TEST_DATABASE_CONFIRM") != "ephemeral-local" {
		t.Fatal("set ARAMYA_TEST_DATABASE_CONFIRM=ephemeral-local only for an isolated local test database")
	}
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1" {
		t.Fatal("integration database must use a loopback host")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
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
	var actorID string
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,display_name,phone) VALUES($1,'Order workflow test','9876500000') RETURNING id::text`, "admin-order-test-"+unique+"@example.invalid").Scan(&actorID); err != nil {
		t.Fatalf("create test actor/customer: %v", err)
	}
	var paidOrderID, pendingOrderID string
	t.Cleanup(func() { cleanupAdminOrders(t, pool, paidOrderID, pendingOrderID, actorID) })
	var productID, variantID, productName, productSKU string
	if err := pool.QueryRow(ctx, `SELECT p.id::text,v.id::text,p.name,p.sku FROM products p JOIN product_variants v ON v.product_id=p.id AND v.is_default ORDER BY p.sku LIMIT 1`).Scan(&productID, &variantID, &productName, &productSKU); err != nil {
		t.Fatalf("select seeded order item: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO orders(order_number,customer_id,status,subtotal_paise,total_paise,shipping_address,idempotency_key) VALUES($1,$2,'paid',200000,200000,'{"recipientName":"Test Customer","phone":"9876500000","line1":"12 Silk Street","city":"Ankola","region":"Karnataka","postalCode":"581314","countryCode":"IN"}'::jsonb,$3) RETURNING id::text`, "AR-TEST-"+unique, actorID, "admin-paid-"+unique).Scan(&paidOrderID); err != nil {
		t.Fatalf("create paid test order: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO order_items(order_id,product_id,product_variant_id,product_name,product_sku,variant_description,quantity,unit_price_paise,line_total_paise) VALUES($1,$2,$3,$4,$5,'Made to measure',1,200000,200000)`, paidOrderID, productID, variantID, productName, productSKU); err != nil {
		t.Fatalf("insert paid order snapshot: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO order_status_events(order_id,to_status,reason) VALUES($1,'paid','Verified test fixture')`, paidOrderID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO orders(order_number,customer_id,status,subtotal_paise,total_paise,shipping_address,idempotency_key) VALUES($1,$2,'pending_payment',0,0,'{}'::jsonb,$3) RETURNING id::text`, "AR-PENDING-"+unique, actorID, "admin-pending-"+unique).Scan(&pendingOrderID); err != nil {
		t.Fatalf("create pending test order: %v", err)
	}
	var pendingItemID string
	if err := pool.QueryRow(ctx, `INSERT INTO order_items(order_id,product_id,product_variant_id,product_name,product_sku,variant_description,quantity,unit_price_paise,line_total_paise) VALUES($1,$2,$3,$4,$5,'Made to measure',1,0,0) RETURNING id::text`, pendingOrderID, productID, variantID, productName, productSKU).Scan(&pendingItemID); err != nil {
		t.Fatalf("insert pending order snapshot: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO inventory_reservations(order_item_id,product_variant_id,quantity,status,expires_at) VALUES($1,$2,1,'active',now()+interval '1 hour')`, pendingItemID, variantID); err != nil {
		t.Fatalf("insert pending order reservation: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO payment_attempts(order_id,provider,status,amount_paise,idempotency_key) VALUES($1,'razorpay','initiated',1,$2)`, pendingOrderID, "admin-pending-payment-"+unique); err != nil {
		t.Fatal(err)
	}

	repository, err := NewOrderRepository(pool, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	items, err := repository.List(ctx, OrderFilter{Search: "AR-TEST-" + unique, Limit: 10})
	if err != nil || len(items) != 1 || items[0].Status != "paid" || items[0].CustomerEmail == "" {
		t.Fatalf("order search=%+v err=%v", items, err)
	}
	detail, err := repository.Get(ctx, paidOrderID)
	if err != nil || len(detail.Events) != 1 || len(detail.Items) != 1 || detail.Items[0].ProductSKU != productSKU || detail.ShippingAddress["city"] != "Ankola" {
		t.Fatalf("order detail=%+v err=%v", detail, err)
	}

	if _, err := repository.Update(ctx, paidOrderID, OrderUpdate{ExpectedVersion: 1, Status: "shipped", Reason: "Skip required states", ShipmentProvider: "Aramya Logistics", TrackingNumber: "TRK-123"}, actorID, "order-integration-invalid-transition"); !errors.Is(err, ErrOrderState) {
		t.Fatalf("skipped transition err=%v", err)
	}
	if _, err := repository.Update(ctx, pendingOrderID, OrderUpdate{ExpectedVersion: 1, Status: "cancelled", Reason: "Customer requested cancellation"}, actorID, "order-integration-pending-payment"); !errors.Is(err, ErrOrderPayment) {
		t.Fatalf("cancel order with open payment err=%v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE payment_attempts SET status='failed' WHERE order_id=$1`, pendingOrderID); err != nil {
		t.Fatal(err)
	}
	cancelled, err := repository.Update(ctx, pendingOrderID, OrderUpdate{ExpectedVersion: 1, Status: "cancelled", Reason: "Customer requested cancellation"}, actorID, "order-integration-cancel-pending")
	if err != nil || cancelled.Status != "cancelled" {
		t.Fatalf("cancel failed payment order=%+v err=%v", cancelled.AdminOrder, err)
	}
	var reservationStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM inventory_reservations WHERE order_item_id=$1`, pendingItemID).Scan(&reservationStatus); err != nil || reservationStatus != "released" {
		t.Fatalf("cancelled reservation status=%q err=%v", reservationStatus, err)
	}

	steps := []OrderUpdate{
		{ExpectedVersion: 1, Status: "confirmed", Reason: "Payment verified by provider"},
		{ExpectedVersion: 2, Status: "processing", Reason: "Workshop started preparation"},
		{ExpectedVersion: 3, Status: "ready_to_ship", Reason: "Order packed and checked"},
		{ExpectedVersion: 4, Status: "shipped", Reason: "Carrier collected parcel", ShipmentProvider: "Aramya Logistics", TrackingNumber: "TRK-" + unique, TrackingURL: "https://track.example.test/TRK-" + unique},
		{ExpectedVersion: 5, Status: "delivered", Reason: "Delivery confirmed by carrier"},
	}
	for _, step := range steps {
		updated, err := repository.Update(ctx, paidOrderID, step, actorID, "order-integration-"+step.Status)
		if err != nil {
			t.Fatalf("transition to %s: %v", step.Status, err)
		}
		if updated.Status != step.Status || updated.Version != step.ExpectedVersion+1 {
			t.Fatalf("updated order=%+v, want status %s version %d", updated.AdminOrder, step.Status, step.ExpectedVersion+1)
		}
	}
	final, err := repository.Get(ctx, paidOrderID)
	if err != nil || final.ShipmentStatus != "delivered" || final.ShipmentProvider != "Aramya Logistics" || len(final.Events) != 6 {
		t.Fatalf("final order=%+v err=%v", final, err)
	}
	if _, err := repository.Update(ctx, paidOrderID, steps[len(steps)-1], actorID, "order-integration-stale"); !errors.Is(err, ErrOrderStale) {
		t.Fatalf("duplicate stale transition err=%v", err)
	}
	var auditCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE entity_type='order' AND entity_id=$1 AND actor_user_id=$2`, paidOrderID, actorID).Scan(&auditCount); err != nil || auditCount != 5 {
		t.Fatalf("order audit rows=%d err=%v", auditCount, err)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM orders WHERE id=$1`, pendingOrderID).Scan(&final.Status); err != nil || final.Status != "cancelled" {
		t.Fatalf("resolved pending payment order status=%s err=%v", final.Status, err)
	}

	cleanupAdminOrders(t, pool, paidOrderID, pendingOrderID, actorID)
	paidOrderID, pendingOrderID, actorID = "", "", ""
}

func cleanupAdminOrders(t *testing.T, pool *pgxpool.Pool, orderIDs ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	for _, id := range orderIDs[:2] {
		if id == "" {
			continue
		}
		_, _ = pool.Exec(ctx, `DELETE FROM audit_logs WHERE entity_type='order' AND entity_id=$1`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM order_status_events WHERE order_id=$1`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM shipments WHERE order_id=$1`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM inventory_reservations WHERE order_item_id IN (SELECT id FROM order_items WHERE order_id=$1)`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM refunds WHERE payment_attempt_id IN (SELECT id FROM payment_attempts WHERE order_id=$1)`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM payment_events WHERE payment_attempt_id IN (SELECT id FROM payment_attempts WHERE order_id=$1)`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM payment_attempts WHERE order_id=$1`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM order_items WHERE order_id=$1`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM orders WHERE id=$1`, id)
	}
	actorID := orderIDs[2]
	if actorID != "" {
		_, _ = pool.Exec(ctx, `UPDATE orders SET customer_id=NULL WHERE customer_id=$1`, actorID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	}
}
