package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound          = errors.New("studio request not found")
	ErrInvalidStatus     = errors.New("invalid studio request status")
	ErrInvalidTransition = errors.New("studio request status transition is not allowed")
	ErrStaleRequest      = errors.New("studio request changed since it was loaded")
	ErrInvalidQuote      = errors.New("a positive quotation in paise is required for quoted requests")
)

type ListFilter struct {
	Status string
	Search string
	Limit  int
	Offset int
}

type Appointment struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Email       string     `json:"email"`
	Phone       string     `json:"phone"`
	PreferredAt *time.Time `json:"preferredAt"`
	Interest    string     `json:"interest"`
	Note        string     `json:"note"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type CustomOrderRequest struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Email          string          `json:"email"`
	Phone          string          `json:"phone"`
	GarmentType    string          `json:"garmentType"`
	Occasion       string          `json:"occasion"`
	Requirements   string          `json:"requirements"`
	Measurements   json.RawMessage `json:"measurements"`
	QuotationPaise *int64          `json:"quotationPaise"`
	Currency       string          `json:"currency"`
	Status         string          `json:"status"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

type StudioRepository struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewStudioRepository(pool *pgxpool.Pool, queryTimeout time.Duration) *StudioRepository {
	return &StudioRepository{pool: pool, queryTimeout: queryTimeout}
}

func (r *StudioRepository) ListAppointments(ctx context.Context, filter ListFilter) ([]Appointment, error) {
	dbCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	rows, err := r.pool.Query(dbCtx, `SELECT id::text,name,email,phone,preferred_at,interest,note,status,created_at,updated_at
		FROM appointments
		WHERE ($1='' OR status=$1) AND ($2='' OR name ILIKE '%'||$2||'%' OR email ILIKE '%'||$2||'%' OR phone ILIKE '%'||$2||'%')
		ORDER BY created_at DESC,id DESC LIMIT $3 OFFSET $4`, filter.Status, filter.Search, filter.Limit, filter.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Appointment, 0)
	for rows.Next() {
		var item Appointment
		var preferredAt pgtype.Timestamptz
		if err := rows.Scan(&item.ID, &item.Name, &item.Email, &item.Phone, &preferredAt, &item.Interest, &item.Note, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		if preferredAt.Valid {
			value := preferredAt.Time
			item.PreferredAt = &value
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *StudioRepository) ListCustomOrders(ctx context.Context, filter ListFilter) ([]CustomOrderRequest, error) {
	dbCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	rows, err := r.pool.Query(dbCtx, `SELECT id::text,name,email,phone,garment_type,occasion,requirements,measurements,
		quotation_paise,currency,status,created_at,updated_at FROM custom_order_requests
		WHERE ($1='' OR status=$1) AND ($2='' OR name ILIKE '%'||$2||'%' OR email ILIKE '%'||$2||'%' OR phone ILIKE '%'||$2||'%' OR garment_type ILIKE '%'||$2||'%')
		ORDER BY created_at DESC,id DESC LIMIT $3 OFFSET $4`, filter.Status, filter.Search, filter.Limit, filter.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]CustomOrderRequest, 0)
	for rows.Next() {
		var item CustomOrderRequest
		var quotation pgtype.Int8
		if err := rows.Scan(&item.ID, &item.Name, &item.Email, &item.Phone, &item.GarmentType, &item.Occasion, &item.Requirements, &item.Measurements, &quotation, &item.Currency, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		if quotation.Valid {
			value := quotation.Int64
			item.QuotationPaise = &value
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *StudioRepository) UpdateAppointment(ctx context.Context, id, expectedStatus, targetStatus, actorID, requestID string) (Appointment, error) {
	if !ValidAppointmentStatus(targetStatus) {
		return Appointment{}, ErrInvalidStatus
	}
	if !CanTransitionAppointment(expectedStatus, targetStatus) {
		return Appointment{}, ErrInvalidTransition
	}
	dbCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	tx, err := r.pool.Begin(dbCtx)
	if err != nil {
		return Appointment{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var current string
	if err := tx.QueryRow(dbCtx, `SELECT status FROM appointments WHERE id=$1 FOR UPDATE`, id).Scan(&current); errors.Is(err, pgx.ErrNoRows) {
		return Appointment{}, ErrNotFound
	} else if err != nil {
		return Appointment{}, err
	}
	if current != expectedStatus {
		return Appointment{}, ErrStaleRequest
	}
	var item Appointment
	var preferredAt pgtype.Timestamptz
	err = tx.QueryRow(dbCtx, `UPDATE appointments SET status=$2,updated_at=now() WHERE id=$1
		RETURNING id::text,name,email,phone,preferred_at,interest,note,status,created_at,updated_at`, id, targetStatus).
		Scan(&item.ID, &item.Name, &item.Email, &item.Phone, &preferredAt, &item.Interest, &item.Note, &item.Status, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return Appointment{}, err
	}
	if preferredAt.Valid {
		value := preferredAt.Time
		item.PreferredAt = &value
	}
	if _, err := tx.Exec(dbCtx, `INSERT INTO audit_logs(actor_user_id,action,entity_type,entity_id,request_id,metadata)
		VALUES($1,'appointment.status_changed','appointment',$2,$3,jsonb_build_object('from',$4::text,'to',$5::text))`, actorID, id, requestID, current, targetStatus); err != nil {
		return Appointment{}, err
	}
	if err := tx.Commit(dbCtx); err != nil {
		return Appointment{}, err
	}
	return item, nil
}

func (r *StudioRepository) UpdateCustomOrder(ctx context.Context, id, expectedStatus, targetStatus string, quotationPaise *int64, actorID, requestID string) (CustomOrderRequest, error) {
	if !ValidCustomOrderStatus(targetStatus) {
		return CustomOrderRequest{}, ErrInvalidStatus
	}
	if quotationPaise != nil && *quotationPaise <= 0 {
		return CustomOrderRequest{}, ErrInvalidQuote
	}
	if targetStatus != "quoted" && quotationPaise != nil {
		return CustomOrderRequest{}, ErrInvalidQuote
	}
	if !CanTransitionCustomOrder(expectedStatus, targetStatus) {
		return CustomOrderRequest{}, ErrInvalidTransition
	}
	dbCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	tx, err := r.pool.Begin(dbCtx)
	if err != nil {
		return CustomOrderRequest{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var current string
	var currentQuote pgtype.Int8
	if err := tx.QueryRow(dbCtx, `SELECT status,quotation_paise FROM custom_order_requests WHERE id=$1 FOR UPDATE`, id).Scan(&current, &currentQuote); errors.Is(err, pgx.ErrNoRows) {
		return CustomOrderRequest{}, ErrNotFound
	} else if err != nil {
		return CustomOrderRequest{}, err
	}
	if current != expectedStatus {
		return CustomOrderRequest{}, ErrStaleRequest
	}
	if targetStatus == "quoted" && (quotationPaise == nil || *quotationPaise <= 0) {
		return CustomOrderRequest{}, ErrInvalidQuote
	}
	if targetStatus == "awaiting_approval" && !currentQuote.Valid {
		return CustomOrderRequest{}, ErrInvalidQuote
	}
	var item CustomOrderRequest
	var quote pgtype.Int8
	err = tx.QueryRow(dbCtx, `UPDATE custom_order_requests SET status=$2,quotation_paise=COALESCE($3,quotation_paise),updated_at=now()
		WHERE id=$1 RETURNING id::text,name,email,phone,garment_type,occasion,requirements,measurements,
		quotation_paise,currency,status,created_at,updated_at`, id, targetStatus, quotationPaise).
		Scan(&item.ID, &item.Name, &item.Email, &item.Phone, &item.GarmentType, &item.Occasion, &item.Requirements, &item.Measurements, &quote, &item.Currency, &item.Status, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return CustomOrderRequest{}, err
	}
	if quote.Valid {
		value := quote.Int64
		item.QuotationPaise = &value
	}
	metadata := map[string]any{"from": current, "to": targetStatus}
	if quotationPaise != nil {
		metadata["quotationPaise"] = *quotationPaise
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return CustomOrderRequest{}, fmt.Errorf("encode audit metadata: %w", err)
	}
	if _, err := tx.Exec(dbCtx, `INSERT INTO audit_logs(actor_user_id,action,entity_type,entity_id,request_id,metadata)
		VALUES($1,'custom_order_request.status_changed','custom_order_request',$2,$3,$4::jsonb)`, actorID, id, requestID, string(metadataJSON)); err != nil {
		return CustomOrderRequest{}, err
	}
	if err := tx.Commit(dbCtx); err != nil {
		return CustomOrderRequest{}, err
	}
	return item, nil
}

func ValidAppointmentStatus(status string) bool {
	switch status {
	case "requested", "confirmed", "completed", "cancelled":
		return true
	}
	return false
}

func CanTransitionAppointment(from, to string) bool {
	switch from {
	case "requested":
		return to == "confirmed" || to == "cancelled"
	case "confirmed":
		return to == "completed" || to == "cancelled"
	}
	return false
}

func ValidCustomOrderStatus(status string) bool {
	switch status {
	case "requested", "reviewing", "fitting", "quoted", "awaiting_approval", "approved", "in_progress", "completed", "declined", "cancelled":
		return true
	}
	return false
}

func CanTransitionCustomOrder(from, to string) bool {
	switch from {
	case "requested":
		return to == "reviewing" || to == "declined" || to == "cancelled"
	case "reviewing":
		return to == "fitting" || to == "quoted" || to == "declined" || to == "cancelled"
	case "fitting":
		return to == "quoted" || to == "declined" || to == "cancelled"
	case "quoted":
		return to == "awaiting_approval" || to == "declined" || to == "cancelled"
	case "awaiting_approval":
		return to == "declined" || to == "cancelled"
	}
	return false
}

func NormalizeSearch(value string) string { return strings.TrimSpace(value) }
