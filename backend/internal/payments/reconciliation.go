package payments

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

type ReconciliationReport struct {
	Checked  int `json:"checked"`
	Captured int `json:"captured"`
	Failed   int `json:"failed"`
	Pending  int `json:"pending"`
	Errors   int `json:"errors"`
}

type reconciliationCandidate struct {
	attemptID, orderID, orderNumber, providerOrderID, providerPaymentID string
	amount                                                              int64
	currency                                                            string
}

// ReconcilePayments checks aged pending attempts against Razorpay's order
// payment collection. It applies only provider-validated payment states.
func (s *CheckoutPaymentService) ReconcilePayments(ctx context.Context, limit int) (ReconciliationReport, error) {
	var report ReconciliationReport
	if limit < 1 || limit > 100 {
		return report, errors.New("reconciliation batch limit must be between 1 and 100")
	}
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	claimTx, err := s.pool.Begin(queryCtx)
	if err != nil {
		cancel()
		return report, err
	}
	rows, err := claimTx.Query(queryCtx, `WITH candidates AS (
			SELECT pa.id, o.id AS order_id, o.order_number, pa.provider_order_id, COALESCE(pa.provider_payment_id,'') AS provider_payment_id, pa.amount_paise, pa.currency
			FROM payment_attempts pa JOIN orders o ON o.id=pa.order_id
			WHERE pa.status IN ('pending','authorized') AND pa.provider_order_id IS NOT NULL
			AND pa.created_at <= now()-interval '2 minutes'
			AND COALESCE(pa.last_reconciled_at,pa.updated_at) <= now()-interval '2 minutes'
			AND (pa.reconciliation_lease_until IS NULL OR pa.reconciliation_lease_until <= now())
			ORDER BY COALESCE(pa.last_reconciled_at,pa.updated_at),pa.created_at
			LIMIT $1
			FOR UPDATE OF pa SKIP LOCKED
		)
		UPDATE payment_attempts pa SET reconciliation_lease_until=now()+interval '1 minute'
		FROM candidates c WHERE pa.id=c.id
		RETURNING pa.id::text,c.order_id::text,c.order_number,c.provider_order_id,c.provider_payment_id,c.amount_paise,c.currency`, limit)
	if err != nil {
		_ = claimTx.Rollback(context.Background())
		cancel()
		return report, err
	}
	candidates := make([]reconciliationCandidate, 0)
	for rows.Next() {
		var candidate reconciliationCandidate
		if err := rows.Scan(&candidate.attemptID, &candidate.orderID, &candidate.orderNumber, &candidate.providerOrderID, &candidate.providerPaymentID, &candidate.amount, &candidate.currency); err != nil {
			rows.Close()
			_ = claimTx.Rollback(context.Background())
			cancel()
			return report, err
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		_ = claimTx.Rollback(context.Background())
		cancel()
		return report, err
	}
	rows.Close()
	if err := claimTx.Commit(queryCtx); err != nil {
		cancel()
		return report, err
	}
	cancel()

	for _, candidate := range candidates {
		report.Checked++
		providerCtx, providerCancel := context.WithTimeout(ctx, s.queryTimeout)
		providerPayments, providerErr := s.provider.FetchOrderPayments(providerCtx, candidate.providerOrderID)
		providerCancel()
		if providerErr != nil {
			report.Errors++
			if err := s.markReconciliationAttempt(ctx, candidate.attemptID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				report.Errors++
			}
			continue
		}

		payment, nextStatus, captureCount, valid := selectReconciliationPayment(providerPayments, candidate)
		if !valid {
			report.Errors++
			_ = s.markReconciliationAttempt(ctx, candidate.attemptID)
			continue
		}
		if payment == nil {
			if err := s.markReconciliationAttempt(ctx, candidate.attemptID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				report.Errors++
			} else {
				report.Pending++
			}
			continue
		}

		applyCtx, applyCancel := context.WithTimeout(ctx, s.queryTimeout)
		tx, err := s.pool.Begin(applyCtx)
		var applied PaymentResult
		if err == nil {
			applied, err = s.applyProviderPaymentTx(applyCtx, tx, candidate.attemptID, candidate.orderID, candidate.orderNumber, payment.ID, nextStatus, "razorpay-reconciliation", false)
		}
		if err == nil {
			_, err = tx.Exec(applyCtx, `UPDATE payment_attempts SET last_reconciled_at=now(),reconciliation_lease_until=NULL WHERE id=$1`, candidate.attemptID)
		}
		if err == nil && captureCount > 1 {
			_, err = tx.Exec(applyCtx, `INSERT INTO audit_logs(action,entity_type,entity_id,request_id,metadata)
				SELECT 'payment.multiple_provider_captures_review_required','order',$1,'razorpay-reconciliation',jsonb_build_object('paymentAttemptId',$2,'capturedPaymentCount',$3)
				WHERE NOT EXISTS (SELECT 1 FROM audit_logs WHERE action='payment.multiple_provider_captures_review_required' AND entity_id=$1 AND metadata->>'paymentAttemptId'=$2)`, candidate.orderID, candidate.attemptID, captureCount)
		}
		if err == nil {
			err = tx.Commit(applyCtx)
		} else if tx != nil {
			_ = tx.Rollback(context.Background())
		}
		applyCancel()
		if err != nil {
			report.Errors++
			continue
		}
		if applied.PaymentStatus == "captured" {
			report.Captured++
		} else if applied.PaymentStatus == "failed" {
			report.Failed++
		} else {
			report.Pending++
		}
	}
	return report, nil
}

func selectReconciliationPayment(providerPayments []Payment, candidate reconciliationCandidate) (*Payment, string, int, bool) {
	var captured, preferredCapture, authorized, failed *Payment
	captureCount := 0
	for i := range providerPayments {
		payment := &providerPayments[i]
		if payment.OrderID != candidate.providerOrderID || payment.Amount != candidate.amount || payment.Currency != candidate.currency {
			return nil, "", 0, false
		}
		switch payment.Status {
		case "captured":
			if payment.Captured {
				captureCount++
				if captured == nil {
					captured = payment
				}
				if payment.ID == candidate.providerPaymentID {
					preferredCapture = payment
				}
			}
		case "authorized":
			if authorized == nil {
				authorized = payment
			}
		case "failed":
			if failed == nil {
				failed = payment
			}
		}
	}
	if captured != nil {
		if preferredCapture != nil {
			captured = preferredCapture
		}
		return captured, "captured", captureCount, true
	}
	if authorized != nil {
		return authorized, "authorized", captureCount, true
	}
	if failed != nil && len(providerPayments) > 0 {
		for _, payment := range providerPayments {
			if payment.Status != "failed" {
				return nil, "", captureCount, true
			}
		}
		return failed, "failed", captureCount, true
	}
	return nil, "", captureCount, true
}

func (s *CheckoutPaymentService) markReconciliationAttempt(ctx context.Context, attemptID string) error {
	markCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()
	result, err := s.pool.Exec(markCtx, `UPDATE payment_attempts SET last_reconciled_at=now(),reconciliation_lease_until=NULL
		WHERE id=$1 AND status IN ('pending','authorized')`, attemptID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// RunReconciliationWorker periodically reconciles aged attempts. The service
// enforces expiring per-attempt leases and backoff in PostgreSQL, coordinating
// work across API instances while recovering automatically from worker crashes.
func RunReconciliationWorker(ctx context.Context, service *CheckoutPaymentService, logger *slog.Logger, recordOutcome func(string)) {
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
			workCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
			report, err := service.ReconcilePayments(workCtx, 25)
			cancel()
			if err != nil {
				logger.Error("payment_reconciliation_failed", "error_type", "database_or_service_error")
				if recordOutcome != nil {
					recordOutcome("error")
				}
				continue
			}
			if report.Checked > 0 {
				logger.Info("payment_reconciliation_completed", "checked", report.Checked, "captured", report.Captured,
					"failed", report.Failed, "pending", report.Pending, "errors", report.Errors)
				if recordOutcome != nil {
					for i := 0; i < report.Captured; i++ {
						recordOutcome("captured")
					}
					for i := 0; i < report.Failed; i++ {
						recordOutcome("payment_failed")
					}
					for i := 0; i < report.Pending; i++ {
						recordOutcome("pending")
					}
					for i := 0; i < report.Errors; i++ {
						recordOutcome("error")
					}
				}
			}
		}
	}
}
