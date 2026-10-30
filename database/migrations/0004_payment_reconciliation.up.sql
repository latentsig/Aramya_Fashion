ALTER TABLE payment_attempts
    ADD COLUMN last_reconciled_at timestamptz,
    ADD COLUMN reconciliation_lease_until timestamptz;

CREATE INDEX payment_attempts_reconciliation_idx
    ON payment_attempts(COALESCE(last_reconciled_at, updated_at), created_at)
    WHERE status IN ('pending', 'authorized') AND provider_order_id IS NOT NULL;
