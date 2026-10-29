package appointments

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrIdempotencyConflict = errors.New("studio request key was already used with different details")

type AppointmentInput struct {
	Name     string
	Email    string
	Phone    string
	Interest string
	Note     string
}

type CustomOrderInput struct {
	Name         string
	Email        string
	Phone        string
	GarmentType  string
	Occasion     string
	Requirements string
}

type CreatedRequest struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

type Repository struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewRepository(pool *pgxpool.Pool, queryTimeout time.Duration) *Repository {
	return &Repository{pool: pool, queryTimeout: queryTimeout}
}

func (r *Repository) CreateAppointment(ctx context.Context, customerID, idempotencyKey string, input AppointmentInput) (CreatedRequest, bool, error) {
	dbCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	return r.createIdempotent(dbCtx, "appointment", customerID, idempotencyKey, input, func(tx pgx.Tx) (CreatedRequest, error) {
		var result CreatedRequest
		err := tx.QueryRow(dbCtx, `
			INSERT INTO appointments(customer_id, name, email, phone, interest, note)
			VALUES(NULLIF($1, '')::uuid, $2, $3, $4, $5, $6)
			RETURNING id::text, status, created_at`, customerID, input.Name, input.Email, input.Phone, input.Interest, input.Note).
			Scan(&result.ID, &result.Status, &result.CreatedAt)
		return result, err
	})
}

func (r *Repository) CreateCustomOrderRequest(ctx context.Context, customerID, idempotencyKey string, input CustomOrderInput) (CreatedRequest, bool, error) {
	dbCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	return r.createIdempotent(dbCtx, "custom-order", customerID, idempotencyKey, input, func(tx pgx.Tx) (CreatedRequest, error) {
		var result CreatedRequest
		err := tx.QueryRow(dbCtx, `
			INSERT INTO custom_order_requests(customer_id, name, email, phone, garment_type, occasion, requirements)
			VALUES(NULLIF($1, '')::uuid, $2, $3, $4, $5, $6, $7)
			RETURNING id::text, status, created_at`, customerID, input.Name, input.Email, input.Phone, input.GarmentType, input.Occasion, input.Requirements).
			Scan(&result.ID, &result.Status, &result.CreatedAt)
		return result, err
	})
}

type create func(pgx.Tx) (CreatedRequest, error)

func (r *Repository) createIdempotent(ctx context.Context, kind, customerID, key string, request any, save create) (CreatedRequest, bool, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return CreatedRequest{}, false, err
	}
	hash := sha256.Sum256(body)
	scope := "studio:" + kind + ":guest"
	if customerID != "" {
		scope = "studio:" + kind + ":user:" + customerID
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return CreatedRequest{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `INSERT INTO idempotency_records(scope, idempotency_key, request_sha256, expires_at)
		VALUES($1,$2,$3,now()+interval '24 hours')
		ON CONFLICT(scope,idempotency_key) DO UPDATE SET
			request_sha256=EXCLUDED.request_sha256,response_status=NULL,response_body=NULL,completed_at=NULL,
			expires_at=EXCLUDED.expires_at,created_at=now()
		WHERE idempotency_records.expires_at <= now()`, scope, key, hash[:]); err != nil {
		return CreatedRequest{}, false, err
	}
	var storedHash []byte
	var responseBody []byte
	err = tx.QueryRow(ctx, `SELECT request_sha256,response_body FROM idempotency_records
		WHERE scope=$1 AND idempotency_key=$2 FOR UPDATE`, scope, key).Scan(&storedHash, &responseBody)
	if err != nil {
		return CreatedRequest{}, false, err
	}
	if !bytes.Equal(storedHash, hash[:]) {
		return CreatedRequest{}, false, ErrIdempotencyConflict
	}
	if len(responseBody) > 0 {
		var saved CreatedRequest
		if err := json.Unmarshal(responseBody, &saved); err != nil {
			return CreatedRequest{}, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return CreatedRequest{}, false, err
		}
		return saved, true, nil
	}
	saved, err := save(tx)
	if err != nil {
		return CreatedRequest{}, false, err
	}
	responseBody, err = json.Marshal(saved)
	if err != nil {
		return CreatedRequest{}, false, err
	}
	if _, err = tx.Exec(ctx, `UPDATE idempotency_records SET response_status=201,response_body=$3::jsonb,completed_at=now()
		WHERE scope=$1 AND idempotency_key=$2`, scope, key, string(responseBody)); err != nil {
		return CreatedRequest{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CreatedRequest{}, false, err
	}
	return saved, false, nil
}
