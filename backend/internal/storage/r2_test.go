package storage

import (
	"net/url"
	"strings"
	"testing"
)

func TestPresignPutBindsObjectTypeAndLengthWithoutCallingR2(t *testing.T) {
	store, err := NewR2Store(R2Config{
		AccountID: "0123456789abcdef0123456789abcdef", AccessKeyID: "TESTACCESSKEY123456", SecretAccessKey: "test-only-secret-access-key-value-000000000000",
		UploadBucket: "aramya-staging", ApprovedBucket: "aramya-approved", PublicBaseURL: "https://media.example.test",
	})
	if err != nil {
		t.Fatal(err)
	}
	intent, err := store.PresignPut(t.Context(), "products/sarees/01234567-89ab-cdef-0123-456789abcdef.jpg", "image/jpeg", 125_000)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(intent.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(parsed.Host, "0123456789abcdef0123456789abcdef.r2.cloudflarestorage.com") || parsed.Query().Get("X-Amz-Algorithm") != "AWS4-HMAC-SHA256" || parsed.Query().Get("X-Amz-Expires") != "300" {
		t.Fatalf("unexpected R2 presigned URL metadata: host=%q algorithm=%q expires=%q", parsed.Host, parsed.Query().Get("X-Amz-Algorithm"), parsed.Query().Get("X-Amz-Expires"))
	}
	if !strings.Contains(parsed.Query().Get("X-Amz-SignedHeaders"), "content-type") {
		t.Fatalf("Content-Type is not signed: %q", parsed.Query().Get("X-Amz-SignedHeaders"))
	}
	if !strings.Contains(parsed.Query().Get("X-Amz-SignedHeaders"), "content-length") {
		t.Fatalf("Content-Length is not signed to the declared byte size: %q", parsed.Query().Get("X-Amz-SignedHeaders"))
	}
	if intent.RequiredHeaders["Content-Type"] != "image/jpeg" || intent.ObjectKey != "products/sarees/01234567-89ab-cdef-0123-456789abcdef.jpg" {
		t.Fatalf("unexpected upload intent: %+v", intent)
	}
}

func TestPresignPutRejectsUntrustedKeyTypeAndOversize(t *testing.T) {
	store, err := NewR2Store(R2Config{
		AccountID: "0123456789abcdef0123456789abcdef", AccessKeyID: "TESTACCESSKEY123456", SecretAccessKey: "test-only-secret-access-key-value-000000000000",
		UploadBucket: "aramya-staging", ApprovedBucket: "aramya-approved", PublicBaseURL: "https://media.example.test",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		key, mime string
		size      int64
	}{
		{"../../other-bucket/key.jpg", "image/jpeg", 10},
		{"products/sarees/file.svg", "image/svg+xml", 10},
		{"products/sarees/file.jpg", "image/jpeg", MaxImageBytes + 1},
	} {
		if _, err := store.PresignPut(t.Context(), test.key, test.mime, test.size); err != ErrInvalidUpload {
			t.Errorf("PresignPut(%q, %q, %d) err=%v, want ErrInvalidUpload", test.key, test.mime, test.size, err)
		}
	}
}

func TestImageSignatureValidation(t *testing.T) {
	if !imageSignatureMatches([]byte("\xff\xd8\xff\xe0jpeg-data"), "image/jpeg") {
		t.Fatal("valid JPEG signature was rejected")
	}
	if !imageSignatureMatches([]byte("\x89PNG\r\n\x1a\nrest"), "image/png") {
		t.Fatal("valid PNG signature was rejected")
	}
	if !imageSignatureMatches([]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), "image/webp") {
		t.Fatal("valid WebP signature was rejected")
	}
	if !imageSignatureMatches([]byte("xxxxftypavifmore"), "image/avif") {
		t.Fatal("valid AVIF brand box was rejected")
	}
	if imageSignatureMatches([]byte("<svg onload=alert(1)></svg>"), "image/png") {
		t.Fatal("SVG data must not pass as a raster image")
	}
}

func TestObjectKeyAndBucketValidation(t *testing.T) {
	for _, key := range []string{"products/sarees/item.jpg", "brand/logo.png", "collections/home.webp"} {
		if !validObjectKey(key) {
			t.Errorf("valid key rejected: %q", key)
		}
	}
	for _, key := range []string{"../item.jpg", "products/../item.jpg", "products//item.jpg", "products/sarees/a?b.jpg", "uploads/item.jpg"} {
		if validObjectKey(key) {
			t.Errorf("invalid key accepted: %q", key)
		}
	}
	if !validBucketName("aramya-media-staging") || validBucketName("Aramya Bucket") || validBucketName("ab") {
		t.Fatal("bucket name validation failed")
	}
}
