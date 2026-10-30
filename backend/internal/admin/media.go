package admin

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrMediaNotFound = errors.New("media upload was not found")

type MediaObject struct {
	ID               string    `json:"id"`
	ObjectKey        string    `json:"objectKey"`
	OriginalFilename string    `json:"originalFilename"`
	MimeType         string    `json:"mimeType"`
	ByteSize         int64     `json:"byteSize"`
	SHA256           string    `json:"sha256"`
	AltText          string    `json:"altText"`
	Status           string    `json:"status"`
	PublicURL        string    `json:"publicUrl,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
}

type MediaRepository struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewMediaRepository(pool *pgxpool.Pool, queryTimeout time.Duration) *MediaRepository {
	return &MediaRepository{pool: pool, queryTimeout: queryTimeout}
}

func (r *MediaRepository) CreatePendingMedia(ctx context.Context, item MediaObject, actorID, requestID string) (MediaObject, error) {
	dbCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	sha, err := hex.DecodeString(item.SHA256)
	if err != nil || len(sha) != 32 {
		return MediaObject{}, errors.New("invalid media checksum")
	}
	tx, err := r.pool.BeginTx(dbCtx, pgx.TxOptions{})
	if err != nil {
		return MediaObject{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	err = tx.QueryRow(dbCtx, `INSERT INTO media_objects(id,object_key,original_filename,mime_type,byte_size,sha256,alt_text,status,created_by)
		VALUES($1,$2,$3,$4,$5,$6,$7,'pending',$8)
		RETURNING id::text,object_key,original_filename,mime_type,byte_size,encode(sha256,'hex'),alt_text,status,created_at`, item.ID, item.ObjectKey,
		item.OriginalFilename, item.MimeType, item.ByteSize, sha, item.AltText, actorID).
		Scan(&item.ID, &item.ObjectKey, &item.OriginalFilename, &item.MimeType, &item.ByteSize, &item.SHA256, &item.AltText, &item.Status, &item.CreatedAt)
	if err != nil {
		return MediaObject{}, err
	}
	metadata, _ := json.Marshal(map[string]any{"objectKey": item.ObjectKey, "mimeType": item.MimeType, "byteSize": item.ByteSize, "sha256": item.SHA256})
	if _, err = tx.Exec(dbCtx, `INSERT INTO audit_logs(actor_user_id,action,entity_type,entity_id,request_id,metadata)
		VALUES($1,'media.upload_intent_created','media_object',$2,$3,$4::jsonb)`, actorID, item.ID, requestID, string(metadata)); err != nil {
		return MediaObject{}, err
	}
	if err = tx.Commit(dbCtx); err != nil {
		return MediaObject{}, err
	}
	return item, nil
}

func (r *MediaRepository) GetMediaForAdmin(ctx context.Context, id, actorID string) (MediaObject, error) {
	dbCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	var item MediaObject
	err := r.pool.QueryRow(dbCtx, `SELECT id::text,object_key,original_filename,mime_type,byte_size,encode(sha256,'hex'),alt_text,status,created_at
		FROM media_objects WHERE id=$1 AND created_by=$2`, id, actorID).
		Scan(&item.ID, &item.ObjectKey, &item.OriginalFilename, &item.MimeType, &item.ByteSize, &item.SHA256, &item.AltText, &item.Status, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return MediaObject{}, ErrMediaNotFound
	}
	return item, err
}

func (r *MediaRepository) ApproveMedia(ctx context.Context, id, actorID, requestID, publicURL string) (MediaObject, error) {
	dbCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	tx, err := r.pool.BeginTx(dbCtx, pgx.TxOptions{})
	if err != nil {
		return MediaObject{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var item MediaObject
	err = tx.QueryRow(dbCtx, `SELECT id::text,object_key,original_filename,mime_type,byte_size,encode(sha256,'hex'),alt_text,status,created_at
		FROM media_objects WHERE id=$1 AND created_by=$2 FOR UPDATE`, id, actorID).
		Scan(&item.ID, &item.ObjectKey, &item.OriginalFilename, &item.MimeType, &item.ByteSize, &item.SHA256, &item.AltText, &item.Status, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return MediaObject{}, ErrMediaNotFound
	}
	if err != nil {
		return MediaObject{}, err
	}
	if item.Status == "approved" {
		item.PublicURL = publicURL
		if err := tx.Commit(dbCtx); err != nil {
			return MediaObject{}, err
		}
		return item, nil
	}
	if item.Status != "pending" {
		return MediaObject{}, ErrMediaNotFound
	}
	if err := tx.QueryRow(dbCtx, `UPDATE media_objects SET status='approved' WHERE id=$1 AND status='pending'
		RETURNING status`, id).Scan(&item.Status); err != nil {
		return MediaObject{}, err
	}
	metadata, _ := json.Marshal(map[string]any{"objectKey": item.ObjectKey, "publicUrl": publicURL, "sha256": item.SHA256, "byteSize": item.ByteSize})
	if _, err := tx.Exec(dbCtx, `INSERT INTO audit_logs(actor_user_id,action,entity_type,entity_id,request_id,metadata)
		VALUES($1,'media.approved','media_object',$2,$3,$4::jsonb)`, actorID, item.ID, requestID, string(metadata)); err != nil {
		return MediaObject{}, err
	}
	if err := tx.Commit(dbCtx); err != nil {
		return MediaObject{}, err
	}
	item.PublicURL = publicURL
	return item, nil
}
