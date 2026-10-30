package payments

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jackc/pgx/v5"
)

var (
	ErrWebhookSignature = errors.New("webhook signature is invalid")
	ErrWebhookConflict  = errors.New("webhook event ID was reused with different content")
	ErrWebhookInvalid   = errors.New("webhook payload is invalid")
)

type WebhookResult struct {
	Duplicate bool `json:"duplicate"`
}

type razorpayWebhook struct {
	Event   string `json:"event"`
	Payload struct {
		Payment struct {
			Entity struct {
				ID       string `json:"id"`
				OrderID  string `json:"order_id"`
				Amount   int64  `json:"amount"`
				Currency string `json:"currency"`
				Status   string `json:"status"`
				Captured bool   `json:"captured"`
			} `json:"entity"`
		} `json:"payment"`
	} `json:"payload"`
}

// ProcessWebhook verifies the exact signed bytes and commits the inbox receipt
// and any resulting payment/order transition in one database transaction.
func (s *CheckoutPaymentService) ProcessWebhook(ctx context.Context, eventID, signature string, raw []byte) (WebhookResult, error) {
	if len(raw) == 0 || len(raw) > 1<<20 || !providerIDPattern.MatchString(eventID) {
		return WebhookResult{}, ErrWebhookInvalid
	}
	if !s.provider.VerifyWebhookSignature(raw, signature) {
		return WebhookResult{}, ErrWebhookSignature
	}
	var event razorpayWebhook
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	if err := decoder.Decode(&event); err != nil || event.Event == "" || len(event.Event) > 100 || strings.ContainsAny(event.Event, "\r\n\x00") {
		return WebhookResult{}, ErrWebhookInvalid
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return WebhookResult{}, ErrWebhookInvalid
	}
	digest := sha256.Sum256(raw)
	dbCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()
	tx, err := s.pool.Begin(dbCtx)
	if err != nil {
		return WebhookResult{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	inserted, err := tx.Exec(dbCtx, `INSERT INTO payment_events(provider,provider_event_id,event_type,payload_sha256)
		VALUES('razorpay',$1,$2,$3) ON CONFLICT(provider_event_id) DO NOTHING`, eventID, event.Event, digest[:])
	if err != nil {
		return WebhookResult{}, err
	}
	if inserted.RowsAffected() == 0 {
		var previous []byte
		if err := tx.QueryRow(dbCtx, `SELECT payload_sha256 FROM payment_events WHERE provider_event_id=$1 FOR UPDATE`, eventID).Scan(&previous); err != nil {
			return WebhookResult{}, err
		}
		if !equalBytes(previous, digest[:]) {
			return WebhookResult{}, ErrWebhookConflict
		}
		if err := tx.Commit(dbCtx); err != nil {
			return WebhookResult{}, err
		}
		return WebhookResult{Duplicate: true}, nil
	}

	processingError := ""
	if event.Event == "payment.authorized" || event.Event == "payment.captured" || event.Event == "payment.failed" {
		entity := event.Payload.Payment.Entity
		if !providerIDPattern.MatchString(entity.ID) || !providerIDPattern.MatchString(entity.OrderID) || entity.Amount <= 0 || entity.Currency != "INR" {
			return WebhookResult{}, ErrWebhookInvalid
		}
		var attemptID, orderID, orderNumber, currency string
		var amount int64
		err := tx.QueryRow(dbCtx, `SELECT pa.id::text,o.id::text,o.order_number,pa.amount_paise,pa.currency
			FROM payment_attempts pa JOIN orders o ON o.id=pa.order_id WHERE pa.provider_order_id=$1 FOR UPDATE OF pa,o`, entity.OrderID).
			Scan(&attemptID, &orderID, &orderNumber, &amount, &currency)
		if errors.Is(err, pgx.ErrNoRows) {
			processingError = "unmatched_order"
		} else if err != nil {
			return WebhookResult{}, err
		} else {
			if entity.Amount != amount || entity.Currency != currency {
				return WebhookResult{}, ErrPaymentMismatch
			}
			nextStatus := map[string]string{"payment.authorized": "authorized", "payment.failed": "failed", "payment.captured": "captured"}[event.Event]
			if event.Event == "payment.captured" && (entity.Status != "captured" || !entity.Captured) {
				return WebhookResult{}, ErrWebhookInvalid
			}
			if event.Event == "payment.authorized" && entity.Status != "authorized" {
				return WebhookResult{}, ErrWebhookInvalid
			}
			if event.Event == "payment.failed" && entity.Status != "failed" {
				return WebhookResult{}, ErrWebhookInvalid
			}
			if _, err := tx.Exec(dbCtx, `UPDATE payment_events SET payment_attempt_id=$2 WHERE provider_event_id=$1`, eventID, attemptID); err != nil {
				return WebhookResult{}, err
			}
			if _, err := s.applyProviderPaymentTx(dbCtx, tx, attemptID, orderID, orderNumber, entity.ID, nextStatus, "razorpay-webhook:"+eventID, true); err != nil {
				return WebhookResult{}, err
			}
		}
	}
	if _, err := tx.Exec(dbCtx, `UPDATE payment_events SET processed_at=now(),processing_error_code=$2 WHERE provider_event_id=$1`, eventID, processingError); err != nil {
		return WebhookResult{}, err
	}
	if err := tx.Commit(dbCtx); err != nil {
		return WebhookResult{}, fmt.Errorf("commit Razorpay webhook: %w", err)
	}
	return WebhookResult{}, nil
}

func equalBytes(a, b []byte) bool {
	return bytes.Equal(a, b)
}
