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

func TestCatalogRepositoryPostgresIntegration(t *testing.T) {
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

	var categoryImage, actorID string
	if err := pool.QueryRow(ctx, `SELECT image_key FROM products ORDER BY sku LIMIT 1`).Scan(&categoryImage); err != nil {
		t.Fatalf("select existing catalog asset: %v", err)
	}
	unique := fmt.Sprintf("%d", time.Now().UnixNano())
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,display_name) VALUES($1,'Catalog integration test') RETURNING id::text`, "catalog-test-"+unique+"@example.invalid").Scan(&actorID); err != nil {
		t.Fatalf("create disposable actor: %v", err)
	}
	var categoryID, productID string
	t.Cleanup(func() { cleanupCatalogIntegration(t, pool, productID, categoryID, actorID) })

	repository := NewCatalogRepository(pool, 5*time.Second)
	category, err := repository.CreateCategory(ctx, CategoryWrite{Slug: "integration-category-" + unique, Name: "Integration Collection", Subtitle: "Temporary catalog verification", ImageKey: categoryImage, SortOrder: 999999, Active: true}, actorID, "catalog-test-create-category")
	if err != nil {
		t.Fatalf("create category: %v", err)
	}
	categoryID = category.ID
	if category.Version != 1 || !category.Active || category.ActiveProductCount != 0 {
		t.Fatalf("created category=%+v", category)
	}

	productInput := ProductWrite{SKU: "IT-CATALOG-" + unique, Slug: "integration-product-" + unique, Name: "Integration Product", Description: "Temporary and unpublished after test", CategoryID: category.ID, PricePaise: 1550000, ImageKey: categoryImage, Status: "active", InventoryPolicy: "made_to_order", Variants: []VariantWrite{{SKU: "IT-CATALOG-V-" + unique, OptionName: "Fit", OptionValue: "Made to measure", Active: true, IsDefault: true}}}
	product, err := repository.CreateProduct(ctx, productInput, actorID, "catalog-test-create-product")
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	productID = product.ID
	if product.Version != 1 || len(product.Variants) != 1 || !product.Variants[0].IsDefault {
		t.Fatalf("created product=%+v", product)
	}

	listed, err := repository.ListProducts(ctx, CatalogFilter{Search: product.SKU, Status: "active", CategoryID: category.ID, Limit: 10})
	if err != nil || len(listed) != 1 || listed[0].DefaultVariantID != product.Variants[0].ID {
		t.Fatalf("listed products=%+v err=%v", listed, err)
	}
	imageOptions, err := repository.ListImageOptions(ctx)
	if err != nil {
		t.Fatalf("list image options: %v", err)
	}
	foundImage := false
	for _, option := range imageOptions {
		if option.ImageKey == categoryImage {
			foundImage = true
			break
		}
	}
	if !foundImage {
		t.Fatal("existing catalog image is not selectable")
	}

	productInput.ExpectedVersion = 1
	productInput.Name = "Integration Product Updated"
	productInput.Variants[0].ID = product.Variants[0].ID
	productInput.Variants = append(productInput.Variants, VariantWrite{SKU: "IT-CATALOG-S-" + unique, OptionName: "Size", OptionValue: "S", Active: true})
	product, err = repository.UpdateProduct(ctx, product.ID, productInput, actorID, "catalog-test-update-product")
	if err != nil {
		t.Fatalf("update product and add variant: %v", err)
	}
	if product.Version != 2 || len(product.Variants) != 2 {
		t.Fatalf("updated product version/variants=%d/%d", product.Version, len(product.Variants))
	}
	if _, err := repository.UpdateProduct(ctx, product.ID, productInput, actorID, "catalog-test-stale-product"); !errors.Is(err, ErrCatalogStale) {
		t.Fatalf("stale product update error=%v", err)
	}

	categoryInput := CategoryWrite{ExpectedVersion: 1, Slug: category.Slug, Name: category.Name, Subtitle: category.Subtitle, ImageKey: category.ImageKey, SortOrder: category.SortOrder, Active: false}
	if _, err := repository.UpdateCategory(ctx, category.ID, categoryInput, actorID, "catalog-test-category-in-use"); !errors.Is(err, ErrCategoryHasProducts) {
		t.Fatalf("deactivate category with live product error=%v", err)
	}

	productInput.ExpectedVersion = 2
	productInput.Status = "archived"
	productInput.Variants = product.VariantsAsWrites()
	product, err = repository.UpdateProduct(ctx, product.ID, productInput, actorID, "catalog-test-archive-product")
	if err != nil {
		t.Fatalf("archive product: %v", err)
	}
	category, err = repository.UpdateCategory(ctx, category.ID, categoryInput, actorID, "catalog-test-deactivate-category")
	if err != nil {
		t.Fatalf("deactivate empty category: %v", err)
	}
	if category.Active || category.Version != 2 || category.ActiveProductCount != 0 {
		t.Fatalf("updated category=%+v", category)
	}

	var auditCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE entity_id IN ($1,$2) AND action IN ('category.create','category.update','product.create','product.update')`, category.ID, product.ID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 5 {
		t.Fatalf("audit entries=%d, want 5", auditCount)
	}
	cleanupCatalogIntegration(t, pool, productID, categoryID, actorID)
	var leftProducts, leftCategories int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM products WHERE id=$1`, productID).Scan(&leftProducts); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM categories WHERE id=$1`, categoryID).Scan(&leftCategories); err != nil {
		t.Fatal(err)
	}
	if leftProducts != 0 || leftCategories != 0 {
		t.Fatalf("test cleanup left product/category=%d/%d", leftProducts, leftCategories)
	}
}

func (p AdminProductDetail) VariantsAsWrites() []VariantWrite {
	variants := make([]VariantWrite, 0, len(p.Variants))
	for _, variant := range p.Variants {
		variants = append(variants, VariantWrite{ID: variant.ID, SKU: variant.SKU, OptionName: variant.OptionName, OptionValue: variant.OptionValue, PriceAdjustmentPaise: variant.PriceAdjustmentPaise, Active: variant.Active, IsDefault: variant.IsDefault})
	}
	return variants
}

func cleanupCatalogIntegration(t *testing.T, pool *pgxpool.Pool, productID, categoryID, actorID string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if productID != "" {
		_, _ = pool.Exec(ctx, `DELETE FROM audit_logs WHERE entity_id=$1`, productID)
		_, _ = pool.Exec(ctx, `DELETE FROM product_images WHERE product_id=$1`, productID)
		_, _ = pool.Exec(ctx, `DELETE FROM product_variants WHERE product_id=$1`, productID)
		_, _ = pool.Exec(ctx, `DELETE FROM products WHERE id=$1`, productID)
	}
	if categoryID != "" {
		_, _ = pool.Exec(ctx, `DELETE FROM audit_logs WHERE entity_id=$1`, categoryID)
		_, _ = pool.Exec(ctx, `DELETE FROM categories WHERE id=$1`, categoryID)
	}
	if actorID != "" {
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	}
}
