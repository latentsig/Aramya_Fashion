package payments

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrOrderUnavailable    = errors.New("payment order is unavailable")
	ErrAttemptPending      = errors.New("an earlier payment attempt needs reconciliation")
	ErrPaymentMismatch     = errors.New("payment details do not match the order")
	ErrPaymentNotReady     = errors.New("payment is not captured")
	ErrProviderUnavailable = errors.New("payment provider is temporarily unavailable")
)

type CheckoutProvider interface {
	KeyID() string
	CreateOrder(context.Context, OrderRequest) (Order, error)
	VerifyCheckoutSignature(orderID, paymentID, signature string) bool
	FetchPayment(context.Context, string) (Payment, error)
	VerifyWebhookSignature([]byte, string) bool
}

type CheckoutPaymentService struct {
	pool         *pgxpool.Pool
	provider     CheckoutProvider
	queryTimeout time.Duration
}

func NewCheckoutPaymentService(pool *pgxpool.Pool, provider CheckoutProvider, queryTimeout time.Duration) (*CheckoutPaymentService, error) {
	if pool == nil || provider == nil || queryTimeout <= 0 {
		return nil, errors.New("payment database, provider, and positive query timeout are required")
	}
	return &CheckoutPaymentService{pool: pool, provider: provider, queryTimeout: queryTimeout}, nil
}

type CheckoutSession struct {
	AttemptID       string `json:"attemptId"`
	OrderID         string `json:"orderId"`
	OrderNumber     string `json:"orderNumber"`
	ProviderOrderID string `json:"providerOrderId"`
	KeyID           string `json:"keyId"`
	AmountPaise     int64  `json:"amountPaise"`
	Currency        string `json:"currency"`
	Status          string `json:"status"`
}

type PaymentResult struct {
	OrderID       string `json:"orderId"`
	OrderNumber   string `json:"orderNumber"`
	OrderStatus   string `json:"orderStatus"`
	PaymentStatus string `json:"paymentStatus"`
}

// Start creates one durable attempt before calling Razorpay. A request that
// loses the provider response leaves the attempt unresolved; retries with the
// same key never create another provider order implicitly.
func (s *CheckoutPaymentService) Start(ctx context.Context, orderID, customerID, guestCartID, idempotencyKey string) (CheckoutSession, error) {
	if len(idempotencyKey) < 16 || len(idempotencyKey) > 128 || strings.ContainsAny(idempotencyKey, "\r\n\x00") {
		return CheckoutSession{}, ErrPaymentMismatch
	}
	dbCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()
	tx, err := s.pool.Begin(dbCtx)
	if err != nil {
		return CheckoutSession{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var orderNumber, status, currency, ownerID, orderIdempotencyKey string
	var amount int64
	err = tx.QueryRow(dbCtx, `SELECT order_number,status,currency,total_paise,COALESCE(customer_id::text,''),idempotency_key
		FROM orders WHERE id=$1 FOR UPDATE`, orderID).Scan(&orderNumber, &status, &currency, &amount, &ownerID, &orderIdempotencyKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return CheckoutSession{}, ErrOrderUnavailable
	}
	if err != nil {
		return CheckoutSession{}, err
	}
	owned := customerID != "" && ownerID == customerID
	guestOwned := customerID == "" && guestCartID != "" && strings.HasPrefix(orderIdempotencyKey, "guest:"+guestCartID+":")
	if !owned && !guestOwned {
		return CheckoutSession{}, ErrOrderUnavailable
	}
	if status != "pending_payment" || amount <= 0 || currency != "INR" || len(orderNumber) > 40 {
		return CheckoutSession{}, ErrOrderUnavailable
	}

	requestHash := sha256.Sum256([]byte(orderID + "|" + idempotencyKey))
	attemptKey := "rzp_" + hex.EncodeToString(requestHash[:])
	var session CheckoutSession
	created := false
	err = tx.QueryRow(dbCtx, `SELECT pa.id::text,COALESCE(pa.provider_order_id,''),pa.status,pa.amount_paise,pa.currency
		FROM payment_attempts pa WHERE pa.idempotency_key=$1 FOR UPDATE`, attemptKey).
		Scan(&session.AttemptID, &session.ProviderOrderID, &session.Status, &session.AmountPaise, &session.Currency)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return CheckoutSession{}, err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		var latestStatus string
		err = tx.QueryRow(dbCtx, `SELECT status FROM payment_attempts WHERE order_id=$1 ORDER BY created_at DESC,id DESC LIMIT 1 FOR UPDATE`, orderID).Scan(&latestStatus)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return CheckoutSession{}, err
		}
		if err == nil && latestStatus != "failed" && latestStatus != "cancelled" {
			return CheckoutSession{}, ErrAttemptPending
		}
		err = tx.QueryRow(dbCtx, `INSERT INTO payment_attempts(order_id,provider,status,amount_paise,currency,idempotency_key)
			VALUES($1,'razorpay','initiated',$2,$3,$4) RETURNING id::text,status,amount_paise,currency`, orderID, amount, currency, attemptKey).
			Scan(&session.AttemptID, &session.Status, &session.AmountPaise, &session.Currency)
		if err != nil {
			return CheckoutSession{}, err
		}
		created = true
	}
	session.OrderID, session.OrderNumber, session.KeyID = orderID, orderNumber, s.provider.KeyID()
	if session.AmountPaise != amount || session.Currency != currency {
		return CheckoutSession{}, ErrPaymentMismatch
	}
	if session.ProviderOrderID != "" {
		if session.Status != "pending" {
			return CheckoutSession{}, ErrAttemptPending
		}
		if err := tx.Commit(dbCtx); err != nil {
			return CheckoutSession{}, err
		}
		return session, nil
	}
	if session.Status != "initiated" || !created {
		return CheckoutSession{}, ErrAttemptPending
	}
	if err := tx.Commit(dbCtx); err != nil {
		return CheckoutSession{}, err
	}

	providerOrder, err := s.provider.CreateOrder(ctx, OrderRequest{Amount: amount, Currency: currency, Receipt: orderNumber})
	if err != nil {
		return CheckoutSession{}, fmt.Errorf("%w: create Razorpay order: %v", ErrProviderUnavailable, err)
	}
	updateCtx, updateCancel := context.WithTimeout(ctx, s.queryTimeout)
	defer updateCancel()
	result, err := s.pool.Exec(updateCtx, `UPDATE payment_attempts SET provider_order_id=$2,status='pending',updated_at=now()
		WHERE id=$1 AND status='initiated' AND provider_order_id IS NULL`, session.AttemptID, providerOrder.ID)
	if err != nil {
		return CheckoutSession{}, err
	}
	if result.RowsAffected() != 1 {
		return CheckoutSession{}, ErrAttemptPending
	}
	session.ProviderOrderID, session.Status = providerOrder.ID, "pending"
	return session, nil
}

// Verify checks the signed callback and then fetches authoritative payment
// state from Razorpay. Only a captured, amount-matched payment consumes stock.
func (s *CheckoutPaymentService) Verify(ctx context.Context, orderID, customerID, guestCartID, providerOrderID, paymentID, signature string) (PaymentResult, error) {
	dbCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()
	var attemptID, expectedOrderID, attemptStatus, ownerID, orderKey, orderNumber, orderStatus, currency string
	var amount, orderTotal int64
	err := s.pool.QueryRow(dbCtx, `SELECT pa.id::text,pa.provider_order_id,pa.status,pa.amount_paise,pa.currency,
		o.order_number,o.status,o.total_paise,COALESCE(o.customer_id::text,''),o.idempotency_key
		FROM payment_attempts pa JOIN orders o ON o.id=pa.order_id
		WHERE o.id=$1 AND pa.provider_order_id=$2`, orderID, providerOrderID).
		Scan(&attemptID, &expectedOrderID, &attemptStatus, &amount, &currency, &orderNumber, &orderStatus, &orderTotal, &ownerID, &orderKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return PaymentResult{}, ErrOrderUnavailable
	}
	if err != nil {
		return PaymentResult{}, err
	}
	owned := customerID != "" && ownerID == customerID
	guestOwned := customerID == "" && guestCartID != "" && strings.HasPrefix(orderKey, "guest:"+guestCartID+":")
	if !owned && !guestOwned {
		return PaymentResult{}, ErrOrderUnavailable
	}
	if amount != orderTotal {
		return PaymentResult{}, ErrPaymentMismatch
	}
	if !s.provider.VerifyCheckoutSignature(expectedOrderID, paymentID, signature) {
		return PaymentResult{}, ErrPaymentMismatch
	}
	providerPayment, err := s.provider.FetchPayment(ctx, paymentID)
	if err != nil {
		return PaymentResult{}, fmt.Errorf("%w: fetch Razorpay payment: %v", ErrProviderUnavailable, err)
	}
	if providerPayment.ID != paymentID || providerPayment.OrderID != expectedOrderID || providerPayment.Amount != amount || providerPayment.Currency != currency {
		return PaymentResult{}, ErrPaymentMismatch
	}
	nextStatus := "pending"
	switch providerPayment.Status {
	case "captured":
		if !providerPayment.Captured {
			return PaymentResult{}, ErrPaymentMismatch
		}
		nextStatus = "captured"
	case "authorized":
		nextStatus = "authorized"
	case "failed":
		nextStatus = "failed"
	case "created":
		nextStatus = "pending"
	default:
		return PaymentResult{}, ErrPaymentNotReady
	}

	applyCtx, applyCancel := context.WithTimeout(ctx, s.queryTimeout)
	defer applyCancel()
	tx, err := s.pool.Begin(applyCtx)
	if err != nil {
		return PaymentResult{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	result, err := s.applyProviderPaymentTx(applyCtx, tx, attemptID, orderID, orderNumber, paymentID, nextStatus, "razorpay-callback")
	if err != nil {
		return PaymentResult{}, err
	}
	if err := tx.Commit(applyCtx); err != nil {
		return PaymentResult{}, err
	}
	return result, nil
}

func (s *CheckoutPaymentService) applyProviderPaymentTx(ctx context.Context, tx pgx.Tx, attemptID, orderID, orderNumber, paymentID, nextStatus, requestID string) (PaymentResult, error) {
	var attemptStatus, orderStatus string
	if err := tx.QueryRow(ctx, `SELECT pa.status,o.status FROM payment_attempts pa JOIN orders o ON o.id=pa.order_id
		WHERE pa.id=$1 FOR UPDATE OF pa,o`, attemptID).Scan(&attemptStatus, &orderStatus); err != nil {
		return PaymentResult{}, err
	}
	if attemptStatus != "captured" || nextStatus == "captured" {
		if _, err := tx.Exec(ctx, `UPDATE payment_attempts SET status=$2,provider_payment_id=$3,
			signature_verified_at=now(),failure_code=CASE WHEN $2='failed' THEN 'provider_failed' ELSE '' END,updated_at=now() WHERE id=$1`, attemptID, nextStatus, paymentID); err != nil {
			return PaymentResult{}, err
		}
		attemptStatus = nextStatus
	}
	if nextStatus == "captured" && orderStatus == "pending_payment" {
		if err := consumeOrderReservations(ctx, tx, orderID); err != nil {
			return PaymentResult{}, err
		}
		updated, err := tx.Exec(ctx, `UPDATE orders SET status='paid',version=version+1,updated_at=now() WHERE id=$1 AND status='pending_payment'`, orderID)
		if err != nil {
			return PaymentResult{}, err
		}
		if updated.RowsAffected() != 1 {
			return PaymentResult{}, ErrAttemptPending
		}
		if _, err := tx.Exec(ctx, `INSERT INTO order_status_events(order_id,from_status,to_status,reason)
			VALUES($1,'pending_payment','paid','Razorpay capture verified by server')`, orderID); err != nil {
			return PaymentResult{}, err
		}
		orderStatus = "paid"
	} else if nextStatus == "captured" && orderStatus != "paid" && orderStatus != "confirmed" && orderStatus != "processing" && orderStatus != "ready_to_ship" && orderStatus != "shipped" && orderStatus != "delivered" {
		if _, err := tx.Exec(ctx, `INSERT INTO audit_logs(action,entity_type,entity_id,request_id,metadata)
			SELECT 'payment.capture_review_required','order',$1,$3,jsonb_build_object('paymentAttemptId',$2)
			WHERE NOT EXISTS (SELECT 1 FROM audit_logs WHERE action='payment.capture_review_required' AND entity_id=$1 AND metadata->>'paymentAttemptId'=$2)`, orderID, attemptID, requestID); err != nil {
			return PaymentResult{}, err
		}
	}
	if nextStatus == "captured" && (orderStatus == "paid" || orderStatus == "confirmed" || orderStatus == "processing" || orderStatus == "ready_to_ship" || orderStatus == "shipped" || orderStatus == "delivered") {
		var capturedAttempts int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM payment_attempts WHERE order_id=$1 AND status='captured'`, orderID).Scan(&capturedAttempts); err != nil {
			return PaymentResult{}, err
		}
		if capturedAttempts > 1 {
			if _, err := tx.Exec(ctx, `INSERT INTO audit_logs(action,entity_type,entity_id,request_id,metadata)
				SELECT 'payment.multiple_captures_review_required','order',$1,$3,jsonb_build_object('capturedAttemptCount',$2)
				WHERE NOT EXISTS (SELECT 1 FROM audit_logs WHERE action='payment.multiple_captures_review_required' AND entity_id=$1)`, orderID, capturedAttempts, requestID); err != nil {
				return PaymentResult{}, err
			}
		}
	}
	return PaymentResult{OrderID: orderID, OrderNumber: orderNumber, OrderStatus: orderStatus, PaymentStatus: attemptStatus}, nil
}

func consumeOrderReservations(ctx context.Context, tx pgx.Tx, orderID string) error {
	rows, err := tx.Query(ctx, `SELECT ir.order_item_id::text,ir.product_variant_id::text,ir.quantity
		FROM inventory_reservations ir JOIN order_items oi ON oi.id=ir.order_item_id
		WHERE oi.order_id=$1 AND ir.status='active' ORDER BY ir.product_variant_id FOR UPDATE OF ir`, orderID)
	if err != nil {
		return err
	}
	type reservation struct {
		itemID, variantID string
		quantity          int
	}
	items := make([]reservation, 0)
	for rows.Next() {
		var item reservation
		if err := rows.Scan(&item.itemID, &item.variantID, &item.quantity); err != nil {
			rows.Close()
			return err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, item := range items {
		var onHand int
		if err := tx.QueryRow(ctx, `UPDATE inventory SET on_hand=on_hand-$2,reserved=reserved-$2,updated_at=now()
			WHERE product_variant_id=$1 AND on_hand >= $2 AND reserved >= $2 RETURNING on_hand`, item.variantID, item.quantity).Scan(&onHand); err != nil {
			return fmt.Errorf("consume reserved inventory: %w", err)
		}
		result, err := tx.Exec(ctx, `UPDATE inventory_reservations SET status='consumed',resolved_at=now() WHERE order_item_id=$1 AND status='active'`, item.itemID)
		if err != nil {
			return err
		}
		if result.RowsAffected() != 1 {
			return errors.New("inventory reservation changed while capture was being applied")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO inventory_movements(product_variant_id,movement_type,quantity_delta,quantity_after,reference_type,reference_id,note)
			VALUES($1,'sale',$2,$3,'order',$4,'Razorpay capture verified')`, item.variantID, -item.quantity, onHand, orderID); err != nil {
			return err
		}
	}
	return nil
}
