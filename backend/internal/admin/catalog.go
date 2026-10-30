package admin

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrCatalogNotFound     = errors.New("catalog record not found")
	ErrCatalogStale        = errors.New("catalog record changed since it was loaded")
	ErrCatalogConflict     = errors.New("catalog SKU or slug already exists")
	ErrCatalogImage        = errors.New("catalog image is not in the approved existing asset library")
	ErrCategoryInactive    = errors.New("active products require an active category")
	ErrCategoryHasProducts = errors.New("category still has active products")
	ErrInvalidCatalog      = errors.New("catalog input is invalid")
	adminSlugPattern       = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	adminSKUPattern        = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{1,63}$`)
	adminUUIDPattern       = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

type CatalogFilter struct {
	Search     string
	Status     string
	CategoryID string
	Limit      int
	Offset     int
}

type AdminCategory struct {
	ID                 string    `json:"id"`
	Slug               string    `json:"slug"`
	Name               string    `json:"name"`
	Subtitle           string    `json:"subtitle"`
	ImageKey           string    `json:"imageKey"`
	SortOrder          int       `json:"sortOrder"`
	Active             bool      `json:"active"`
	Version            int       `json:"version"`
	ActiveProductCount int       `json:"activeProductCount"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type AdminProductSummary struct {
	ID               string    `json:"id"`
	Version          int       `json:"version"`
	SKU              string    `json:"sku"`
	Slug             string    `json:"slug"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	CategoryID       string    `json:"categoryId"`
	CategoryName     string    `json:"categoryName"`
	PricePaise       int64     `json:"pricePaise"`
	Currency         string    `json:"currency"`
	ImageKey         string    `json:"imageKey"`
	Status           string    `json:"status"`
	InventoryPolicy  string    `json:"inventoryPolicy"`
	DefaultVariantID string    `json:"defaultVariantId"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type AdminVariant struct {
	ID                   string `json:"id"`
	SKU                  string `json:"sku"`
	OptionName           string `json:"optionName"`
	OptionValue          string `json:"optionValue"`
	PriceAdjustmentPaise int64  `json:"priceAdjustmentPaise"`
	Active               bool   `json:"active"`
	IsDefault            bool   `json:"isDefault"`
}

type AdminProductDetail struct {
	AdminProductSummary
	Variants []AdminVariant `json:"variants"`
}

type ProductWrite struct {
	ExpectedVersion int            `json:"expectedVersion,omitempty"`
	SKU             string         `json:"sku"`
	Slug            string         `json:"slug"`
	Name            string         `json:"name"`
	Description     string         `json:"description"`
	CategoryID      string         `json:"categoryId"`
	PricePaise      int64          `json:"pricePaise"`
	ImageKey        string         `json:"imageKey"`
	Status          string         `json:"status"`
	InventoryPolicy string         `json:"inventoryPolicy"`
	Variants        []VariantWrite `json:"variants"`
}

type VariantWrite struct {
	ID                   string `json:"id,omitempty"`
	SKU                  string `json:"sku"`
	OptionName           string `json:"optionName"`
	OptionValue          string `json:"optionValue"`
	PriceAdjustmentPaise int64  `json:"priceAdjustmentPaise"`
	Active               bool   `json:"active"`
	IsDefault            bool   `json:"isDefault"`
}

type CategoryWrite struct {
	ExpectedVersion int    `json:"expectedVersion,omitempty"`
	Slug            string `json:"slug"`
	Name            string `json:"name"`
	Subtitle        string `json:"subtitle"`
	ImageKey        string `json:"imageKey"`
	SortOrder       int    `json:"sortOrder"`
	Active          bool   `json:"active"`
}

type ImageOption struct {
	ImageKey string `json:"imageKey"`
	Label    string `json:"label"`
}

type CatalogRepository struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewCatalogRepository(pool *pgxpool.Pool, queryTimeout time.Duration) *CatalogRepository {
	return &CatalogRepository{pool: pool, queryTimeout: queryTimeout}
}

func ValidateProductWrite(input ProductWrite, creating bool) error {
	input.SKU, input.Slug, input.Name = strings.TrimSpace(input.SKU), strings.TrimSpace(input.Slug), strings.TrimSpace(input.Name)
	input.CategoryID, input.ImageKey = strings.TrimSpace(input.CategoryID), strings.TrimSpace(input.ImageKey)
	if !adminSKUPattern.MatchString(input.SKU) || len(input.Slug) < 2 || len(input.Slug) > 160 || !adminSlugPattern.MatchString(input.Slug) ||
		len(input.Name) < 2 || len(input.Name) > 160 || len(input.Description) > 5000 || !utf8.ValidString(input.Description) ||
		!uuidString(input.CategoryID) || !validAssetKey(input.ImageKey) || input.PricePaise < 0 || input.PricePaise > 100_000_000_000 ||
		input.Status != "draft" && input.Status != "active" && input.Status != "archived" ||
		input.InventoryPolicy != "tracked" && input.InventoryPolicy != "made_to_order" && input.InventoryPolicy != "untracked" ||
		!creating && input.ExpectedVersion < 1 || len(input.Variants) == 0 || len(input.Variants) > 50 {
		return ErrInvalidCatalog
	}
	if input.Status == "active" && input.InventoryPolicy == "tracked" && input.PricePaise == 0 {
		return ErrInvalidCatalog
	}
	seenIDs, seenSKUs, seenOptions := map[string]bool{}, map[string]bool{}, map[string]bool{}
	defaults, activeDefaults := 0, 0
	for _, variant := range input.Variants {
		if variant.ID != "" {
			if !uuidString(variant.ID) || creating || seenIDs[variant.ID] {
				return ErrInvalidCatalog
			}
			seenIDs[variant.ID] = true
		}
		sku, optionName, optionValue := strings.TrimSpace(variant.SKU), strings.TrimSpace(variant.OptionName), strings.TrimSpace(variant.OptionValue)
		optionKey := strings.ToLower(optionName + "\x00" + optionValue)
		if !adminSKUPattern.MatchString(sku) || seenSKUs[strings.ToUpper(sku)] || len(optionName) < 1 || len(optionName) > 60 || len(optionValue) < 1 || len(optionValue) > 100 ||
			!utf8.ValidString(optionName) || !utf8.ValidString(optionValue) || seenOptions[optionKey] || input.PricePaise+variant.PriceAdjustmentPaise < 0 || input.PricePaise+variant.PriceAdjustmentPaise > 100_000_000_000 {
			return ErrInvalidCatalog
		}
		if variant.PriceAdjustmentPaise < -100_000_000_000 || variant.PriceAdjustmentPaise > 100_000_000_000 {
			return ErrInvalidCatalog
		}
		seenSKUs[strings.ToUpper(sku)] = true
		seenOptions[optionKey] = true
		if variant.IsDefault {
			defaults++
			if variant.Active {
				activeDefaults++
			}
		}
	}
	if defaults != 1 || activeDefaults != 1 {
		return ErrInvalidCatalog
	}
	return nil
}

func NormalizeProductWrite(input ProductWrite) ProductWrite {
	input.SKU = strings.TrimSpace(input.SKU)
	input.Slug = strings.TrimSpace(input.Slug)
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.CategoryID = strings.TrimSpace(input.CategoryID)
	input.ImageKey = strings.TrimSpace(input.ImageKey)
	for index := range input.Variants {
		input.Variants[index].SKU = strings.TrimSpace(input.Variants[index].SKU)
		input.Variants[index].OptionName = strings.TrimSpace(input.Variants[index].OptionName)
		input.Variants[index].OptionValue = strings.TrimSpace(input.Variants[index].OptionValue)
	}
	return input
}

func ValidateCategoryWrite(input CategoryWrite, creating bool) error {
	input.Slug, input.Name, input.ImageKey = strings.TrimSpace(input.Slug), strings.TrimSpace(input.Name), strings.TrimSpace(input.ImageKey)
	if len(input.Slug) < 2 || len(input.Slug) > 120 || !adminSlugPattern.MatchString(input.Slug) || len(input.Name) < 2 || len(input.Name) > 120 ||
		len(input.Subtitle) > 300 || !utf8.ValidString(input.Subtitle) || !validAssetKey(input.ImageKey) || input.SortOrder < 0 || input.SortOrder > 1000000 || !creating && input.ExpectedVersion < 1 {
		return ErrInvalidCatalog
	}
	return nil
}

func NormalizeCategoryWrite(input CategoryWrite) CategoryWrite {
	input.Slug = strings.TrimSpace(input.Slug)
	input.Name = strings.TrimSpace(input.Name)
	input.Subtitle = strings.TrimSpace(input.Subtitle)
	input.ImageKey = strings.TrimSpace(input.ImageKey)
	return input
}

func (r *CatalogRepository) ListCategories(ctx context.Context) ([]AdminCategory, error) {
	dbCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	rows, err := r.pool.Query(dbCtx, `SELECT c.id::text,c.slug,c.name,c.subtitle,c.image_key,c.sort_order,c.active,c.version,
		(count(p.id) FILTER (WHERE p.status='active'))::int,c.updated_at
		FROM categories c LEFT JOIN products p ON p.category_id=c.id
		GROUP BY c.id ORDER BY c.sort_order,c.name,c.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]AdminCategory, 0)
	for rows.Next() {
		var item AdminCategory
		if err := rows.Scan(&item.ID, &item.Slug, &item.Name, &item.Subtitle, &item.ImageKey, &item.SortOrder, &item.Active, &item.Version, &item.ActiveProductCount, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *CatalogRepository) ListProducts(ctx context.Context, filter CatalogFilter) ([]AdminProductSummary, error) {
	dbCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	rows, err := r.pool.Query(dbCtx, `SELECT p.id::text,p.version,p.sku,p.slug,p.name,coalesce(p.description,''),p.category_id::text,c.name,
		p.price_paise,p.currency,p.image_key,p.status,p.inventory_policy,coalesce(v.id::text,''),p.updated_at
		FROM products p JOIN categories c ON c.id=p.category_id
		LEFT JOIN product_variants v ON v.product_id=p.id AND v.is_default
		WHERE ($1='' OR p.status=$1) AND ($2='' OR p.category_id=$2::uuid)
		AND ($3='' OR p.name ILIKE '%'||$3||'%' OR p.slug ILIKE '%'||$3||'%' OR p.sku ILIKE '%'||$3||'%' OR c.name ILIKE '%'||$3||'%')
		ORDER BY p.updated_at DESC,p.id LIMIT $4 OFFSET $5`, filter.Status, filter.CategoryID, filter.Search, filter.Limit, filter.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]AdminProductSummary, 0)
	for rows.Next() {
		var item AdminProductSummary
		if err := rows.Scan(&item.ID, &item.Version, &item.SKU, &item.Slug, &item.Name, &item.Description, &item.CategoryID, &item.CategoryName, &item.PricePaise, &item.Currency, &item.ImageKey, &item.Status, &item.InventoryPolicy, &item.DefaultVariantID, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *CatalogRepository) GetProduct(ctx context.Context, id string) (AdminProductDetail, error) {
	dbCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	var item AdminProductDetail
	err := r.pool.QueryRow(dbCtx, `SELECT p.id::text,p.version,p.sku,p.slug,p.name,coalesce(p.description,''),p.category_id::text,c.name,p.price_paise,p.currency,p.image_key,p.status,p.inventory_policy,coalesce(v.id::text,''),p.updated_at
		FROM products p JOIN categories c ON c.id=p.category_id LEFT JOIN product_variants v ON v.product_id=p.id AND v.is_default WHERE p.id=$1`, id).
		Scan(&item.ID, &item.Version, &item.SKU, &item.Slug, &item.Name, &item.Description, &item.CategoryID, &item.CategoryName, &item.PricePaise, &item.Currency, &item.ImageKey, &item.Status, &item.InventoryPolicy, &item.DefaultVariantID, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminProductDetail{}, ErrCatalogNotFound
	}
	if err != nil {
		return AdminProductDetail{}, err
	}
	rows, err := r.pool.Query(dbCtx, `SELECT id::text,sku,option_name,option_value,price_adjustment_paise,active,is_default FROM product_variants WHERE product_id=$1 ORDER BY is_default DESC,option_name,option_value,id`, id)
	if err != nil {
		return AdminProductDetail{}, err
	}
	defer rows.Close()
	item.Variants = make([]AdminVariant, 0)
	for rows.Next() {
		var v AdminVariant
		if err := rows.Scan(&v.ID, &v.SKU, &v.OptionName, &v.OptionValue, &v.PriceAdjustmentPaise, &v.Active, &v.IsDefault); err != nil {
			return AdminProductDetail{}, err
		}
		item.Variants = append(item.Variants, v)
	}
	return item, rows.Err()
}

func (r *CatalogRepository) ListImageOptions(ctx context.Context) ([]ImageOption, error) {
	dbCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	rows, err := r.pool.Query(dbCtx, `SELECT image_key FROM (SELECT image_key FROM products UNION SELECT image_key FROM categories UNION SELECT object_key AS image_key FROM product_images UNION SELECT object_key AS image_key FROM media_objects WHERE status='approved') assets ORDER BY image_key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ImageOption, 0)
	for rows.Next() {
		var item ImageOption
		if err := rows.Scan(&item.ImageKey); err != nil {
			return nil, err
		}
		item.Label = assetLabel(item.ImageKey)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *CatalogRepository) CreateCategory(ctx context.Context, input CategoryWrite, actorID, requestID string) (AdminCategory, error) {
	dbCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	tx, err := r.pool.Begin(dbCtx)
	if err != nil {
		return AdminCategory{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := ensureExistingImage(dbCtx, tx, input.ImageKey); err != nil {
		return AdminCategory{}, err
	}
	var item AdminCategory
	err = tx.QueryRow(dbCtx, `INSERT INTO categories(id,slug,name,subtitle,image_key,sort_order,active) VALUES(gen_random_uuid(),$1,$2,$3,$4,$5,$6)
		RETURNING id::text,slug,name,subtitle,image_key,sort_order,active,version,0,updated_at`, input.Slug, input.Name, input.Subtitle, input.ImageKey, input.SortOrder, input.Active).
		Scan(&item.ID, &item.Slug, &item.Name, &item.Subtitle, &item.ImageKey, &item.SortOrder, &item.Active, &item.Version, &item.ActiveProductCount, &item.UpdatedAt)
	if err != nil {
		return AdminCategory{}, catalogWriteError(err)
	}
	if err := writeCatalogAudit(dbCtx, tx, actorID, requestID, "category.create", "category", item.ID, map[string]any{"version": item.Version}); err != nil {
		return AdminCategory{}, err
	}
	if err := tx.Commit(dbCtx); err != nil {
		return AdminCategory{}, err
	}
	return item, nil
}

func (r *CatalogRepository) UpdateCategory(ctx context.Context, id string, input CategoryWrite, actorID, requestID string) (AdminCategory, error) {
	dbCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	tx, err := r.pool.Begin(dbCtx)
	if err != nil {
		return AdminCategory{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var currentVersion int
	var currentActive bool
	err = tx.QueryRow(dbCtx, `SELECT version,active FROM categories WHERE id=$1 FOR UPDATE`, id).Scan(&currentVersion, &currentActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminCategory{}, ErrCatalogNotFound
	}
	if err != nil {
		return AdminCategory{}, err
	}
	if currentVersion != input.ExpectedVersion {
		return AdminCategory{}, ErrCatalogStale
	}
	if currentActive && !input.Active {
		var count int
		if err := tx.QueryRow(dbCtx, `SELECT count(*) FROM products WHERE category_id=$1 AND status='active'`, id).Scan(&count); err != nil {
			return AdminCategory{}, err
		}
		if count > 0 {
			return AdminCategory{}, ErrCategoryHasProducts
		}
	}
	if err := ensureExistingImage(dbCtx, tx, input.ImageKey); err != nil {
		return AdminCategory{}, err
	}
	var item AdminCategory
	err = tx.QueryRow(dbCtx, `UPDATE categories SET slug=$2,name=$3,subtitle=$4,image_key=$5,sort_order=$6,active=$7,version=version+1,updated_at=now()
		WHERE id=$1 RETURNING id::text,slug,name,subtitle,image_key,sort_order,active,version,
		(SELECT count(*)::int FROM products WHERE category_id=$1 AND status='active'),updated_at`, id, input.Slug, input.Name, input.Subtitle, input.ImageKey, input.SortOrder, input.Active).
		Scan(&item.ID, &item.Slug, &item.Name, &item.Subtitle, &item.ImageKey, &item.SortOrder, &item.Active, &item.Version, &item.ActiveProductCount, &item.UpdatedAt)
	if err != nil {
		return AdminCategory{}, catalogWriteError(err)
	}
	if err := writeCatalogAudit(dbCtx, tx, actorID, requestID, "category.update", "category", item.ID, map[string]any{"version": item.Version, "active": item.Active}); err != nil {
		return AdminCategory{}, err
	}
	if err := tx.Commit(dbCtx); err != nil {
		return AdminCategory{}, err
	}
	return item, nil
}

func (r *CatalogRepository) CreateProduct(ctx context.Context, input ProductWrite, actorID, requestID string) (AdminProductDetail, error) {
	dbCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	tx, err := r.pool.Begin(dbCtx)
	if err != nil {
		return AdminProductDetail{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := checkProductCategory(dbCtx, tx, input.CategoryID, input.Status); err != nil {
		return AdminProductDetail{}, err
	}
	if err := ensureExistingImage(dbCtx, tx, input.ImageKey); err != nil {
		return AdminProductDetail{}, err
	}
	var productID string
	err = tx.QueryRow(dbCtx, `INSERT INTO products(id,sku,slug,name,description,category_id,price_paise,currency,image_key,status,inventory_policy)
		VALUES(gen_random_uuid(),$1,$2,$3,NULLIF($4,''),$5,$6,'INR',$7,$8,$9) RETURNING id::text`, input.SKU, input.Slug, input.Name, input.Description, input.CategoryID, input.PricePaise, input.ImageKey, input.Status, input.InventoryPolicy).Scan(&productID)
	if err != nil {
		return AdminProductDetail{}, catalogWriteError(err)
	}
	if err := saveProductVariants(dbCtx, tx, productID, input.Variants, false); err != nil {
		return AdminProductDetail{}, catalogWriteError(err)
	}
	if err := savePrimaryImage(dbCtx, tx, productID, input.ImageKey, input.Name); err != nil {
		return AdminProductDetail{}, err
	}
	if err := writeCatalogAudit(dbCtx, tx, actorID, requestID, "product.create", "product", productID, map[string]any{"status": input.Status, "variantCount": len(input.Variants)}); err != nil {
		return AdminProductDetail{}, err
	}
	if err := tx.Commit(dbCtx); err != nil {
		return AdminProductDetail{}, err
	}
	return r.GetProduct(ctx, productID)
}

func (r *CatalogRepository) UpdateProduct(ctx context.Context, id string, input ProductWrite, actorID, requestID string) (AdminProductDetail, error) {
	dbCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	tx, err := r.pool.Begin(dbCtx)
	if err != nil {
		return AdminProductDetail{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var version int
	err = tx.QueryRow(dbCtx, `SELECT version FROM products WHERE id=$1 FOR UPDATE`, id).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminProductDetail{}, ErrCatalogNotFound
	}
	if err != nil {
		return AdminProductDetail{}, err
	}
	if version != input.ExpectedVersion {
		return AdminProductDetail{}, ErrCatalogStale
	}
	if err := checkProductCategory(dbCtx, tx, input.CategoryID, input.Status); err != nil {
		return AdminProductDetail{}, err
	}
	if err := ensureExistingImage(dbCtx, tx, input.ImageKey); err != nil {
		return AdminProductDetail{}, err
	}
	var updatedVersion int
	err = tx.QueryRow(dbCtx, `UPDATE products SET sku=$2,slug=$3,name=$4,description=NULLIF($5,''),category_id=$6,price_paise=$7,image_key=$8,status=$9,inventory_policy=$10,version=version+1,updated_at=now()
		WHERE id=$1 RETURNING version`, id, input.SKU, input.Slug, input.Name, input.Description, input.CategoryID, input.PricePaise, input.ImageKey, input.Status, input.InventoryPolicy).Scan(&updatedVersion)
	if err != nil {
		return AdminProductDetail{}, catalogWriteError(err)
	}
	if err := saveProductVariants(dbCtx, tx, id, input.Variants, true); err != nil {
		return AdminProductDetail{}, catalogWriteError(err)
	}
	if err := savePrimaryImage(dbCtx, tx, id, input.ImageKey, input.Name); err != nil {
		return AdminProductDetail{}, err
	}
	if err := writeCatalogAudit(dbCtx, tx, actorID, requestID, "product.update", "product", id, map[string]any{"status": input.Status, "version": updatedVersion, "variantCount": len(input.Variants)}); err != nil {
		return AdminProductDetail{}, err
	}
	if err := tx.Commit(dbCtx); err != nil {
		return AdminProductDetail{}, err
	}
	return r.GetProduct(ctx, id)
}

func saveProductVariants(ctx context.Context, tx pgx.Tx, productID string, variants []VariantWrite, existing bool) error {
	if existing {
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM product_variants WHERE product_id=$1`, productID).Scan(&count); err != nil {
			return err
		}
		provided := 0
		for _, v := range variants {
			if v.ID != "" {
				provided++
			}
		}
		if count != provided {
			return ErrInvalidCatalog
		}
		if _, err := tx.Exec(ctx, `UPDATE product_variants SET is_default=false WHERE product_id=$1`, productID); err != nil {
			return err
		}
	}
	for _, v := range variants {
		if v.ID == "" {
			_, err := tx.Exec(ctx, `INSERT INTO product_variants(product_id,sku,option_name,option_value,price_adjustment_paise,active,is_default) VALUES($1,$2,$3,$4,$5,$6,$7)`, productID, v.SKU, strings.TrimSpace(v.OptionName), strings.TrimSpace(v.OptionValue), v.PriceAdjustmentPaise, v.Active, v.IsDefault)
			if err != nil {
				return err
			}
			continue
		}
		result, err := tx.Exec(ctx, `UPDATE product_variants SET sku=$3,option_name=$4,option_value=$5,price_adjustment_paise=$6,active=$7,is_default=$8,version=version+1,updated_at=now() WHERE product_id=$1 AND id=$2`, productID, v.ID, v.SKU, strings.TrimSpace(v.OptionName), strings.TrimSpace(v.OptionValue), v.PriceAdjustmentPaise, v.Active, v.IsDefault)
		if err != nil {
			return err
		}
		if result.RowsAffected() != 1 {
			return ErrCatalogNotFound
		}
	}
	return nil
}

func checkProductCategory(ctx context.Context, tx pgx.Tx, id, status string) error {
	var active bool
	err := tx.QueryRow(ctx, `SELECT active FROM categories WHERE id=$1 FOR SHARE`, id).Scan(&active)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrCatalogNotFound
	}
	if err != nil {
		return err
	}
	if status == "active" && !active {
		return ErrCategoryInactive
	}
	return nil
}
func ensureExistingImage(ctx context.Context, tx pgx.Tx, key string) error {
	var exists bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM products WHERE image_key=$1 UNION ALL SELECT 1 FROM categories WHERE image_key=$1 UNION ALL SELECT 1 FROM product_images WHERE object_key=$1 UNION ALL SELECT 1 FROM media_objects WHERE object_key=$1 AND status='approved')`, key).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrCatalogImage
	}
	return nil
}
func savePrimaryImage(ctx context.Context, tx pgx.Tx, productID, key, alt string) error {
	if _, err := tx.Exec(ctx, `UPDATE product_images SET is_primary=false WHERE product_id=$1 AND is_primary`, productID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO product_images(product_id,object_key,alt_text,sort_order,is_primary) VALUES($1,$2,$3,0,true) ON CONFLICT(product_id,object_key) DO UPDATE SET alt_text=EXCLUDED.alt_text,sort_order=0,is_primary=true`, productID, key, alt)
	return err
}
func writeCatalogAudit(ctx context.Context, tx pgx.Tx, actorID, requestID, action, entityType, entityID string, metadata map[string]any) error {
	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_logs(actor_user_id,action,entity_type,entity_id,request_id,metadata) VALUES($1,$2,$3,$4,$5,$6::jsonb)`, actorID, action, entityType, entityID, requestID, string(data))
	return err
}
func catalogWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrCatalogConflict
	}
	return err
}
func uuidString(value string) bool { return adminUUIDPattern.MatchString(value) }
func validAssetKey(value string) bool {
	return value != "" && len(value) <= 512 && utf8.ValidString(value) && !strings.Contains(value, "..") && !strings.HasPrefix(value, "/") && !strings.Contains(value, "\\")
}
func assetLabel(key string) string {
	if index := strings.LastIndexAny(key, "/\\"); index >= 0 {
		return key[index+1:]
	}
	return key
}
