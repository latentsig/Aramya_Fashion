package catalog

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("catalog item not found")

type Category struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	Subtitle  string `json:"subtitle"`
	ImageKey  string `json:"imageKey"`
	SortOrder int    `json:"sortOrder"`
}

type Product struct {
	ID               string  `json:"id"`
	DefaultVariantID string  `json:"defaultVariantId"`
	SKU              string  `json:"sku"`
	Slug             string  `json:"slug"`
	Name             string  `json:"name"`
	Description      *string `json:"description"`
	CategoryID       string  `json:"categoryId"`
	CategorySlug     string  `json:"categorySlug"`
	CategoryName     string  `json:"categoryName"`
	PricePaise       int64   `json:"pricePaise"`
	Currency         string  `json:"currency"`
	ImageKey         string  `json:"imageKey"`
	Status           string  `json:"status"`
	InventoryPolicy  string  `json:"inventoryPolicy"`
}

type ProductVariant struct {
	ID                   string `json:"id"`
	SKU                  string `json:"sku"`
	OptionName           string `json:"optionName"`
	OptionValue          string `json:"optionValue"`
	PriceAdjustmentPaise int64  `json:"priceAdjustmentPaise"`
	IsDefault            bool   `json:"isDefault"`
	Available            bool   `json:"available"`
}

type ProductImage struct {
	ImageKey string `json:"imageKey"`
	AltText  string `json:"altText"`
}

type ProductDetail struct {
	Product
	Variants []ProductVariant `json:"variants"`
	Images   []ProductImage   `json:"images"`
}

type Repository struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewRepository(pool *pgxpool.Pool, queryTimeout time.Duration) *Repository {
	return &Repository{pool: pool, queryTimeout: queryTimeout}
}

func (r *Repository) ListCategories(ctx context.Context) ([]Category, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	rows, err := r.pool.Query(ctx, `SELECT id::text, slug, name, subtitle, image_key, sort_order FROM categories WHERE active = true ORDER BY sort_order, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Category, 0)
	for rows.Next() {
		var item Category
		if err := rows.Scan(&item.ID, &item.Slug, &item.Name, &item.Subtitle, &item.ImageKey, &item.SortOrder); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) ListProducts(ctx context.Context, category, search string, limit, offset int) ([]Product, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	categoryFilter := blankToNil(category)
	searchFilter := blankToNil(search)
	rows, err := r.pool.Query(ctx, `
		SELECT p.id::text, v.id::text, p.sku, p.slug, p.name, p.description, p.category_id::text,
		       c.slug, c.name, p.price_paise, p.currency, p.image_key, p.status, p.inventory_policy
		FROM products p JOIN categories c ON c.id = p.category_id JOIN product_variants v ON v.product_id=p.id AND v.is_default AND v.active
		WHERE p.status = 'active'
		  AND ($1::text IS NULL OR c.slug = $1)
		  AND ($2::text IS NULL OR p.name ILIKE '%' || $2 || '%' OR coalesce(p.description, '') ILIKE '%' || $2 || '%')
		ORDER BY p.name LIMIT $3 OFFSET $4`, categoryFilter, searchFilter, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Product, 0)
	for rows.Next() {
		item, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) GetProduct(ctx context.Context, slug string) (Product, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	row := r.pool.QueryRow(ctx, `
		SELECT p.id::text, v.id::text, p.sku, p.slug, p.name, p.description, p.category_id::text,
		       c.slug, c.name, p.price_paise, p.currency, p.image_key, p.status, p.inventory_policy
		FROM products p JOIN categories c ON c.id = p.category_id JOIN product_variants v ON v.product_id=p.id AND v.is_default AND v.active
		WHERE p.status = 'active' AND p.slug = $1`, slug)
	product, err := scanProduct(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Product{}, ErrNotFound
	}
	return product, err
}

func (r *Repository) GetProductDetail(ctx context.Context, slug string) (ProductDetail, error) {
	product, err := r.GetProduct(ctx, slug)
	if err != nil {
		return ProductDetail{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	rows, err := r.pool.Query(ctx, `
		SELECT v.id::text, v.sku, v.option_name, v.option_value, v.price_adjustment_paise, v.is_default,
		       CASE WHEN p.inventory_policy = 'tracked' THEN coalesce(i.on_hand - i.reserved, 0) > 0 ELSE true END
		FROM product_variants v
		JOIN products p ON p.id = v.product_id
		LEFT JOIN inventory i ON i.product_variant_id = v.id
		WHERE p.id = $1::uuid AND v.active
		ORDER BY v.is_default DESC, v.option_name, v.option_value`, product.ID)
	if err != nil {
		return ProductDetail{}, err
	}
	detail := ProductDetail{Product: product, Variants: make([]ProductVariant, 0), Images: make([]ProductImage, 0)}
	for rows.Next() {
		var variant ProductVariant
		if err := rows.Scan(&variant.ID, &variant.SKU, &variant.OptionName, &variant.OptionValue, &variant.PriceAdjustmentPaise, &variant.IsDefault, &variant.Available); err != nil {
			rows.Close()
			return ProductDetail{}, err
		}
		detail.Variants = append(detail.Variants, variant)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return ProductDetail{}, err
	}
	rows.Close()

	imageRows, err := r.pool.Query(ctx, `
		SELECT object_key, alt_text
		FROM product_images
		WHERE product_id = $1::uuid
		ORDER BY is_primary DESC, sort_order, id`, product.ID)
	if err != nil {
		return ProductDetail{}, err
	}
	defer imageRows.Close()
	for imageRows.Next() {
		var image ProductImage
		if err := imageRows.Scan(&image.ImageKey, &image.AltText); err != nil {
			return ProductDetail{}, err
		}
		detail.Images = append(detail.Images, image)
	}
	if err := imageRows.Err(); err != nil {
		return ProductDetail{}, err
	}
	if len(detail.Images) == 0 {
		detail.Images = append(detail.Images, ProductImage{ImageKey: product.ImageKey, AltText: product.Name})
	}
	return detail, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanProduct(row scanner) (Product, error) {
	var item Product
	err := row.Scan(&item.ID, &item.DefaultVariantID, &item.SKU, &item.Slug, &item.Name, &item.Description, &item.CategoryID,
		&item.CategorySlug, &item.CategoryName, &item.PricePaise, &item.Currency, &item.ImageKey, &item.Status, &item.InventoryPolicy)
	return item, err
}

func blankToNil(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}
