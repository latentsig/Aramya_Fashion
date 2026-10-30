package checkout

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

func TestExpiredReservationsReleaseOnlyWithoutUnresolvedPayments(t *testing.T) {
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
	var categoryID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM categories ORDER BY id LIMIT 1`).Scan(&categoryID); err != nil {
		t.Fatalf("select development category: %v", err)
	}
	unique := fmt.Sprintf("%d", time.Now().UnixNano())
	var productID, variantID string
	orderIDs := make([]string, 0, 2)
	cleanup := func() {
		for _, orderID := range orderIDs {
			_, _ = pool.Exec(context.Background(), `DELETE FROM audit_logs WHERE entity_type='order' AND entity_id=$1`, orderID)
			_, _ = pool.Exec(context.Background(), `DELETE FROM order_status_events WHERE order_id=$1`, orderID)
			_, _ = pool.Exec(context.Background(), `DELETE FROM payment_events WHERE payment_attempt_id IN (SELECT id FROM payment_attempts WHERE order_id=$1)`, orderID)
			_, _ = pool.Exec(context.Background(), `DELETE FROM payment_attempts WHERE order_id=$1`, orderID)
			_, _ = pool.Exec(context.Background(), `DELETE FROM inventory_movements WHERE reference_type='order' AND reference_id=$1`, orderID)
			_, _ = pool.Exec(context.Background(), `DELETE FROM inventory_reservations WHERE order_item_id IN (SELECT id FROM order_items WHERE order_id=$1)`, orderID)
			_, _ = pool.Exec(context.Background(), `DELETE FROM order_items WHERE order_id=$1`, orderID)
			_, _ = pool.Exec(context.Background(), `DELETE FROM orders WHERE id=$1`, orderID)
		}
		if variantID != "" {
			_, _ = pool.Exec(context.Background(), `DELETE FROM inventory WHERE product_variant_id=$1`, variantID)
			_, _ = pool.Exec(context.Background(), `DELETE FROM product_variants WHERE id=$1`, variantID)
		}
		if productID != "" {
			_, _ = pool.Exec(context.Background(), `DELETE FROM products WHERE id=$1`, productID)
		}
	}
	t.Cleanup(cleanup)
	if err := pool.QueryRow(ctx, `INSERT INTO products(sku,slug,name,description,category_id,price_paise,image_key,status,inventory_policy)
		VALUES($1,$2,'Reservation integration product','',$3,10000,'test/no-media','draft','tracked') RETURNING id::text`, "IT-RES-"+unique, "integration-reservation-"+unique, categoryID).Scan(&productID); err != nil {
		t.Fatalf("create test product: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO product_variants(product_id,sku,option_name,option_value,is_default)
		VALUES($1,$2,'Size','Test',true) RETURNING id::text`, productID, "IT-RES-V-"+unique).Scan(&variantID); err != nil {
		t.Fatalf("create test variant: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO inventory(product_variant_id,on_hand,reserved) VALUES($1,10,3)`, variantID); err != nil {
		t.Fatalf("create test inventory: %v", err)
	}
	createFixtureOrder := func(suffix string, quantity int) (string, string) {
		t.Helper()
		var orderID, itemID string
		if err := pool.QueryRow(ctx, `INSERT INTO orders(order_number,guest_email,guest_phone,status,currency,subtotal_paise,total_paise,shipping_address,idempotency_key)
			VALUES($1,'reservation-test@example.invalid','9876500000','pending_payment','INR',$2,$2,'{}'::jsonb,$3) RETURNING id::text`, "AR-RES-"+suffix+unique, quantity*10000, "reservation-idem-"+suffix+unique).Scan(&orderID); err != nil {
			t.Fatalf("create test order: %v", err)
		}
		orderIDs = append(orderIDs, orderID)
		if err := pool.QueryRow(ctx, `INSERT INTO order_items(order_id,product_id,product_variant_id,product_name,product_sku,variant_description,quantity,unit_price_paise,line_total_paise)
			VALUES($1,$2,$3,'Reservation integration product',$4,'Size: Test',$5,10000,$6) RETURNING id::text`, orderID, productID, variantID, "IT-RES-V-"+unique, quantity, quantity*10000).Scan(&itemID); err != nil {
			t.Fatalf("create test order line: %v", err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO inventory_reservations(order_item_id,product_variant_id,quantity,status,expires_at)
			VALUES($1,$2,$3,'active',now()-interval '1 minute')`, itemID, variantID, quantity); err != nil {
			t.Fatalf("create expired reservation: %v", err)
		}
		return orderID, itemID
	}
	orderExpiredID, _ := createFixtureOrder("expired-", 2)
	orderUnresolvedID, _ := createFixtureOrder("open-", 1)
	if _, err := pool.Exec(ctx, `INSERT INTO payment_attempts(order_id,provider,status,amount_paise,provider_order_id,idempotency_key)
		VALUES($1,'razorpay','pending',10000,$2,$3)`, orderUnresolvedID, "order_test_expiration_"+unique, "payment-attempt-"+unique); err != nil {
		t.Fatalf("create unresolved payment attempt: %v", err)
	}

	service, err := NewService(pool, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	count, err := service.ReleaseExpiredReservations(ctx, 10)
	if err != nil || count != 1 {
		t.Fatalf("expired orders=%d err=%v", count, err)
	}
	var orderStatus, reservationStatus string
	var onHand, reserved, releasedMovements int
	if err := pool.QueryRow(ctx, `SELECT status FROM orders WHERE id=$1`, orderExpiredID).Scan(&orderStatus); err != nil {
		t.Fatal(err)
	}
	if orderStatus != "cancelled" {
		t.Fatalf("expired order status=%s", orderStatus)
	}
	if err := pool.QueryRow(ctx, `SELECT ir.status,i.on_hand,i.reserved FROM inventory_reservations ir JOIN order_items oi ON oi.id=ir.order_item_id JOIN inventory i ON i.product_variant_id=ir.product_variant_id WHERE oi.order_id=$1`, orderExpiredID).Scan(&reservationStatus, &onHand, &reserved); err != nil {
		t.Fatal(err)
	}
	if reservationStatus != "expired" || onHand != 10 || reserved != 1 {
		t.Fatalf("expired reservation=%s on_hand=%d reserved=%d", reservationStatus, onHand, reserved)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM inventory_movements WHERE reference_type='order' AND reference_id=$1 AND movement_type='release'`, orderExpiredID).Scan(&releasedMovements); err != nil {
		t.Fatal(err)
	}
	if releasedMovements != 1 {
		t.Fatalf("release ledger rows=%d", releasedMovements)
	}
	if err := pool.QueryRow(ctx, `SELECT o.status,ir.status,i.reserved FROM orders o JOIN order_items oi ON oi.order_id=o.id JOIN inventory_reservations ir ON ir.order_item_id=oi.id JOIN inventory i ON i.product_variant_id=ir.product_variant_id WHERE o.id=$1`, orderUnresolvedID).Scan(&orderStatus, &reservationStatus, &reserved); err != nil {
		t.Fatal(err)
	}
	if orderStatus != "pending_payment" || reservationStatus != "active" || reserved != 1 {
		t.Fatalf("unresolved payment was released: order=%s reservation=%s reserved=%d", orderStatus, reservationStatus, reserved)
	}
	count, err = service.ReleaseExpiredReservations(ctx, 10)
	if err != nil || count != 0 {
		t.Fatalf("second expiration pass orders=%d err=%v", count, err)
	}
}
