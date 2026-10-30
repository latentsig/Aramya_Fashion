package admin

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OverviewOrder struct {
	OrderNumber   string    `json:"orderNumber"`
	Status        string    `json:"status"`
	PaymentStatus string    `json:"paymentStatus"`
	Currency      string    `json:"currency"`
	TotalPaise    int64     `json:"totalPaise"`
	CreatedAt     time.Time `json:"createdAt"`
}

type Overview struct {
	ActiveProducts        int             `json:"activeProducts"`
	TotalOrders           int             `json:"totalOrders"`
	PendingPaymentOrders  int             `json:"pendingPaymentOrders"`
	GrossCapturedPaise    int64           `json:"grossCapturedPaise"`
	ProcessedRefundsPaise int64           `json:"processedRefundsPaise"`
	NetSalesPaise         int64           `json:"netSalesPaise"`
	CapturedPayments      int             `json:"capturedPayments"`
	RequestedAppointments int             `json:"requestedAppointments"`
	OpenCustomOrders      int             `json:"openCustomOrders"`
	RecentOrders          []OverviewOrder `json:"recentOrders"`
}

type OverviewRepository struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

func NewOverviewRepository(pool *pgxpool.Pool, timeout time.Duration) (*OverviewRepository, error) {
	if pool == nil || timeout <= 0 {
		return nil, errors.New("admin overview database pool and positive query timeout are required")
	}
	return &OverviewRepository{pool: pool, timeout: timeout}, nil
}

func (r *OverviewRepository) Get(ctx context.Context) (Overview, error) {
	dbCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	tx, err := r.pool.BeginTx(dbCtx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Overview{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	result := Overview{RecentOrders: make([]OverviewOrder, 0, 8)}
	err = tx.QueryRow(dbCtx, `SELECT
		(SELECT count(*)::int FROM products p JOIN categories c ON c.id=p.category_id WHERE p.status='active' AND c.active),
		(SELECT count(*)::int FROM orders),
		(SELECT count(*)::int FROM orders WHERE status='pending_payment'),
		(SELECT COALESCE(sum(amount_paise),0)::bigint FROM payment_attempts WHERE status IN ('captured','partially_refunded','refunded')),
		(SELECT count(*)::int FROM payment_attempts WHERE status IN ('captured','partially_refunded','refunded')),
		(SELECT COALESCE(sum(amount_paise),0)::bigint FROM refunds WHERE status='processed'),
		(SELECT count(*)::int FROM appointments WHERE status='requested'),
		(SELECT count(*)::int FROM custom_order_requests WHERE status IN ('requested','reviewing','fitting','quoted','awaiting_approval'))`).Scan(
		&result.ActiveProducts, &result.TotalOrders, &result.PendingPaymentOrders, &result.GrossCapturedPaise,
		&result.CapturedPayments, &result.ProcessedRefundsPaise, &result.RequestedAppointments, &result.OpenCustomOrders)
	if err != nil {
		return Overview{}, err
	}
	result.NetSalesPaise = result.GrossCapturedPaise - result.ProcessedRefundsPaise
	rows, err := tx.Query(dbCtx, `SELECT o.order_number,o.status,
		COALESCE((SELECT pa.status FROM payment_attempts pa WHERE pa.order_id=o.id ORDER BY pa.created_at DESC,pa.id DESC LIMIT 1),''),
		o.currency,o.total_paise,o.created_at
		FROM orders o ORDER BY o.created_at DESC,o.id DESC LIMIT 8`)
	if err != nil {
		return Overview{}, err
	}
	for rows.Next() {
		var item OverviewOrder
		if err := rows.Scan(&item.OrderNumber, &item.Status, &item.PaymentStatus, &item.Currency, &item.TotalPaise, &item.CreatedAt); err != nil {
			rows.Close()
			return Overview{}, err
		}
		result.RecentOrders = append(result.RecentOrders, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Overview{}, err
	}
	rows.Close()
	if err := tx.Commit(dbCtx); err != nil {
		return Overview{}, err
	}
	return result, nil
}
