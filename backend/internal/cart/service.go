package cart

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrProductUnavailable = errors.New("product variant is unavailable")
	ErrQuantity           = errors.New("quantity must be between 1 and 20")
	ErrOutOfStock         = errors.New("requested quantity is not available")
	ErrItemNotFound       = errors.New("cart item not found")
)

type Line struct {
	ID             string `json:"id"`
	VariantID      string `json:"variantId"`
	ProductID      string `json:"productId"`
	Slug           string `json:"slug"`
	Name           string `json:"name"`
	Category       string `json:"category"`
	CategoryName   string `json:"categoryName"`
	ImageKey       string `json:"imageKey"`
	Currency       string `json:"currency"`
	UnitPricePaise int64  `json:"unitPricePaise"`
	Quantity       int    `json:"quantity"`
	LineTotalPaise int64  `json:"lineTotalPaise"`
}

type Snapshot struct {
	Lines         []Line `json:"lines"`
	Count         int    `json:"count"`
	SubtotalPaise int64  `json:"subtotalPaise"`
	Currency      string `json:"currency"`
}

type Service struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewService(pool *pgxpool.Pool, queryTimeout time.Duration) (*Service, error) {
	if pool == nil || queryTimeout <= 0 {
		return nil, errors.New("cart database pool and positive query timeout are required")
	}
	return &Service{pool: pool, queryTimeout: queryTimeout}, nil
}

func (s *Service) Get(ctx context.Context, userID, guestToken string) (Snapshot, string, error) {
	dbCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()
	cartID, nextGuestToken, err := s.resolve(dbCtx, userID, guestToken)
	if err != nil {
		return Snapshot{}, "", err
	}
	rows, err := s.pool.Query(dbCtx, `SELECT ci.id::text, v.id::text, p.id::text, p.slug, p.name, c.slug, c.name,
		p.image_key, p.currency, p.price_paise+v.price_adjustment_paise, ci.quantity
		FROM cart_items ci JOIN product_variants v ON v.id=ci.product_variant_id JOIN products p ON p.id=v.product_id
		JOIN categories c ON c.id=p.category_id WHERE ci.cart_id=$1 ORDER BY ci.created_at,ci.id`, cartID)
	if err != nil {
		return Snapshot{}, "", err
	}
	defer rows.Close()
	snapshot := Snapshot{Lines: make([]Line, 0), Currency: "INR"}
	for rows.Next() {
		var line Line
		if err := rows.Scan(&line.ID, &line.VariantID, &line.ProductID, &line.Slug, &line.Name, &line.Category, &line.CategoryName,
			&line.ImageKey, &line.Currency, &line.UnitPricePaise, &line.Quantity); err != nil {
			return Snapshot{}, "", err
		}
		line.LineTotalPaise = line.UnitPricePaise * int64(line.Quantity)
		snapshot.Count += line.Quantity
		snapshot.SubtotalPaise += line.LineTotalPaise
		snapshot.Lines = append(snapshot.Lines, line)
	}
	return snapshot, nextGuestToken, rows.Err()
}

func (s *Service) Add(ctx context.Context, userID, guestToken, variantID string, quantity int) (Snapshot, string, error) {
	if quantity < 1 || quantity > 20 {
		return Snapshot{}, "", ErrQuantity
	}
	dbCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()
	tx, err := s.pool.Begin(dbCtx)
	if err != nil {
		return Snapshot{}, "", err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	cartID, nextGuestToken, err := s.resolveTx(dbCtx, tx, userID, guestToken)
	if err != nil {
		return Snapshot{}, "", err
	}
	var policy string
	err = tx.QueryRow(dbCtx, `SELECT p.inventory_policy FROM product_variants v JOIN products p ON p.id=v.product_id
		WHERE v.id=$1 AND v.active AND p.status='active'`, variantID).Scan(&policy)
	if errors.Is(err, pgx.ErrNoRows) {
		return Snapshot{}, "", ErrProductUnavailable
	}
	if err != nil {
		return Snapshot{}, "", err
	}
	var itemID string
	var existing int
	err = tx.QueryRow(dbCtx, `SELECT id::text,quantity FROM cart_items WHERE cart_id=$1 AND product_variant_id=$2 AND customization='{}'::jsonb FOR UPDATE`, cartID, variantID).Scan(&itemID, &existing)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Snapshot{}, "", err
	}
	if existing+quantity > 20 {
		return Snapshot{}, "", ErrQuantity
	}
	if policy == "tracked" {
		var available int
		if err := tx.QueryRow(dbCtx, `SELECT on_hand-reserved FROM inventory WHERE product_variant_id=$1 FOR UPDATE`, variantID).Scan(&available); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Snapshot{}, "", ErrOutOfStock
			}
			return Snapshot{}, "", err
		}
		if existing+quantity > available {
			return Snapshot{}, "", ErrOutOfStock
		}
	}
	_, err = tx.Exec(dbCtx, `INSERT INTO cart_items(cart_id,product_variant_id,quantity,customization)
		VALUES($1,$2,$3,'{}'::jsonb) ON CONFLICT(cart_id,product_variant_id,customization)
		DO UPDATE SET quantity=cart_items.quantity+EXCLUDED.quantity,updated_at=now()`, cartID, variantID, quantity)
	if err != nil {
		return Snapshot{}, "", err
	}
	if err := tx.Commit(dbCtx); err != nil {
		return Snapshot{}, "", err
	}
	snapshot, cookie, err := s.Get(ctx, userID, firstNonEmpty(nextGuestToken, guestToken))
	return snapshot, cookie, err
}

func (s *Service) SetQuantity(ctx context.Context, userID, guestToken, itemID string, quantity int) (Snapshot, string, error) {
	if quantity < 0 || quantity > 20 {
		return Snapshot{}, "", ErrQuantity
	}
	dbCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()
	tx, err := s.pool.Begin(dbCtx)
	if err != nil {
		return Snapshot{}, "", err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	cartID, nextGuestToken, err := s.resolveTx(dbCtx, tx, userID, guestToken)
	if err != nil {
		return Snapshot{}, "", err
	}
	if quantity == 0 {
		result, err := tx.Exec(dbCtx, `DELETE FROM cart_items WHERE id=$1 AND cart_id=$2`, itemID, cartID)
		if err != nil {
			return Snapshot{}, "", err
		}
		if result.RowsAffected() != 1 {
			return Snapshot{}, "", ErrItemNotFound
		}
	} else {
		var variantID, policy string
		if err := tx.QueryRow(dbCtx, `SELECT ci.product_variant_id::text,p.inventory_policy FROM cart_items ci
			JOIN product_variants v ON v.id=ci.product_variant_id JOIN products p ON p.id=v.product_id
			WHERE ci.id=$1 AND ci.cart_id=$2 AND v.active AND p.status='active' FOR UPDATE OF ci`, itemID, cartID).Scan(&variantID, &policy); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Snapshot{}, "", ErrItemNotFound
			}
			return Snapshot{}, "", err
		}
		if policy == "tracked" {
			var available int
			if err := tx.QueryRow(dbCtx, `SELECT on_hand-reserved FROM inventory WHERE product_variant_id=$1 FOR UPDATE`, variantID).Scan(&available); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return Snapshot{}, "", ErrOutOfStock
				}
				return Snapshot{}, "", err
			}
			if quantity > available {
				return Snapshot{}, "", ErrOutOfStock
			}
		}
		if _, err := tx.Exec(dbCtx, `UPDATE cart_items SET quantity=$1,updated_at=now() WHERE id=$2 AND cart_id=$3`, quantity, itemID, cartID); err != nil {
			return Snapshot{}, "", err
		}
	}
	if err := tx.Commit(dbCtx); err != nil {
		return Snapshot{}, "", err
	}
	snapshot, cookie, err := s.Get(ctx, userID, firstNonEmpty(nextGuestToken, guestToken))
	return snapshot, cookie, err
}

func (s *Service) Remove(ctx context.Context, userID, guestToken, itemID string) (Snapshot, string, error) {
	return s.SetQuantity(ctx, userID, guestToken, itemID, 0)
}

func (s *Service) resolve(ctx context.Context, userID, guestToken string) (string, string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	id, next, err := s.resolveTx(ctx, tx, userID, guestToken)
	if err != nil {
		return "", "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", "", err
	}
	return id, next, nil
}

func (s *Service) resolveTx(ctx context.Context, tx pgx.Tx, userID, guestToken string) (string, string, error) {
	if userID != "" {
		var userCart string
		if err := tx.QueryRow(ctx, `INSERT INTO carts(user_id) VALUES($1) ON CONFLICT(user_id) WHERE user_id IS NOT NULL DO UPDATE SET updated_at=now() RETURNING id::text`, userID).Scan(&userCart); err != nil {
			return "", "", err
		}
		if guestToken != "" {
			hash := sha256.Sum256([]byte(guestToken))
			var guestCart string
			err := tx.QueryRow(ctx, `SELECT id::text FROM carts WHERE guest_token_hash=$1 AND (expires_at IS NULL OR expires_at>now()) FOR UPDATE`, hash[:]).Scan(&guestCart)
			if err == nil {
				if guestCart != userCart {
					_, err = tx.Exec(ctx, `INSERT INTO cart_items(cart_id,product_variant_id,quantity,customization)
			SELECT $1,product_variant_id,quantity,customization FROM cart_items WHERE cart_id=$2
			ON CONFLICT(cart_id,product_variant_id,customization) DO UPDATE SET quantity=LEAST(20,cart_items.quantity+EXCLUDED.quantity),updated_at=now()`, userCart, guestCart)
					if err != nil {
						return "", "", err
					}
					if _, err = tx.Exec(ctx, `DELETE FROM cart_items ci USING product_variants v JOIN products p ON p.id=v.product_id
						LEFT JOIN inventory i ON i.product_variant_id=v.id
						WHERE ci.cart_id=$1 AND ci.product_variant_id=v.id AND p.inventory_policy='tracked' AND COALESCE(i.on_hand-i.reserved,0)<=0`, userCart); err != nil {
						return "", "", err
					}
					if _, err = tx.Exec(ctx, `UPDATE cart_items ci SET quantity=LEAST(ci.quantity,i.on_hand-i.reserved),updated_at=now()
						FROM product_variants v JOIN products p ON p.id=v.product_id JOIN inventory i ON i.product_variant_id=v.id
						WHERE ci.cart_id=$1 AND ci.product_variant_id=v.id AND p.inventory_policy='tracked' AND i.on_hand-i.reserved>0`, userCart); err != nil {
						return "", "", err
					}
				}
				if _, err = tx.Exec(ctx, `DELETE FROM carts WHERE id=$1`, guestCart); err != nil {
					return "", "", err
				}
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return "", "", err
			}
		}
		return userCart, "", nil
	}
	if guestToken != "" {
		hash := sha256.Sum256([]byte(guestToken))
		var cartID string
		err := tx.QueryRow(ctx, `SELECT id::text FROM carts WHERE guest_token_hash=$1 AND (expires_at IS NULL OR expires_at>now()) FOR UPDATE`, hash[:]).Scan(&cartID)
		if err == nil {
			return cartID, "", nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return "", "", err
		}
	}
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", fmt.Errorf("generate guest cart token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(bytes)
	hash := sha256.Sum256([]byte(token))
	var cartID string
	if err := tx.QueryRow(ctx, `INSERT INTO carts(guest_token_hash,expires_at) VALUES($1,now()+interval '30 days') RETURNING id::text`, hash[:]).Scan(&cartID); err != nil {
		return "", "", err
	}
	return cartID, token, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
