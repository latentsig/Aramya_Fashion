package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrOrderNotFound = errors.New("order not found")
	ErrOrderStale    = errors.New("order changed since it was loaded")
	ErrOrderState    = errors.New("order transition is not allowed")
	ErrOrderPayment  = errors.New("order payment state does not permit this transition")
	adminCarrierName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ._/-]{0,79}$`)
)

type OrderFilter struct {
	Status string
	Search string
	Limit  int
	Offset int
}

type AdminOrder struct {
	ID               string    `json:"id"`
	Version          int       `json:"version"`
	OrderNumber      string    `json:"orderNumber"`
	Status           string    `json:"status"`
	Currency         string    `json:"currency"`
	SubtotalPaise    int64     `json:"subtotalPaise"`
	TaxPaise         int64     `json:"taxPaise"`
	ShippingPaise    int64     `json:"shippingPaise"`
	DiscountPaise    int64     `json:"discountPaise"`
	TotalPaise       int64     `json:"totalPaise"`
	CustomerName     string    `json:"customerName"`
	CustomerEmail    string    `json:"customerEmail"`
	CustomerPhone    string    `json:"customerPhone"`
	PaymentStatus    string    `json:"paymentStatus"`
	RefundStatus     string    `json:"refundStatus"`
	RefundedPaise    int64     `json:"refundedPaise"`
	ShipmentStatus   string    `json:"shipmentStatus"`
	ShipmentProvider string    `json:"shipmentProvider"`
	TrackingNumber   string    `json:"trackingNumber"`
	TrackingURL      string    `json:"trackingUrl"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type AdminOrderItem struct {
	ID                 string `json:"id"`
	ProductName        string `json:"productName"`
	ProductSKU         string `json:"productSku"`
	VariantDescription string `json:"variantDescription"`
	Quantity           int    `json:"quantity"`
	UnitPricePaise     int64  `json:"unitPricePaise"`
	TaxPaise           int64  `json:"taxPaise"`
	LineTotalPaise     int64  `json:"lineTotalPaise"`
}

type AdminOrderEvent struct {
	FromStatus string    `json:"fromStatus"`
	ToStatus   string    `json:"toStatus"`
	Reason     string    `json:"reason"`
	ActorName  string    `json:"actorName"`
	CreatedAt  time.Time `json:"createdAt"`
}

type AdminOrderDetail struct {
	AdminOrder
	ShippingAddress map[string]any    `json:"shippingAddress"`
	CustomerNote    string            `json:"customerNote"`
	Items           []AdminOrderItem  `json:"items"`
	Events          []AdminOrderEvent `json:"events"`
}

type OrderUpdate struct {
	ExpectedVersion  int    `json:"expectedVersion"`
	Status           string `json:"status"`
	Reason           string `json:"reason"`
	ShipmentProvider string `json:"shipmentProvider,omitempty"`
	TrackingNumber   string `json:"trackingNumber,omitempty"`
	TrackingURL      string `json:"trackingUrl,omitempty"`
}

type OrderRepository struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

func NewOrderRepository(pool *pgxpool.Pool, timeout time.Duration) (*OrderRepository, error) {
	if pool == nil || timeout <= 0 {
		return nil, errors.New("admin order database pool and positive query timeout are required")
	}
	return &OrderRepository{pool: pool, timeout: timeout}, nil
}

func ValidOrderStatus(status string) bool {
	switch status {
	case "pending_payment", "paid", "confirmed", "processing", "ready_to_ship", "shipped", "delivered", "cancelled", "refund_pending", "refunded":
		return true
	default:
		return false
	}
}

func ValidAdminOrderTransition(from, to string) bool {
	switch from {
	case "pending_payment":
		return to == "cancelled"
	case "paid":
		return to == "confirmed"
	case "confirmed":
		return to == "processing"
	case "processing":
		return to == "ready_to_ship"
	case "ready_to_ship":
		return to == "shipped"
	case "shipped":
		return to == "delivered"
	default:
		return false
	}
}

func ValidateOrderUpdate(input OrderUpdate) error {
	input.Status = strings.TrimSpace(input.Status)
	input.Reason = strings.TrimSpace(input.Reason)
	input.ShipmentProvider = strings.TrimSpace(input.ShipmentProvider)
	input.TrackingNumber = strings.TrimSpace(input.TrackingNumber)
	input.TrackingURL = strings.TrimSpace(input.TrackingURL)
	if input.ExpectedVersion < 1 || !ValidOrderStatus(input.Status) || len(input.Reason) < 3 || len(input.Reason) > 500 || !utf8.ValidString(input.Reason) ||
		len(input.ShipmentProvider) > 80 || len(input.TrackingNumber) > 100 || len(input.TrackingURL) > 500 ||
		!utf8.ValidString(input.ShipmentProvider) || !utf8.ValidString(input.TrackingNumber) || !utf8.ValidString(input.TrackingURL) {
		return ErrInvalidCatalog
	}
	if input.ShipmentProvider != "" && !adminCarrierName.MatchString(input.ShipmentProvider) {
		return ErrInvalidCatalog
	}
	if input.Status == "shipped" {
		if input.ShipmentProvider == "" || input.TrackingNumber == "" {
			return ErrInvalidCatalog
		}
	}
	if input.TrackingURL != "" {
		parsed, err := url.ParseRequestURI(input.TrackingURL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
			return ErrInvalidCatalog
		}
	}
	if input.Status != "shipped" && (input.ShipmentProvider != "" || input.TrackingNumber != "" || input.TrackingURL != "") {
		return ErrInvalidCatalog
	}
	return nil
}

func (r *OrderRepository) List(ctx context.Context, filter OrderFilter) ([]AdminOrder, error) {
	if filter.Limit < 1 || filter.Limit > 51 || filter.Offset < 0 || filter.Offset > 100000 {
		return nil, errors.New("admin order pagination is out of range")
	}
	if filter.Status != "" && !ValidOrderStatus(filter.Status) || len(filter.Search) > 100 || !utf8.ValidString(filter.Search) {
		return nil, ErrInvalidCatalog
	}
	dbCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	rows, err := r.pool.Query(dbCtx, `
		SELECT o.id::text,o.version,o.order_number,o.status,o.currency,o.subtotal_paise,o.tax_paise,o.shipping_paise,o.discount_paise,o.total_paise,
		       coalesce(nullif(u.display_name,''),nullif(o.shipping_address->>'recipientName',''),''),coalesce(nullif(o.guest_email,''),u.email,''),coalesce(nullif(o.guest_phone,''),nullif(o.shipping_address->>'phone',''),u.phone,''),
	       coalesce((SELECT pa.status FROM payment_attempts pa WHERE pa.order_id=o.id ORDER BY pa.created_at DESC,pa.id DESC LIMIT 1),''),
	       coalesce((SELECT rf.status FROM refunds rf JOIN payment_attempts pa ON pa.id=rf.payment_attempt_id WHERE pa.order_id=o.id ORDER BY rf.created_at DESC,rf.id DESC LIMIT 1),''),
	       coalesce((SELECT sum(rf.amount_paise) FROM refunds rf JOIN payment_attempts pa ON pa.id=rf.payment_attempt_id WHERE pa.order_id=o.id AND rf.status='processed'),0)::bigint,
		       coalesce(s.status,'unfulfilled'),coalesce(s.provider,''),coalesce(s.tracking_number,''),coalesce(s.tracking_url,''),o.created_at,o.updated_at
		FROM orders o LEFT JOIN users u ON u.id=o.customer_id LEFT JOIN shipments s ON s.order_id=o.id
		WHERE ($1::text='' OR o.status=$1)
		  AND ($2::text='' OR o.order_number ILIKE '%'||$2||'%' OR coalesce(nullif(o.guest_email,''),u.email,'') ILIKE '%'||$2||'%' OR coalesce(nullif(o.guest_phone,''),'') ILIKE '%'||$2||'%' OR coalesce(u.display_name,'') ILIKE '%'||$2||'%')
		ORDER BY o.created_at DESC,o.id DESC LIMIT $3 OFFSET $4`, filter.Status, strings.TrimSpace(filter.Search), filter.Limit, filter.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]AdminOrder, 0)
	for rows.Next() {
		var item AdminOrder
		if err := rows.Scan(&item.ID, &item.Version, &item.OrderNumber, &item.Status, &item.Currency, &item.SubtotalPaise, &item.TaxPaise, &item.ShippingPaise, &item.DiscountPaise, &item.TotalPaise, &item.CustomerName, &item.CustomerEmail, &item.CustomerPhone, &item.PaymentStatus, &item.RefundStatus, &item.RefundedPaise, &item.ShipmentStatus, &item.ShipmentProvider, &item.TrackingNumber, &item.TrackingURL, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *OrderRepository) Get(ctx context.Context, id string) (AdminOrderDetail, error) {
	dbCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	detail := AdminOrderDetail{}
	var address []byte
	err := r.pool.QueryRow(dbCtx, `
		SELECT o.id::text,o.version,o.order_number,o.status,o.currency,o.subtotal_paise,o.tax_paise,o.shipping_paise,o.discount_paise,o.total_paise,
		       coalesce(nullif(u.display_name,''),nullif(o.shipping_address->>'recipientName',''),''),coalesce(nullif(o.guest_email,''),u.email,''),coalesce(nullif(o.guest_phone,''),nullif(o.shipping_address->>'phone',''),u.phone,''),
	       coalesce((SELECT pa.status FROM payment_attempts pa WHERE pa.order_id=o.id ORDER BY pa.created_at DESC,pa.id DESC LIMIT 1),''),
	       coalesce((SELECT rf.status FROM refunds rf JOIN payment_attempts pa ON pa.id=rf.payment_attempt_id WHERE pa.order_id=o.id ORDER BY rf.created_at DESC,rf.id DESC LIMIT 1),''),
	       coalesce((SELECT sum(rf.amount_paise) FROM refunds rf JOIN payment_attempts pa ON pa.id=rf.payment_attempt_id WHERE pa.order_id=o.id AND rf.status='processed'),0)::bigint,
		       coalesce(s.status,'unfulfilled'),coalesce(s.provider,''),coalesce(s.tracking_number,''),coalesce(s.tracking_url,''),o.created_at,o.updated_at,o.shipping_address,o.customer_note
		FROM orders o LEFT JOIN users u ON u.id=o.customer_id LEFT JOIN shipments s ON s.order_id=o.id WHERE o.id=$1`, id).
		Scan(&detail.ID, &detail.Version, &detail.OrderNumber, &detail.Status, &detail.Currency, &detail.SubtotalPaise, &detail.TaxPaise, &detail.ShippingPaise, &detail.DiscountPaise, &detail.TotalPaise, &detail.CustomerName, &detail.CustomerEmail, &detail.CustomerPhone, &detail.PaymentStatus, &detail.RefundStatus, &detail.RefundedPaise, &detail.ShipmentStatus, &detail.ShipmentProvider, &detail.TrackingNumber, &detail.TrackingURL, &detail.CreatedAt, &detail.UpdatedAt, &address, &detail.CustomerNote)
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminOrderDetail{}, ErrOrderNotFound
	}
	if err != nil {
		return AdminOrderDetail{}, err
	}
	if err := json.Unmarshal(address, &detail.ShippingAddress); err != nil {
		return AdminOrderDetail{}, err
	}
	detail.Items = make([]AdminOrderItem, 0)
	rows, err := r.pool.Query(dbCtx, `SELECT id::text,product_name,product_sku,variant_description,quantity,unit_price_paise,tax_paise,line_total_paise FROM order_items WHERE order_id=$1 ORDER BY created_at,id`, id)
	if err != nil {
		return AdminOrderDetail{}, err
	}
	for rows.Next() {
		var item AdminOrderItem
		if err := rows.Scan(&item.ID, &item.ProductName, &item.ProductSKU, &item.VariantDescription, &item.Quantity, &item.UnitPricePaise, &item.TaxPaise, &item.LineTotalPaise); err != nil {
			rows.Close()
			return AdminOrderDetail{}, err
		}
		detail.Items = append(detail.Items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return AdminOrderDetail{}, err
	}
	rows.Close()
	detail.Events = make([]AdminOrderEvent, 0)
	events, err := r.pool.Query(dbCtx, `SELECT coalesce(e.from_status,''),e.to_status,e.reason,coalesce(u.display_name,''),e.created_at FROM order_status_events e LEFT JOIN users u ON u.id=e.actor_user_id WHERE e.order_id=$1 ORDER BY e.created_at,e.id`, id)
	if err != nil {
		return AdminOrderDetail{}, err
	}
	defer events.Close()
	for events.Next() {
		var event AdminOrderEvent
		if err := events.Scan(&event.FromStatus, &event.ToStatus, &event.Reason, &event.ActorName, &event.CreatedAt); err != nil {
			return AdminOrderDetail{}, err
		}
		detail.Events = append(detail.Events, event)
	}
	return detail, events.Err()
}

func (r *OrderRepository) Update(ctx context.Context, id string, input OrderUpdate, actorID, requestID string) (AdminOrderDetail, error) {
	if ValidateOrderUpdate(input) != nil {
		return AdminOrderDetail{}, ErrInvalidCatalog
	}
	input.Status, input.Reason = strings.TrimSpace(input.Status), strings.TrimSpace(input.Reason)
	input.ShipmentProvider, input.TrackingNumber, input.TrackingURL = strings.TrimSpace(input.ShipmentProvider), strings.TrimSpace(input.TrackingNumber), strings.TrimSpace(input.TrackingURL)
	dbCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	tx, err := r.pool.Begin(dbCtx)
	if err != nil {
		return AdminOrderDetail{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var current string
	var version int
	err = tx.QueryRow(dbCtx, `SELECT status,version FROM orders WHERE id=$1 FOR UPDATE`, id).Scan(&current, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminOrderDetail{}, ErrOrderNotFound
	}
	if err != nil {
		return AdminOrderDetail{}, err
	}
	if version != input.ExpectedVersion {
		return AdminOrderDetail{}, ErrOrderStale
	}
	if !ValidAdminOrderTransition(current, input.Status) {
		return AdminOrderDetail{}, ErrOrderState
	}
	if current == "pending_payment" {
		var paymentExists bool
		if err := tx.QueryRow(dbCtx, `SELECT EXISTS(SELECT 1 FROM payment_attempts WHERE order_id=$1 AND status NOT IN ('failed','cancelled'))`, id).Scan(&paymentExists); err != nil {
			return AdminOrderDetail{}, err
		}
		if paymentExists {
			return AdminOrderDetail{}, ErrOrderPayment
		}
	}
	if input.Status == "cancelled" {
		if _, err := tx.Exec(dbCtx, `UPDATE inventory_reservations SET status='released',resolved_at=now() WHERE status='active' AND order_item_id IN (SELECT id FROM order_items WHERE order_id=$1)`, id); err != nil {
			return AdminOrderDetail{}, err
		}
	}
	if input.Status == "shipped" {
		_, err = tx.Exec(dbCtx, `INSERT INTO shipments(order_id,provider,tracking_number,tracking_url,status,shipped_at,updated_at)
			VALUES($1,$2,$3,$4,'shipped',now(),now())
			ON CONFLICT(order_id) DO UPDATE SET provider=EXCLUDED.provider,tracking_number=EXCLUDED.tracking_number,tracking_url=EXCLUDED.tracking_url,status='shipped',shipped_at=now(),updated_at=now()`, id, input.ShipmentProvider, input.TrackingNumber, input.TrackingURL)
		if err != nil {
			return AdminOrderDetail{}, err
		}
	} else if input.Status == "delivered" {
		if _, err := tx.Exec(dbCtx, `UPDATE shipments SET status='delivered',delivered_at=now(),updated_at=now() WHERE order_id=$1 AND status='shipped'`, id); err != nil {
			return AdminOrderDetail{}, err
		}
	} else if input.Status == "ready_to_ship" {
		if _, err := tx.Exec(dbCtx, `INSERT INTO shipments(order_id,status) VALUES($1,'ready') ON CONFLICT(order_id) DO UPDATE SET status='ready',updated_at=now()`, id); err != nil {
			return AdminOrderDetail{}, err
		}
	}
	if _, err := tx.Exec(dbCtx, `UPDATE orders SET status=$2,version=version+1,updated_at=now() WHERE id=$1`, id, input.Status); err != nil {
		return AdminOrderDetail{}, err
	}
	if _, err := tx.Exec(dbCtx, `INSERT INTO order_status_events(order_id,from_status,to_status,actor_user_id,reason) VALUES($1,$2,$3,$4,$5)`, id, current, input.Status, actorID, input.Reason); err != nil {
		return AdminOrderDetail{}, err
	}
	metadata, err := json.Marshal(map[string]any{"fromStatus": current, "toStatus": input.Status, "version": version + 1})
	if err != nil {
		return AdminOrderDetail{}, err
	}
	if _, err := tx.Exec(dbCtx, `INSERT INTO audit_logs(actor_user_id,action,entity_type,entity_id,request_id,metadata) VALUES($1,'order.status.update','order',$2,$3,$4)`, actorID, id, requestID, metadata); err != nil {
		return AdminOrderDetail{}, err
	}
	if err := tx.Commit(dbCtx); err != nil {
		return AdminOrderDetail{}, err
	}
	return r.Get(ctx, id)
}
