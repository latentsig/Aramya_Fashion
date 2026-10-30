package media

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/latentsig/aramya-fashion/backend/internal/admin"
	"github.com/latentsig/aramya-fashion/backend/internal/storage"
)

type fakeRepository struct {
	item               admin.MediaObject
	created, approved  int
	actor, request     string
	getErr, approveErr error
}

func (r *fakeRepository) CreatePendingMedia(_ context.Context, item admin.MediaObject, actor, request string) (admin.MediaObject, error) {
	r.created++
	r.actor, r.request = actor, request
	item.Status = "pending"
	r.item = item
	return item, nil
}
func (r *fakeRepository) GetMediaForAdmin(_ context.Context, id, actor string) (admin.MediaObject, error) {
	if r.getErr != nil {
		return admin.MediaObject{}, r.getErr
	}
	if id != r.item.ID || actor != r.actor {
		return admin.MediaObject{}, admin.ErrMediaNotFound
	}
	return r.item, nil
}
func (r *fakeRepository) ApproveMedia(_ context.Context, id, actor, request, public string) (admin.MediaObject, error) {
	r.approved++
	r.request = request
	if r.approveErr != nil {
		return admin.MediaObject{}, r.approveErr
	}
	r.item.Status = "approved"
	r.item.PublicURL = public
	return r.item, nil
}

type fakeObjectStore struct {
	intent               storage.UploadIntent
	verifyErr, deleteErr error
	verified, deleted    int
	order                *[]string
}

func (s *fakeObjectStore) PresignPut(_ context.Context, key, mime string, size int64) (storage.UploadIntent, error) {
	return storage.UploadIntent{URL: "https://r2.example.test/signed", ObjectKey: key, ExpiresAt: time.Now().Add(storage.UploadURLTTL), RequiredHeaders: map[string]string{"Content-Type": mime}}, nil
}
func (s *fakeObjectStore) VerifyAndPromote(_ context.Context, _ storage.UploadExpectation) error {
	s.verified++
	if s.order != nil {
		*s.order = append(*s.order, "verify")
	}
	return s.verifyErr
}
func (s *fakeObjectStore) DeleteStagingObject(_ context.Context, _ string) error {
	s.deleted++
	if s.order != nil {
		*s.order = append(*s.order, "delete")
	}
	return s.deleteErr
}
func (s *fakeObjectStore) PublicURL(key string) string { return "https://media.example.test/" + key }

func validUpload() UploadRequest {
	return UploadRequest{OriginalFilename: "saree.jpg", MimeType: "image/jpeg", ByteSize: 1024, SHA256: strings.Repeat("a", 64), AltText: "Saree", Category: "sarees"}
}

func TestCreateUploadIntentValidatesBeforeSigningOrPersistence(t *testing.T) {
	repo, store := &fakeRepository{}, &fakeObjectStore{}
	svc, _ := NewService(repo, store)
	for _, mutate := range []func(*UploadRequest){
		func(v *UploadRequest) { v.OriginalFilename = "bad.jpg\nimage.jpg" },
		func(v *UploadRequest) { v.MimeType = "image/svg+xml" },
		func(v *UploadRequest) { v.OriginalFilename = "bad.png" },
		func(v *UploadRequest) { v.ByteSize = storage.MaxImageBytes + 1 },
		func(v *UploadRequest) { v.SHA256 = "not-a-checksum" },
		func(v *UploadRequest) { v.Category = "unknown" },
	} {
		input := validUpload()
		mutate(&input)
		if _, err := svc.CreateUploadIntent(context.Background(), input, "actor", "request"); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("invalid upload err=%v", err)
		}
	}
	if repo.created != 0 {
		t.Fatalf("invalid requests were persisted: %d", repo.created)
	}
}

func TestCreateUploadIntentCreatesBoundedPrivateIntent(t *testing.T) {
	repo, store := &fakeRepository{}, &fakeObjectStore{}
	svc, _ := NewService(repo, store)
	input := validUpload()
	input.OriginalFilename = `C:\Users\person\bridal.jpg`
	intent, err := svc.CreateUploadIntent(context.Background(), input, "actor-123", "request-456")
	if err != nil {
		t.Fatal(err)
	}
	if repo.created != 1 || repo.actor != "actor-123" || repo.request != "request-456" || repo.item.OriginalFilename != "bridal.jpg" || repo.item.Status != "pending" {
		t.Fatalf("repository item=%+v", repo.item)
	}
	if !strings.HasPrefix(intent.MediaObject.ObjectKey, "products/sarees/") || intent.UploadURL != "https://r2.example.test/signed" || intent.ExpiresAt.After(time.Now().Add(storage.UploadURLTTL+time.Second)) || intent.Headers["Content-Type"] != "image/jpeg" {
		t.Fatalf("unexpected intent: %+v", intent)
	}
}

func TestCompleteUploadVerifiesThenApprovesThenCleansStage(t *testing.T) {
	var order []string
	repo, store := &fakeRepository{item: admin.MediaObject{ID: "media-id", ObjectKey: "products/sarees/item.jpg", MimeType: "image/jpeg", ByteSize: 1024, SHA256: strings.Repeat("a", 64), Status: "pending"}, actor: "admin"}, &fakeObjectStore{order: &order}
	svc, _ := NewService(repo, store)
	item, err := svc.CompleteUpload(context.Background(), "media-id", "admin", "request-id")
	if err != nil {
		t.Fatal(err)
	}
	if item.Status != "approved" || item.PublicURL != "https://media.example.test/products/sarees/item.jpg" || store.verified != 1 || store.deleted != 1 || repo.approved != 1 || strings.Join(order, ",") != "verify,delete" {
		t.Fatalf("item=%+v order=%v verify=%d delete=%d approve=%d", item, order, store.verified, store.deleted, repo.approved)
	}
}

func TestCompleteUploadFailureDoesNotApproveOrDelete(t *testing.T) {
	repo, store := &fakeRepository{item: admin.MediaObject{ID: "media-id", ObjectKey: "products/sarees/item.jpg", MimeType: "image/jpeg", ByteSize: 1024, SHA256: strings.Repeat("a", 64), Status: "pending"}, actor: "admin"}, &fakeObjectStore{verifyErr: storage.ErrObjectMismatch}
	svc, _ := NewService(repo, store)
	if _, err := svc.CompleteUpload(context.Background(), "media-id", "admin", "request"); !errors.Is(err, storage.ErrObjectMismatch) {
		t.Fatalf("err=%v", err)
	}
	if repo.approved != 0 || store.deleted != 0 {
		t.Fatalf("failed verification mutated state: approvals=%d deletes=%d", repo.approved, store.deleted)
	}
}

func TestCompleteApprovedUploadRetryOnlyCleansStaging(t *testing.T) {
	repo, store := &fakeRepository{item: admin.MediaObject{ID: "media-id", ObjectKey: "products/sarees/item.jpg", Status: "approved"}, actor: "admin"}, &fakeObjectStore{}
	svc, _ := NewService(repo, store)
	if _, err := svc.CompleteUpload(context.Background(), "media-id", "admin", "request"); err != nil {
		t.Fatal(err)
	}
	if repo.approved != 0 || store.verified != 0 || store.deleted != 1 {
		t.Fatalf("retry actions: approvals=%d verify=%d delete=%d", repo.approved, store.verified, store.deleted)
	}
}
