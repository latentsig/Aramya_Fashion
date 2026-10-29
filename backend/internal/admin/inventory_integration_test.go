package admin

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// This integration test only connects when explicitly pointed at a loopback
// disposable database and confirmed by the operator.
func TestInventoryRepositoryPostgresIntegration(t *testing.T) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open integration database: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping integration database: %v", err)
	}

	var categoryID, productID, variantID, actorID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM categories ORDER BY id LIMIT 1`).Scan(&categoryID); err != nil {
		t.Fatalf("select development category: %v", err)
	}
	unique := fmt.Sprintf("%d", time.Now().UnixNano())
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,display_name) VALUES($1,'Inventory integration test') RETURNING id::text`, "inventory-test-"+unique+"@example.invalid").Scan(&actorID); err != nil {
		t.Fatalf("create disposable actor: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO products(id,sku,slug,name,category_id,price_paise,image_key,status,inventory_policy)
		VALUES(gen_random_uuid(),$1,$2,'Inventory integration test product',$3,10000,'integration-test/no-media','draft','tracked') RETURNING id::text`,
		"IT-INV-"+unique, "integration-inventory-"+unique, categoryID).Scan(&productID); err != nil {
		cleanupIntegrationInventory(t, pool, productID, variantID, actorID)
		t.Fatalf("create disposable product: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO product_variants(product_id,sku,option_name,option_value,is_default)
		VALUES($1,$2,'Size','Test',true) RETURNING id::text`, productID, "IT-INV-V-"+unique).Scan(&variantID); err != nil {
		cleanupIntegrationInventory(t, pool, productID, variantID, actorID)
		t.Fatalf("create disposable variant: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO inventory(product_variant_id,on_hand,reserved) VALUES($1,5,2)`, variantID); err != nil {
		cleanupIntegrationInventory(t, pool, productID, variantID, actorID)
		t.Fatalf("seed disposable quantity: %v", err)
	}
	t.Cleanup(func() { cleanupIntegrationInventory(t, pool, productID, variantID, actorID) })

	repository := NewInventoryRepository(pool, 5*time.Second)
	listed, err := repository.ListInventory(ctx, ListFilter{Search: "IT-INV-" + unique, Limit: 10})
	if err != nil || len(listed) != 1 || listed[0].OnHand != 5 || listed[0].Reserved != 2 || listed[0].Available != 3 {
		var matching int
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM product_variants v JOIN products p ON p.id=v.product_id JOIN categories c ON c.id=p.category_id WHERE p.sku=$1`, "IT-INV-"+unique).Scan(&matching)
		t.Fatalf("list inventory = %+v, err=%v, matching rows=%d", listed, err, matching)
	}
	updated, err := repository.SetOnHand(ctx, variantID, 5, 8, "Integration receipt", actorID, "inventory-integration")
	if err != nil || updated.OnHand != 8 || updated.Available != 6 {
		t.Fatalf("initial adjustment = %+v, err=%v", updated, err)
	}
	if _, err := repository.SetOnHand(ctx, variantID, 5, 9, "Stale adjustment", actorID, "inventory-integration-stale"); !errors.Is(err, ErrInventoryStale) {
		t.Fatalf("stale adjustment error=%v", err)
	}
	if _, err := repository.SetOnHand(ctx, variantID, 8, 1, "Below reservation", actorID, "inventory-integration-reserved"); !errors.Is(err, ErrInventoryReserved) {
		t.Fatalf("below-reserved adjustment error=%v", err)
	}

	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, target := range []int{9, 10} {
		wg.Add(1)
		go func(target int) {
			defer wg.Done()
			_, err := repository.SetOnHand(ctx, variantID, 8, target, "Concurrent count correction", actorID, fmt.Sprintf("inventory-concurrent-%d", target))
			results <- err
		}(target)
	}
	wg.Wait()
	close(results)
	succeeded, conflicted := 0, 0
	for err := range results {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrInventoryStale):
			conflicted++
		default:
			t.Fatalf("concurrent adjustment error=%v", err)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("concurrent adjustments: succeeded=%d conflicted=%d", succeeded, conflicted)
	}

	var movementCount, auditCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM inventory_movements WHERE product_variant_id=$1 AND movement_type='adjustment'`, variantID).Scan(&movementCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE entity_id=$1 AND action='inventory.adjust'`, variantID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if movementCount != 2 || auditCount != 2 {
		t.Fatalf("ledger/audit counts = %d/%d, want 2/2", movementCount, auditCount)
	}
	cleanupIntegrationInventory(t, pool, productID, variantID, actorID)
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM products WHERE id=$1`, productID).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("disposable product cleanup count=%d, err=%v", remaining, err)
	}
}

func cleanupIntegrationInventory(t *testing.T, pool *pgxpool.Pool, productID, variantID, actorID string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if variantID != "" {
		_, _ = pool.Exec(ctx, `DELETE FROM inventory_movements WHERE product_variant_id=$1`, variantID)
		_, _ = pool.Exec(ctx, `DELETE FROM audit_logs WHERE entity_id=$1`, variantID)
		_, _ = pool.Exec(ctx, `DELETE FROM inventory WHERE product_variant_id=$1`, variantID)
		_, _ = pool.Exec(ctx, `DELETE FROM product_variants WHERE id=$1`, variantID)
	}
	if productID != "" {
		_, _ = pool.Exec(ctx, `DELETE FROM products WHERE id=$1`, productID)
	}
	if actorID != "" {
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	}
}
