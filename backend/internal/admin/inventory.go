package admin

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInventoryNotFound = errors.New("inventory variant not found")
	ErrInventoryStale    = errors.New("inventory changed since it was loaded")
	ErrInventoryPolicy   = errors.New("only tracked inventory can be adjusted")
	ErrInventoryReserved = errors.New("on-hand quantity cannot be lower than reserved quantity")
	ErrInventoryReason   = errors.New("an adjustment reason is required")
	ErrInventoryQuantity = errors.New("inventory quantity must be between 0 and 1000000")
)

type InventoryItem struct {
	VariantID       string    `json:"variantId"`
	ProductID       string    `json:"productId"`
	SKU             string    `json:"sku"`
	ProductName     string    `json:"productName"`
	ProductSlug     string    `json:"productSlug"`
	CategoryName    string    `json:"categoryName"`
	OptionName      string    `json:"optionName"`
	OptionValue     string    `json:"optionValue"`
	PricePaise      int64     `json:"pricePaise"`
	InventoryPolicy string    `json:"inventoryPolicy"`
	Active          bool      `json:"active"`
	OnHand          int       `json:"onHand"`
	Reserved        int       `json:"reserved"`
	Available       int       `json:"available"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type InventoryRepository struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewInventoryRepository(pool *pgxpool.Pool, queryTimeout time.Duration) *InventoryRepository {
	return &InventoryRepository{pool: pool, queryTimeout: queryTimeout}
}

func (r *InventoryRepository) ListInventory(ctx context.Context, filter ListFilter) ([]InventoryItem, error) {
	dbCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	rows, err := r.pool.Query(dbCtx, `SELECT v.id::text,p.id::text,v.sku,p.name,p.slug,c.name,v.option_name,v.option_value,
		p.price_paise,p.inventory_policy,v.active,coalesce(i.on_hand,0),coalesce(i.reserved,0),
		coalesce(i.on_hand-i.reserved,0),coalesce(i.updated_at,v.updated_at)
		FROM product_variants v
		JOIN products p ON p.id=v.product_id
		JOIN categories c ON c.id=p.category_id
		LEFT JOIN inventory i ON i.product_variant_id=v.id
		WHERE $1='' OR p.name ILIKE '%'||$1||'%' OR p.slug ILIKE '%'||$1||'%' OR c.name ILIKE '%'||$1||'%' OR p.sku ILIKE '%'||$1||'%' OR v.sku ILIKE '%'||$1||'%' OR v.option_value ILIKE '%'||$1||'%'
		ORDER BY p.name,v.is_default DESC,v.option_name,v.option_value,v.id
		LIMIT $2 OFFSET $3`, filter.Search, filter.Limit, filter.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]InventoryItem, 0)
	for rows.Next() {
		var item InventoryItem
		if err := rows.Scan(&item.VariantID, &item.ProductID, &item.SKU, &item.ProductName, &item.ProductSlug,
			&item.CategoryName, &item.OptionName, &item.OptionValue, &item.PricePaise, &item.InventoryPolicy,
			&item.Active, &item.OnHand, &item.Reserved, &item.Available, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// SetOnHand uses the variant row as a stable serialization lock, including when
// inventory has not been initialized yet. expectedOnHand prevents lost updates;
// each successful change writes both the movement ledger and audit log.
func (r *InventoryRepository) SetOnHand(ctx context.Context, variantID string, expectedOnHand, targetOnHand int, reason, actorID, requestID string) (InventoryItem, error) {
	dbCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	reason = strings.TrimSpace(reason)
	if len(reason) < 3 || len(reason) > 500 || !utf8.ValidString(reason) {
		return InventoryItem{}, ErrInventoryReason
	}
	if ValidateInventoryQuantity(expectedOnHand) != nil || ValidateInventoryQuantity(targetOnHand) != nil {
		return InventoryItem{}, ErrInventoryQuantity
	}
	tx, err := r.pool.BeginTx(dbCtx, pgx.TxOptions{})
	if err != nil {
		return InventoryItem{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var item InventoryItem
	err = tx.QueryRow(dbCtx, `SELECT v.id::text,p.id::text,v.sku,p.name,p.slug,c.name,v.option_name,v.option_value,
		p.price_paise,p.inventory_policy,v.active
		FROM product_variants v JOIN products p ON p.id=v.product_id JOIN categories c ON c.id=p.category_id
		WHERE v.id=$1 FOR UPDATE OF v,p`, variantID).Scan(&item.VariantID, &item.ProductID, &item.SKU,
		&item.ProductName, &item.ProductSlug, &item.CategoryName, &item.OptionName, &item.OptionValue,
		&item.PricePaise, &item.InventoryPolicy, &item.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return InventoryItem{}, ErrInventoryNotFound
	}
	if err != nil {
		return InventoryItem{}, err
	}
	if item.InventoryPolicy != "tracked" {
		return InventoryItem{}, ErrInventoryPolicy
	}
	var updatedAt time.Time
	err = tx.QueryRow(dbCtx, `SELECT on_hand,reserved,updated_at FROM inventory WHERE product_variant_id=$1 FOR UPDATE`, variantID).
		Scan(&item.OnHand, &item.Reserved, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		item.OnHand, item.Reserved = 0, 0
	} else if err != nil {
		return InventoryItem{}, err
	}
	if item.OnHand != expectedOnHand {
		return InventoryItem{}, ErrInventoryStale
	}
	if targetOnHand < item.Reserved {
		return InventoryItem{}, ErrInventoryReserved
	}
	if targetOnHand == item.OnHand {
		item.Available = item.OnHand - item.Reserved
		item.UpdatedAt = updatedAt
		if err := tx.Commit(dbCtx); err != nil {
			return InventoryItem{}, err
		}
		return item, nil
	}

	previous := item.OnHand
	err = tx.QueryRow(dbCtx, `INSERT INTO inventory(product_variant_id,on_hand,reserved)
		VALUES($1,$2,0) ON CONFLICT(product_variant_id) DO UPDATE SET on_hand=EXCLUDED.on_hand,updated_at=now()
		RETURNING on_hand,reserved,updated_at`, variantID, targetOnHand).Scan(&item.OnHand, &item.Reserved, &item.UpdatedAt)
	if err != nil {
		return InventoryItem{}, err
	}
	item.Available = item.OnHand - item.Reserved
	if _, err = tx.Exec(dbCtx, `INSERT INTO inventory_movements(product_variant_id,movement_type,quantity_delta,quantity_after,actor_user_id,note)
		VALUES($1,'adjustment',$2,$3,$4,$5)`, variantID, targetOnHand-previous, targetOnHand, actorID, reason); err != nil {
		return InventoryItem{}, err
	}
	metadata, _ := json.Marshal(map[string]any{"quantityDelta": targetOnHand - previous, "quantityAfter": targetOnHand})
	if _, err = tx.Exec(dbCtx, `INSERT INTO audit_logs(actor_user_id,action,entity_type,entity_id,request_id,metadata)
		VALUES($1,'inventory.adjust','product_variant',$2,$3,$4::jsonb)`, actorID, variantID, requestID, string(metadata)); err != nil {
		return InventoryItem{}, err
	}
	if err := tx.Commit(dbCtx); err != nil {
		return InventoryItem{}, err
	}
	return item, nil
}

func ValidateInventoryQuantity(value int) error {
	if value < 0 || value > 1_000_000 {
		return ErrInventoryQuantity
	}
	return nil
}
