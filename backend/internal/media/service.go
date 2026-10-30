package media

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/latentsig/aramya-fashion/backend/internal/admin"
	"github.com/latentsig/aramya-fashion/backend/internal/storage"
)

var (
	ErrInvalidRequest = errors.New("media upload request is invalid")
	ErrNotFound       = admin.ErrMediaNotFound
)

type UploadRequest struct {
	OriginalFilename string `json:"originalFilename"`
	MimeType         string `json:"mimeType"`
	ByteSize         int64  `json:"byteSize"`
	SHA256           string `json:"sha256"`
	AltText          string `json:"altText"`
	Category         string `json:"category"`
}

type UploadIntent struct {
	MediaObject admin.MediaObject `json:"media"`
	UploadURL   string            `json:"uploadUrl"`
	ExpiresAt   time.Time         `json:"expiresAt"`
	Headers     map[string]string `json:"requiredHeaders"`
}

type Repository interface {
	CreatePendingMedia(context.Context, admin.MediaObject, string, string) (admin.MediaObject, error)
	GetMediaForAdmin(context.Context, string, string) (admin.MediaObject, error)
	ApproveMedia(context.Context, string, string, string, string) (admin.MediaObject, error)
}

type ObjectStore interface {
	PresignPut(context.Context, string, string, int64) (storage.UploadIntent, error)
	VerifyAndPromote(context.Context, storage.UploadExpectation) error
	DeleteStagingObject(context.Context, string) error
	PublicURL(string) string
}

type Service struct {
	repository Repository
	storage    ObjectStore
}

func NewService(repository Repository, store ObjectStore) (*Service, error) {
	if repository == nil || store == nil {
		return nil, errors.New("media repository and R2 store are required")
	}
	return &Service{repository: repository, storage: store}, nil
}

func (s *Service) CreateUploadIntent(ctx context.Context, input UploadRequest, actorID, requestID string) (UploadIntent, error) {
	input.OriginalFilename = strings.TrimSpace(strings.ReplaceAll(input.OriginalFilename, "\\", "/"))
	input.AltText = strings.TrimSpace(input.AltText)
	input.Category = strings.ToLower(strings.TrimSpace(input.Category))
	input.MimeType = strings.ToLower(strings.TrimSpace(input.MimeType))
	input.SHA256 = strings.ToLower(strings.TrimSpace(input.SHA256))
	if !validUploadRequest(input) {
		return UploadIntent{}, ErrInvalidRequest
	}
	identifier, err := newUUID()
	if err != nil {
		return UploadIntent{}, err
	}
	extension := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp", "image/avif": ".avif"}[input.MimeType]
	key := mediaObjectKey(input.Category, identifier, extension)
	signed, err := s.storage.PresignPut(ctx, key, input.MimeType, input.ByteSize)
	if err != nil {
		return UploadIntent{}, err
	}
	item, err := s.repository.CreatePendingMedia(ctx, admin.MediaObject{
		ID: identifier, ObjectKey: key, OriginalFilename: path.Base(input.OriginalFilename), MimeType: input.MimeType,
		ByteSize: input.ByteSize, SHA256: input.SHA256, AltText: input.AltText,
	}, actorID, requestID)
	if err != nil {
		return UploadIntent{}, err
	}
	return UploadIntent{MediaObject: item, UploadURL: signed.URL, ExpiresAt: signed.ExpiresAt.UTC(), Headers: signed.RequiredHeaders}, nil
}

func (s *Service) CompleteUpload(ctx context.Context, intentID, actorID, requestID string) (admin.MediaObject, error) {
	item, err := s.repository.GetMediaForAdmin(ctx, intentID, actorID)
	if err != nil {
		return admin.MediaObject{}, err
	}
	if item.Status != "approved" {
		if item.Status != "pending" {
			return admin.MediaObject{}, admin.ErrMediaNotFound
		}
		if err := s.storage.VerifyAndPromote(ctx, storage.UploadExpectation{ObjectKey: item.ObjectKey, MimeType: item.MimeType, ByteSize: item.ByteSize, SHA256: item.SHA256}); err != nil {
			return admin.MediaObject{}, err
		}
		item, err = s.repository.ApproveMedia(ctx, item.ID, actorID, requestID, s.storage.PublicURL(item.ObjectKey))
		if err != nil {
			return admin.MediaObject{}, err
		}
	}
	if err := s.storage.DeleteStagingObject(ctx, item.ObjectKey); err != nil {
		return item, fmt.Errorf("media approved but private staging cleanup failed; retry completion to clean it up: %w", err)
	}
	return item, nil
}

func validUploadRequest(input UploadRequest) bool {
	if input.OriginalFilename == "" || len(input.OriginalFilename) > 240 || !utf8.ValidString(input.OriginalFilename) || strings.ContainsAny(input.OriginalFilename, "\r\n\x00") ||
		path.Base(input.OriginalFilename) == "." || input.ByteSize <= 0 || input.ByteSize > storage.MaxImageBytes ||
		len(input.AltText) > 240 || !utf8.ValidString(input.AltText) || len(input.SHA256) != 64 {
		return false
	}
	if _, err := hex.DecodeString(input.SHA256); err != nil {
		return false
	}
	if !storage.ValidImageContentType(input.MimeType) {
		return false
	}
	filenameExtension := strings.ToLower(path.Ext(input.OriginalFilename))
	switch input.MimeType {
	case "image/jpeg":
		if filenameExtension != ".jpg" && filenameExtension != ".jpeg" {
			return false
		}
	case "image/png":
		if filenameExtension != ".png" {
			return false
		}
	case "image/webp":
		if filenameExtension != ".webp" {
			return false
		}
	case "image/avif":
		if filenameExtension != ".avif" {
			return false
		}
	}
	switch input.Category {
	case "sarees", "bridal", "occasion", "menswear", "kids", "brand", "collections":
		return true
	default:
		return false
	}
}

func mediaObjectKey(category, identifier, extension string) string {
	prefix := category
	if category != "brand" && category != "collections" {
		prefix = "products/" + category
	}
	return prefix + "/" + identifier + extension
}

func newUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(value[:])
	return encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:], nil
}
