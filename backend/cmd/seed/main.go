package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type seedFile struct {
	Categories []category `json:"categories"`
	Products   []product  `json:"products"`
}

type category struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	Subtitle  string `json:"subtitle"`
	ImageKey  string `json:"image_key"`
	SortOrder int    `json:"sort_order"`
}

type product struct {
	ID              string `json:"id"`
	SKU             string `json:"sku"`
	Slug            string `json:"slug"`
	Name            string `json:"name"`
	CategorySlug    string `json:"category_slug"`
	PricePaise      int64  `json:"price_paise"`
	Currency        string `json:"currency"`
	ImageKey        string `json:"image_key"`
	Status          string `json:"status"`
	InventoryPolicy string `json:"inventory_policy"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	seedPath := os.Getenv("CATALOG_SEED_PATH")
	if seedPath == "" {
		seedPath = filepath.Join("..", "database", "seeds", "catalog.json")
	}
	data, err := os.ReadFile(seedPath)
	if err != nil {
		return fmt.Errorf("read catalog seed: %w", err)
	}
	var seed seedFile
	if err := json.Unmarshal(data, &seed); err != nil {
		return fmt.Errorf("decode catalog seed: %w", err)
	}
	if len(seed.Products) != 39 || len(seed.Categories) != 5 {
		return fmt.Errorf("refusing unexpected seed catalog size: %d products and %d categories", len(seed.Products), len(seed.Categories))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("configure database connection: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin seed transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	for _, item := range seed.Categories {
		_, err := tx.Exec(ctx, `INSERT INTO categories(id,slug,name,subtitle,image_key,sort_order) VALUES($1,$2,$3,$4,$5,$6)
			ON CONFLICT(slug) DO UPDATE SET name=EXCLUDED.name, subtitle=EXCLUDED.subtitle, image_key=EXCLUDED.image_key, sort_order=EXCLUDED.sort_order, updated_at=now()`,
			item.ID, item.Slug, item.Name, item.Subtitle, item.ImageKey, item.SortOrder)
		if err != nil {
			return fmt.Errorf("seed category %q: %w", item.Slug, err)
		}
	}
	for _, item := range seed.Products {
		_, err := tx.Exec(ctx, `INSERT INTO products(id,sku,slug,name,category_id,price_paise,currency,image_key,status,inventory_policy)
			SELECT $1,$2,$3,$4,c.id,$5,$6,$7,$8,$9 FROM categories c WHERE c.slug=$10
			ON CONFLICT(sku) DO UPDATE SET slug=EXCLUDED.slug, name=EXCLUDED.name, category_id=EXCLUDED.category_id,
			price_paise=EXCLUDED.price_paise, currency=EXCLUDED.currency, image_key=EXCLUDED.image_key,
			status=EXCLUDED.status, inventory_policy=EXCLUDED.inventory_policy, updated_at=now()`,
			item.ID, item.SKU, item.Slug, item.Name, item.PricePaise, item.Currency, item.ImageKey, item.Status, item.InventoryPolicy, item.CategorySlug)
		if err != nil {
			return fmt.Errorf("seed product %q: %w", item.SKU, err)
		}
		_, err = tx.Exec(ctx, `INSERT INTO product_variants(product_id,sku,option_name,option_value,is_default)
			SELECT id, sku || '-MTO', 'Fit', 'Made to measure', true FROM products WHERE sku=$1
			ON CONFLICT(sku) DO NOTHING`, item.SKU)
		if err != nil {
			return fmt.Errorf("seed default made-to-measure variant for %q: %w", item.SKU, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit catalog seed: %w", err)
	}
	fmt.Printf("Seeded %d categories and %d products; stock quantities remain unset.\n", len(seed.Categories), len(seed.Products))
	return nil
}
