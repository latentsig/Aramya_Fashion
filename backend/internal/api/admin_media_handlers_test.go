package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/latentsig/aramya-fashion/backend/internal/admin"
	"github.com/latentsig/aramya-fashion/backend/internal/media"
)

type mediaAdminRecorder struct {
	actor, request, completedID string
	creates, completes          int
}

func (r *mediaAdminRecorder) CreateUploadIntent(_ context.Context, in media.UploadRequest, actor, request string) (media.UploadIntent, error) {
	r.creates++
	r.actor = actor
	r.request = request
	return media.UploadIntent{MediaObject: admin.MediaObject{ID: "ce62e45e-c891-4c5a-b9b4-847fc7ba92d9", ObjectKey: "products/sarees/test.jpg", Status: "pending"}, UploadURL: "https://r2.example.test/signed", ExpiresAt: time.Now().Add(time.Minute), Headers: map[string]string{"Content-Type": in.MimeType}}, nil
}
func (r *mediaAdminRecorder) CompleteUpload(_ context.Context, id, actor, request string) (admin.MediaObject, error) {
	r.completes++
	r.completedID = id
	r.actor = actor
	r.request = request
	return admin.MediaObject{ID: id, Status: "approved", PublicURL: "https://media.example.test/products/sarees/test.jpg"}, nil
}

func TestAdminMediaRoutesRequireRoleAndConfiguration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	body := `{"originalFilename":"dress.jpg","mimeType":"image/jpeg","byteSize":1024,"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","altText":"Dress","category":"sarees"}`
	for _, test := range []struct {
		name     string
		identity AdminIdentity
		session  bool
		service  AdminMediaService
		want     int
	}{
		{name: "anonymous", identity: adminIdentityRecorder{allowed: true}, want: http.StatusUnauthorized},
		{name: "customer", identity: adminIdentityRecorder{authenticated: true}, session: true, want: http.StatusForbidden},
		{name: "not configured", identity: adminIdentityRecorder{authenticated: true, allowed: true}, session: true, want: http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := NewRouter(RouterConfig{AdminIdentity: test.identity, AdminMedia: test.service, Logger: logger})
			response := adminRequest(router, http.MethodPost, "/api/v1/admin/media/upload-intent", body, test.session)
			if response.Code != test.want {
				t.Fatalf("status=%d body=%s want=%d", response.Code, response.Body.String(), test.want)
			}
		})
	}
}

func TestAdminMediaUploadAndCompletionRoutes(t *testing.T) {
	recorder := &mediaAdminRecorder{}
	router := NewRouter(RouterConfig{AdminIdentity: adminIdentityRecorder{authenticated: true, allowed: true}, AdminMedia: recorder})
	body := `{"originalFilename":"dress.jpg","mimeType":"image/jpeg","byteSize":1024,"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","altText":"Dress","category":"sarees"}`
	response := adminRequest(router, http.MethodPost, "/api/v1/admin/media/upload-intent", body, true)
	if response.Code != http.StatusCreated || recorder.creates != 1 || recorder.actor != "b82d61bc-d4b2-4c83-9aa0-bf97b60b41d0" {
		t.Fatalf("status=%d calls=%d body=%s", response.Code, recorder.creates, response.Body.String())
	}
	response = adminRequest(router, http.MethodPost, "/api/v1/admin/media/complete", `{"intentId":"ce62e45e-c891-4c5a-b9b4-847fc7ba92d9"}`, true)
	if response.Code != http.StatusOK || recorder.completes != 1 || recorder.completedID != "ce62e45e-c891-4c5a-b9b4-847fc7ba92d9" {
		t.Fatalf("status=%d calls=%d body=%s", response.Code, recorder.completes, response.Body.String())
	}
	response = adminRequest(router, http.MethodPost, "/api/v1/admin/media/complete", `{"intentId":"invalid"}`, true)
	if response.Code != http.StatusBadRequest || recorder.completes != 1 {
		t.Fatalf("invalid id status=%d calls=%d", response.Code, recorder.completes)
	}
}
