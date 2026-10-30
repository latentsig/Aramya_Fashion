package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

const (
	MaxImageBytes        int64 = 10 << 20
	UploadURLTTL               = 5 * time.Minute
	ApprovedCacheControl       = "public, max-age=31536000, immutable"
)

var (
	ErrInvalidUpload  = errors.New("media upload metadata is invalid")
	ErrObjectMismatch = errors.New("uploaded object does not match its approved metadata")
	ErrUploadNotReady = errors.New("media upload object is not present in staging storage")
)

type R2Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	UploadBucket    string
	ApprovedBucket  string
	PublicBaseURL   string
}

type UploadExpectation struct {
	ObjectKey string
	MimeType  string
	ByteSize  int64
	SHA256    string
}

type UploadIntent struct {
	URL             string            `json:"uploadUrl"`
	ObjectKey       string            `json:"objectKey"`
	ExpiresAt       time.Time         `json:"expiresAt"`
	RequiredHeaders map[string]string `json:"requiredHeaders"`
}

type R2Store struct {
	client         *s3.Client
	presigner      *s3.PresignClient
	uploadBucket   string
	approvedBucket string
	publicBaseURL  string
}

func NewR2Store(cfg R2Config) (*R2Store, error) {
	if !validAccountID(cfg.AccountID) || len(cfg.AccessKeyID) < 8 || len(cfg.SecretAccessKey) < 32 ||
		!validBucketName(cfg.UploadBucket) || !validBucketName(cfg.ApprovedBucket) || cfg.UploadBucket == cfg.ApprovedBucket ||
		!validPublicBaseURL(cfg.PublicBaseURL) {
		return nil, ErrInvalidUpload
	}
	client := s3.New(s3.Options{
		Region:       "auto",
		BaseEndpoint: aws.String("https://" + cfg.AccountID + ".r2.cloudflarestorage.com"),
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
	})
	return &R2Store{client: client, presigner: s3.NewPresignClient(client), uploadBucket: cfg.UploadBucket,
		approvedBucket: cfg.ApprovedBucket, publicBaseURL: strings.TrimRight(cfg.PublicBaseURL, "/")}, nil
}

func (s *R2Store) PublicURL(key string) string { return s.publicBaseURL + "/" + key }

func (s *R2Store) PresignPut(ctx context.Context, objectKey, mimeType string, byteSize int64) (UploadIntent, error) {
	if !validObjectKey(objectKey) || !validImageMime(mimeType) || byteSize <= 0 || byteSize > MaxImageBytes {
		return UploadIntent{}, ErrInvalidUpload
	}
	result, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.uploadBucket), Key: aws.String(objectKey),
		ContentType: aws.String(mimeType), ContentLength: aws.Int64(byteSize),
	}, s3.WithPresignExpires(UploadURLTTL))
	if err != nil {
		return UploadIntent{}, fmt.Errorf("presign R2 upload: %w", err)
	}
	return UploadIntent{URL: result.URL, ObjectKey: objectKey, ExpiresAt: time.Now().UTC().Add(UploadURLTTL),
		RequiredHeaders: map[string]string{"Content-Type": mimeType}}, nil
}

// VerifyAndPromote downloads the private staging object, checks its exact size,
// SHA-256 and file signature, then copies it to the separately configured
// approved-media bucket and verifies the copied bytes before returning.
func (s *R2Store) VerifyAndPromote(ctx context.Context, expected UploadExpectation) error {
	if !validObjectKey(expected.ObjectKey) || !validImageMime(expected.MimeType) || expected.ByteSize <= 0 || expected.ByteSize > MaxImageBytes || len(expected.SHA256) != 64 {
		return ErrInvalidUpload
	}
	if _, err := hex.DecodeString(expected.SHA256); err != nil {
		return ErrInvalidUpload
	}
	if err := s.verifyObject(ctx, s.uploadBucket, expected, true); err != nil {
		return err
	}
	copySource := encodeCopySource(s.uploadBucket, expected.ObjectKey)
	_, err := s.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket: aws.String(s.approvedBucket), Key: aws.String(expected.ObjectKey), CopySource: aws.String(copySource),
		ContentType: aws.String(expected.MimeType), CacheControl: aws.String(ApprovedCacheControl),
		ContentDisposition: aws.String("inline"), MetadataDirective: types.MetadataDirectiveCopy,
	})
	if err != nil {
		return fmt.Errorf("promote verified R2 object: %w", err)
	}
	if err := s.verifyObject(ctx, s.approvedBucket, expected, false); err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = s.client.DeleteObject(cleanupCtx, &s3.DeleteObjectInput{Bucket: aws.String(s.approvedBucket), Key: aws.String(expected.ObjectKey)})
		return err
	}
	return nil
}

func (s *R2Store) DeleteStagingObject(ctx context.Context, objectKey string) error {
	if !validObjectKey(objectKey) {
		return ErrInvalidUpload
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.uploadBucket), Key: aws.String(objectKey)})
	if err != nil {
		return fmt.Errorf("delete R2 staging object: %w", err)
	}
	return nil
}

func (s *R2Store) verifyObject(ctx context.Context, bucket string, expected UploadExpectation, checkImageSignature bool) error {
	head, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(expected.ObjectKey)})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NoSuchKey" || apiErr.ErrorCode() == "NotFound") {
			return ErrUploadNotReady
		}
		return fmt.Errorf("inspect R2 object: %w", err)
	}
	if head.ContentLength == nil || *head.ContentLength != expected.ByteSize || head.ContentType == nil || *head.ContentType != expected.MimeType {
		return ErrObjectMismatch
	}
	object, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(expected.ObjectKey)})
	if err != nil {
		return fmt.Errorf("read R2 object for checksum verification: %w", err)
	}
	body, readErr := io.ReadAll(io.LimitReader(object.Body, MaxImageBytes+1))
	closeErr := object.Body.Close()
	if readErr != nil {
		return fmt.Errorf("read R2 object for checksum verification: %w", readErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close R2 object stream: %w", closeErr)
	}
	if int64(len(body)) != expected.ByteSize {
		return ErrObjectMismatch
	}
	digest := sha256.Sum256(body)
	if !strings.EqualFold(hex.EncodeToString(digest[:]), expected.SHA256) {
		return ErrObjectMismatch
	}
	if checkImageSignature && !imageSignatureMatches(body, expected.MimeType) {
		return ErrObjectMismatch
	}
	return nil
}

func imageSignatureMatches(body []byte, mimeType string) bool {
	if len(body) == 0 {
		return false
	}
	if mimeType == "image/avif" {
		return len(body) >= 12 && string(body[4:8]) == "ftyp" && (strings.HasPrefix(string(body[8:]), "avif") || strings.HasPrefix(string(body[8:]), "avis"))
	}
	detected := http.DetectContentType(body[:min(len(body), 512)])
	if mimeType == "image/jpeg" {
		return strings.HasPrefix(detected, "image/jpeg")
	}
	if mimeType == "image/png" {
		return strings.HasPrefix(detected, "image/png")
	}
	if mimeType == "image/webp" {
		return strings.HasPrefix(detected, "image/webp")
	}
	return false
}

func validImageMime(value string) bool {
	switch value {
	case "image/jpeg", "image/png", "image/webp", "image/avif":
		return true
	default:
		return false
	}
}

func ValidImageContentType(value string) bool { return validImageMime(value) }

func validObjectKey(key string) bool {
	if len(key) < 12 || len(key) > 240 || strings.Contains(key, "..") || strings.ContainsAny(key, "\\?#%") || strings.HasPrefix(key, "/") {
		return false
	}
	if !(strings.HasPrefix(key, "products/") || strings.HasPrefix(key, "brand/") || strings.HasPrefix(key, "collections/")) {
		return false
	}
	for _, part := range strings.Split(key, "/") {
		if part == "" || part == "." {
			return false
		}
	}
	return true
}

func encodeCopySource(bucket, key string) string {
	parts := strings.Split(key, "/")
	for index, part := range parts {
		parts[index] = url.PathEscape(part)
	}
	return url.PathEscape(bucket) + "/" + strings.Join(parts, "/")
}

func validAccountID(value string) bool {
	if len(value) != 32 {
		return false
	}
	for _, char := range value {
		if !(char >= '0' && char <= '9' || char >= 'a' && char <= 'f' || char >= 'A' && char <= 'F') {
			return false
		}
	}
	return true
}

func validBucketName(value string) bool {
	if len(value) < 3 || len(value) > 63 {
		return false
	}
	for _, char := range value {
		if !(char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '-' || char == '.') {
			return false
		}
	}
	return value[0] != '.' && value[0] != '-' && value[len(value)-1] != '.' && value[len(value)-1] != '-' && !strings.Contains(value, "..")
}

func validPublicBaseURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" && (parsed.Path == "" || parsed.Path == "/")
}
