package checkout

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrEmptyCart               = errors.New("cart is empty")
	ErrInvalidCheckout         = errors.New("checkout details are invalid")
	ErrInsufficientStock       = errors.New("requested quantity is no longer available")
	ErrIdempotencyConflict     = errors.New("idempotency key was already used for different checkout details")
	ErrInvalidReservationBatch = errors.New("reservation cleanup batch size is invalid")
)

const ReservationTTL = 20 * time.Minute

type Address struct {
	RecipientName string `json:"recipientName"`
	Phone         string `json:"phone"`
	Line1         string `json:"line1"`
	Line2         string `json:"line2"`
	City          string `json:"city"`
	Region        string `json:"region"`
	PostalCode    string `json:"postalCode"`
	CountryCode   string `json:"countryCode"`
}

type Details struct {
	Email        string  `json:"email"`
	Phone        string  `json:"phone"`
	Address      Address `json:"address"`
	CustomerNote string  `json:"customerNote"`
}

type Quote struct {
	Currency         string `json:"currency"`
	SubtotalPaise    int64  `json:"subtotalPaise"`
	TaxPaise         int64  `json:"taxPaise"`
	ShippingPaise    int64  `json:"shippingPaise"`
	DiscountPaise    int64  `json:"discountPaise"`
	TotalPaise       int64  `json:"totalPaise"`
	QuoteStatus      string `json:"quoteStatus"`
	PaymentAvailable bool   `json:"paymentAvailable"`
}

type Order struct {
	ID            string `json:"id"`
	OrderNumber   string `json:"orderNumber"`
	Status        string `json:"status"`
	Currency      string `json:"currency"`
	SubtotalPaise int64  `json:"subtotalPaise"`
	TaxPaise      int64  `json:"taxPaise"`
	ShippingPaise int64  `json:"shippingPaise"`
	TotalPaise    int64  `json:"totalPaise"`
	CreatedAt     string `json:"createdAt"`
}

type lineSnapshot struct {
	ProductID       string
	VariantID       string
	Name            string
	ProductSKU      string
	VariantSummary  string
	Quantity        int
	UnitPricePaise  int64
	InventoryPolicy string
}

type Service struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewService(pool *pgxpool.Pool, queryTimeout time.Duration) (*Service, error) {
	if pool == nil || queryTimeout <= 0 {
		return nil, errors.New("checkout database pool and positive query timeout are required")
	}
	return &Service{pool: pool, queryTimeout: queryTimeout}, nil
}

func (s *Service) Quote(ctx context.Context, cartID string) (Quote, error) {
	dbCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()
	var count int
	var subtotal int64
	err := s.pool.QueryRow(dbCtx, `SELECT COALESCE(sum(ci.quantity),0)::int,
		COALESCE(sum(ci.quantity*(p.price_paise+v.price_adjustment_paise)),0)::bigint
		FROM cart_items ci JOIN product_variants v ON v.id=ci.product_variant_id
		JOIN products p ON p.id=v.product_id WHERE ci.cart_id=$1 AND v.active AND p.status='active'`, cartID).Scan(&count, &subtotal)
	if err != nil {
		return Quote{}, err
	}
	if count == 0 {
		return Quote{}, ErrEmptyCart
	}
	if subtotal < 0 {
		return Quote{}, ErrInvalidCheckout
	}
	return Quote{Currency: "INR", SubtotalPaise: subtotal, TotalPaise: subtotal, QuoteStatus: "provisional", PaymentAvailable: false}, nil
}

func (s *Service) CreateOrder(ctx context.Context, cartID, userID, guestEmail, guestPhone, idempotencyKey string, details Details) (Order, bool, error) {
	if err := validateDetails(details, userID != ""); err != nil {
		return Order{}, false, err
	}
	if len(idempotencyKey) < 16 || len(idempotencyKey) > 128 {
		return Order{}, false, ErrInvalidCheckout
	}
	normalized, err := normalizeDetails(details, userID != "")
	if err != nil {
		return Order{}, false, err
	}
	body, err := json.Marshal(normalized)
	if err != nil {
		return Order{}, false, err
	}
	requestHash := sha256.Sum256(body)
	dbCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()
	tx, err := s.pool.Begin(dbCtx)
	if err != nil {
		return Order{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var lockedCart string
	if err := tx.QueryRow(dbCtx, `SELECT id::text FROM carts WHERE id=$1 FOR UPDATE`, cartID).Scan(&lockedCart); err != nil {
		return Order{}, false, err
	}
	scope := "guest"
	if userID != "" {
		scope = "user:" + userID
	} else {
		scope = "guest:" + cartID
	}
	_, err = tx.Exec(dbCtx, `INSERT INTO idempotency_records(scope,idempotency_key,request_sha256,expires_at)
		VALUES($1,$2,$3,now()+interval '24 hours') ON CONFLICT(scope,idempotency_key) DO NOTHING`, scope, idempotencyKey, requestHash[:])
	if err != nil {
		return Order{}, false, err
	}
	var storedHash []byte
	var responseBody []byte
	err = tx.QueryRow(dbCtx, `SELECT request_sha256,response_body FROM idempotency_records WHERE scope=$1 AND idempotency_key=$2 FOR UPDATE`, scope, idempotencyKey).Scan(&storedHash, &responseBody)
	if err != nil {
		return Order{}, false, err
	}
	if len(storedHash) != len(requestHash) || !equalBytes(storedHash, requestHash[:]) {
		return Order{}, false, ErrIdempotencyConflict
	}
	if len(responseBody) > 0 {
		var replay Order
		if err := json.Unmarshal(responseBody, &replay); err != nil {
			return Order{}, false, err
		}
		if err := tx.Commit(dbCtx); err != nil {
			return Order{}, false, err
		}
		return replay, true, nil
	}
	rows, err := tx.Query(dbCtx, `SELECT p.id::text,v.id::text,p.name,p.sku,v.option_name||': '||v.option_value,
		ci.quantity,p.price_paise+v.price_adjustment_paise,p.inventory_policy
		FROM cart_items ci JOIN product_variants v ON v.id=ci.product_variant_id JOIN products p ON p.id=v.product_id
		WHERE ci.cart_id=$1 AND v.active AND p.status='active' ORDER BY ci.created_at,ci.id FOR UPDATE OF ci,v,p`, cartID)
	if err != nil {
		return Order{}, false, err
	}
	lines := make([]lineSnapshot, 0)
	var subtotal int64
	for rows.Next() {
		var line lineSnapshot
		if err := rows.Scan(&line.ProductID, &line.VariantID, &line.Name, &line.ProductSKU, &line.VariantSummary, &line.Quantity, &line.UnitPricePaise, &line.InventoryPolicy); err != nil {
			rows.Close()
			return Order{}, false, err
		}
		if line.UnitPricePaise < 0 {
			rows.Close()
			return Order{}, false, ErrInvalidCheckout
		}
		if line.InventoryPolicy == "tracked" {
			var available int
			if err := tx.QueryRow(dbCtx, `SELECT on_hand-reserved FROM inventory WHERE product_variant_id=$1 FOR UPDATE`, line.VariantID).Scan(&available); err != nil {
				rows.Close()
				if errors.Is(err, pgx.ErrNoRows) {
					return Order{}, false, ErrInsufficientStock
				}
				return Order{}, false, err
			}
			if line.Quantity > available {
				rows.Close()
				return Order{}, false, ErrInsufficientStock
			}
		}
		subtotal += int64(line.Quantity) * line.UnitPricePaise
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Order{}, false, err
	}
	rows.Close()
	if len(lines) == 0 {
		return Order{}, false, ErrEmptyCart
	}
	if userID == "" {
		guestEmail = normalized.Email
		guestPhone = normalized.Phone
	} else {
		normalized.Email = ""
		guestEmail = ""
		guestPhone = ""
	}
	addressBytes, err := json.Marshal(normalized.Address)
	if err != nil {
		return Order{}, false, err
	}
	orderNumber, err := newOrderNumber()
	if err != nil {
		return Order{}, false, err
	}
	idempotencyScope := scope + ":" + idempotencyKey
	var order Order
	err = tx.QueryRow(dbCtx, `INSERT INTO orders(order_number,customer_id,guest_email,guest_phone,status,currency,subtotal_paise,tax_paise,shipping_paise,discount_paise,total_paise,shipping_address,customer_note,idempotency_key)
		VALUES($1,NULLIF($2,'')::uuid,$3,$4,'pending_payment','INR',$5,0,0,0,$5,$6::jsonb,$7,$8)
		RETURNING id::text,order_number,status,currency,subtotal_paise,tax_paise,shipping_paise,total_paise,created_at::text`, orderNumber, userID, guestEmail, guestPhone, subtotal, string(addressBytes), normalized.CustomerNote, idempotencyScope).Scan(&order.ID, &order.OrderNumber, &order.Status, &order.Currency, &order.SubtotalPaise, &order.TaxPaise, &order.ShippingPaise, &order.TotalPaise, &order.CreatedAt)
	if err != nil {
		return Order{}, false, err
	}
	for _, line := range lines {
		lineTotal := line.UnitPricePaise * int64(line.Quantity)
		var orderItemID string
		err = tx.QueryRow(dbCtx, `INSERT INTO order_items(order_id,product_id,product_variant_id,product_name,product_sku,variant_description,quantity,unit_price_paise,tax_rate_basis_points,tax_paise,line_total_paise)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,0,0,$9) RETURNING id::text`, order.ID, line.ProductID, line.VariantID, line.Name, line.ProductSKU, line.VariantSummary, line.Quantity, line.UnitPricePaise, lineTotal).Scan(&orderItemID)
		if err != nil {
			return Order{}, false, err
		}
		if line.InventoryPolicy == "tracked" {
			var onHand int
			if err := tx.QueryRow(dbCtx, `UPDATE inventory SET reserved=reserved+$2,updated_at=now()
				WHERE product_variant_id=$1 AND on_hand-reserved >= $2 RETURNING on_hand`, line.VariantID, line.Quantity).Scan(&onHand); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return Order{}, false, ErrInsufficientStock
				}
				return Order{}, false, err
			}
			if _, err := tx.Exec(dbCtx, `INSERT INTO inventory_reservations(order_item_id,product_variant_id,quantity,status,expires_at)
				VALUES($1,$2,$3,'active',now()+($4 * interval '1 second'))`, orderItemID, line.VariantID, line.Quantity, int64(ReservationTTL.Seconds())); err != nil {
				return Order{}, false, err
			}
			if _, err := tx.Exec(dbCtx, `INSERT INTO inventory_movements(product_variant_id,movement_type,quantity_delta,quantity_after,reference_type,reference_id,note)
				VALUES($1,'reservation',$2,$3,'order',$4,'Checkout stock reservation')`, line.VariantID, line.Quantity, onHand, order.ID); err != nil {
				return Order{}, false, err
			}
		}
	}
	if _, err = tx.Exec(dbCtx, `INSERT INTO order_status_events(order_id,from_status,to_status,reason) VALUES($1,NULL,'pending_payment','Customer submitted checkout details')`, order.ID); err != nil {
		return Order{}, false, err
	}
	if _, err = tx.Exec(dbCtx, `DELETE FROM cart_items WHERE cart_id=$1`, cartID); err != nil {
		return Order{}, false, err
	}
	response, err := json.Marshal(order)
	if err != nil {
		return Order{}, false, err
	}
	if _, err = tx.Exec(dbCtx, `UPDATE idempotency_records SET response_status=201,response_body=$3::jsonb,completed_at=now() WHERE scope=$1 AND idempotency_key=$2`, scope, idempotencyKey, string(response)); err != nil {
		return Order{}, false, err
	}
	if err := tx.Commit(dbCtx); err != nil {
		return Order{}, false, err
	}
	return order, false, nil
}

func validateDetails(details Details, authenticated bool) error {
	if len(details.Address.RecipientName) < 1 || len(details.Address.RecipientName) > 120 || len(details.Phone) < 7 || len(details.Phone) > 32 || len(details.Address.Phone) < 7 || len(details.Address.Phone) > 32 || len(details.Address.Line1) < 2 || len(details.Address.Line1) > 200 || len(details.Address.Line2) > 200 || len(details.Address.City) < 1 || len(details.Address.City) > 120 || len(details.Address.Region) < 1 || len(details.Address.Region) > 120 || len(details.Address.PostalCode) < 3 || len(details.Address.PostalCode) > 24 || len(details.Address.CountryCode) != 2 || len(details.CustomerNote) > 1000 {
		return ErrInvalidCheckout
	}
	for _, char := range details.Address.CountryCode {
		if char < 'A' || (char > 'Z' && char < 'a') || char > 'z' {
			return ErrInvalidCheckout
		}
	}
	if !authenticated {
		email, err := mail.ParseAddress(strings.TrimSpace(details.Email))
		if err != nil || email.Address != strings.TrimSpace(details.Email) || len(details.Email) > 254 {
			return ErrInvalidCheckout
		}
	}
	return nil
}

func normalizeDetails(details Details, authenticated bool) (Details, error) {
	details.Email = strings.ToLower(strings.TrimSpace(details.Email))
	details.Phone = strings.TrimSpace(details.Phone)
	details.Address.RecipientName = strings.TrimSpace(details.Address.RecipientName)
	details.Address.Phone = strings.TrimSpace(details.Address.Phone)
	details.Address.Line1 = strings.TrimSpace(details.Address.Line1)
	details.Address.Line2 = strings.TrimSpace(details.Address.Line2)
	details.Address.City = strings.TrimSpace(details.Address.City)
	details.Address.Region = strings.TrimSpace(details.Address.Region)
	details.Address.PostalCode = strings.TrimSpace(details.Address.PostalCode)
	details.Address.CountryCode = strings.ToUpper(strings.TrimSpace(details.Address.CountryCode))
	details.CustomerNote = strings.TrimSpace(details.CustomerNote)
	if err := validateDetails(details, authenticated); err != nil {
		return Details{}, err
	}
	return details, nil
}

func newOrderNumber() (string, error) {
	b := make([]byte, 7)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate order number: %w", err)
	}
	return "AR-" + time.Now().UTC().Format("060102") + "-" + strings.ToUpper(base64.RawURLEncoding.EncodeToString(b)), nil
}
func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
