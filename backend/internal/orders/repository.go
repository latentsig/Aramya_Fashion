package orders

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("order not found")

type Summary struct {
	ID            string    `json:"id"`
	OrderNumber   string    `json:"orderNumber"`
	Status        string    `json:"status"`
	Currency      string    `json:"currency"`
	SubtotalPaise int64     `json:"subtotalPaise"`
	TaxPaise      int64     `json:"taxPaise"`
	ShippingPaise int64     `json:"shippingPaise"`
	DiscountPaise int64     `json:"discountPaise"`
	TotalPaise    int64     `json:"totalPaise"`
	CreatedAt     time.Time `json:"createdAt"`
}

type Item struct {
	ID                 string `json:"id"`
	ProductID          string `json:"productId"`
	VariantID          string `json:"variantId"`
	ProductName        string `json:"productName"`
	ProductSKU         string `json:"productSku"`
	VariantDescription string `json:"variantDescription"`
	Quantity           int    `json:"quantity"`
	UnitPricePaise     int64  `json:"unitPricePaise"`
	TaxPaise           int64  `json:"taxPaise"`
	LineTotalPaise     int64  `json:"lineTotalPaise"`
}

type Detail struct {
	Summary
	ShippingAddress map[string]any `json:"shippingAddress"`
	CustomerNote    string         `json:"customerNote"`
	Items           []Item         `json:"items"`
}

type Repository struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

func NewRepository(pool *pgxpool.Pool, timeout time.Duration) (*Repository, error) {
	if pool == nil || timeout <= 0 {
		return nil, errors.New("orders database pool and positive query timeout are required")
	}
	return &Repository{pool: pool, timeout: timeout}, nil
}

func (r *Repository) List(ctx context.Context, userID string, limit, offset int) ([]Summary, error) {
	if limit < 1 || limit > 51 || offset < 0 || offset > 100000 {
		return nil, errors.New("order pagination is out of range")
	}
	dbCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	rows, err := r.pool.Query(dbCtx, `SELECT id::text,order_number,status,currency,subtotal_paise,tax_paise,shipping_paise,discount_paise,total_paise,created_at
		FROM orders WHERE customer_id=$1 ORDER BY created_at DESC,id DESC LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Summary, 0)
	for rows.Next() {
		var item Summary
		if err := rows.Scan(&item.ID, &item.OrderNumber, &item.Status, &item.Currency, &item.SubtotalPaise, &item.TaxPaise, &item.ShippingPaise, &item.DiscountPaise, &item.TotalPaise, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) Get(ctx context.Context, userID, orderID string) (Detail, error) {
	dbCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var detail Detail
	var address []byte
	err := r.pool.QueryRow(dbCtx, `SELECT id::text,order_number,status,currency,subtotal_paise,tax_paise,shipping_paise,discount_paise,total_paise,created_at,shipping_address,customer_note
		FROM orders WHERE customer_id=$1 AND id=$2`, userID, orderID).Scan(&detail.ID, &detail.OrderNumber, &detail.Status, &detail.Currency, &detail.SubtotalPaise, &detail.TaxPaise, &detail.ShippingPaise, &detail.DiscountPaise, &detail.TotalPaise, &detail.CreatedAt, &address, &detail.CustomerNote)
	if errors.Is(err, pgx.ErrNoRows) {
		return Detail{}, ErrNotFound
	}
	if err != nil {
		return Detail{}, err
	}
	if err := json.Unmarshal(address, &detail.ShippingAddress); err != nil {
		return Detail{}, err
	}

	rows, err := r.pool.Query(dbCtx, `SELECT id::text,product_id::text,product_variant_id::text,product_name,product_sku,variant_description,quantity,unit_price_paise,tax_paise,line_total_paise
		FROM order_items WHERE order_id=$1 ORDER BY created_at,id`, orderID)
	if err != nil {
		return Detail{}, err
	}
	defer rows.Close()
	detail.Items = make([]Item, 0)
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.ID, &item.ProductID, &item.VariantID, &item.ProductName, &item.ProductSKU, &item.VariantDescription, &item.Quantity, &item.UnitPricePaise, &item.TaxPaise, &item.LineTotalPaise); err != nil {
			return Detail{}, err
		}
		detail.Items = append(detail.Items, item)
	}
	return detail, rows.Err()
}
