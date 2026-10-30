package checkout

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

// ReleaseExpiredReservations cancels only pending orders whose tracked-stock
// reservations have expired and which have no unresolved payment attempt. An
// initiated, pending, authorized, or captured attempt keeps stock reserved until
// payment reconciliation resolves it.
func (s *Service) ReleaseExpiredReservations(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 500 {
		return 0, ErrInvalidReservationBatch
	}
	dbCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()
	tx, err := s.pool.Begin(dbCtx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	rows, err := tx.Query(dbCtx, `SELECT o.id::text FROM orders o
		WHERE o.status='pending_payment'
		  AND EXISTS (
		    SELECT 1 FROM order_items oi JOIN inventory_reservations ir ON ir.order_item_id=oi.id
		    WHERE oi.order_id=o.id AND ir.status='active' AND ir.expires_at<=now()
		  )
		  AND NOT EXISTS (
		    SELECT 1 FROM payment_attempts pa WHERE pa.order_id=o.id AND pa.status NOT IN ('failed','cancelled')
		  )
		ORDER BY o.created_at,o.id LIMIT $1 FOR UPDATE OF o SKIP LOCKED`, limit)
	if err != nil {
		return 0, err
	}
	orderIDs := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		orderIDs = append(orderIDs, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	expiredOrders := 0
	for _, orderID := range orderIDs {
		expired, err := s.expireOrderReservations(dbCtx, tx, orderID)
		if err != nil {
			return 0, err
		}
		if expired {
			expiredOrders++
		}
	}
	if err := tx.Commit(dbCtx); err != nil {
		return 0, err
	}
	return expiredOrders, nil
}

func (s *Service) expireOrderReservations(ctx context.Context, tx pgx.Tx, orderID string) (bool, error) {
	rows, err := tx.Query(ctx, `SELECT ir.order_item_id::text,ir.product_variant_id::text,ir.quantity
		FROM inventory_reservations ir JOIN order_items oi ON oi.id=ir.order_item_id
		WHERE oi.order_id=$1 AND ir.status='active' AND ir.expires_at<=now()
		ORDER BY ir.product_variant_id FOR UPDATE OF ir`, orderID)
	if err != nil {
		return false, err
	}
	type reservation struct {
		orderItemID string
		variantID   string
		quantity    int
	}
	reservations := make([]reservation, 0)
	for rows.Next() {
		var item reservation
		if err := rows.Scan(&item.orderItemID, &item.variantID, &item.quantity); err != nil {
			rows.Close()
			return false, err
		}
		reservations = append(reservations, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return false, err
	}
	rows.Close()
	if len(reservations) == 0 {
		return false, nil
	}
	for _, item := range reservations {
		var onHand int
		if err := tx.QueryRow(ctx, `UPDATE inventory SET reserved=reserved-$2,updated_at=now()
			WHERE product_variant_id=$1 AND reserved >= $2 RETURNING on_hand`, item.variantID, item.quantity).Scan(&onHand); err != nil {
			return false, fmt.Errorf("release expired inventory reservation: %w", err)
		}
		result, err := tx.Exec(ctx, `UPDATE inventory_reservations SET status='expired',resolved_at=now()
			WHERE order_item_id=$1 AND status='active' AND expires_at<=now()`, item.orderItemID)
		if err != nil {
			return false, err
		}
		if result.RowsAffected() != 1 {
			return false, errors.New("reservation changed while expiration was locked")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO inventory_movements(product_variant_id,movement_type,quantity_delta,quantity_after,reference_type,reference_id,note)
			VALUES($1,'release',$2,$3,'order',$4,'Expired unpaid checkout reservation')`, item.variantID, -item.quantity, onHand, orderID); err != nil {
			return false, err
		}
	}
	var hasActive bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM inventory_reservations ir JOIN order_items oi ON oi.id=ir.order_item_id WHERE oi.order_id=$1 AND ir.status='active')`, orderID).Scan(&hasActive); err != nil {
		return false, err
	}
	if hasActive {
		return false, nil
	}
	var updated bool
	if err := tx.QueryRow(ctx, `UPDATE orders o SET status='cancelled',version=version+1,updated_at=now()
		WHERE o.id=$1 AND o.status='pending_payment'
		  AND NOT EXISTS(SELECT 1 FROM payment_attempts pa WHERE pa.order_id=o.id AND pa.status NOT IN ('failed','cancelled'))
		RETURNING true`, orderID).Scan(&updated); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, errors.New("pending order changed while reservations were expiring")
		}
		return false, err
	}
	if !updated {
		return false, nil
	}
	if _, err := tx.Exec(ctx, `INSERT INTO order_status_events(order_id,from_status,to_status,reason)
		VALUES($1,'pending_payment','cancelled','Payment window expired and stock reservations were released')`, orderID); err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_logs(action,entity_type,entity_id,request_id,metadata)
		VALUES('checkout.reservations.expired','order',$1,'reservation-expiry-worker',jsonb_build_object('reservationCount',$2))`, orderID, len(reservations)); err != nil {
		return false, err
	}
	return true, nil
}

func RunReservationExpirationWorker(ctx context.Context, service *Service, logger *slog.Logger) {
	if service == nil || logger == nil {
		return
	}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			workCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			count, err := service.ReleaseExpiredReservations(workCtx, 100)
			cancel()
			if err != nil {
				logger.Error("reservation_expiration_failed", "error_type", fmt.Sprintf("%T", err))
				continue
			}
			if count > 0 {
				logger.Info("reservation_expiration_completed", "orders_cancelled", count)
			}
		}
	}
}
