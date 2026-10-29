package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/latentsig/aramya-fashion/backend/internal/admin"
	"github.com/latentsig/aramya-fashion/backend/internal/auth"
)

type studioAdminRecorder struct {
	appointmentCalls int
	customCalls      int
	filter           admin.ListFilter
}

func (r *studioAdminRecorder) ListAppointments(_ context.Context, f admin.ListFilter) ([]admin.Appointment, error) {
	r.filter = f
	return []admin.Appointment{}, nil
}
func (r *studioAdminRecorder) ListCustomOrders(_ context.Context, f admin.ListFilter) ([]admin.CustomOrderRequest, error) {
	r.filter = f
	return []admin.CustomOrderRequest{}, nil
}
func (r *studioAdminRecorder) UpdateAppointment(_ context.Context, _, _, status, _, _ string) (admin.Appointment, error) {
	r.appointmentCalls++
	return admin.Appointment{ID: "2b4172b4-2897-4aaa-aa58-9196176f6f03", Status: status, UpdatedAt: time.Now()}, nil
}
func (r *studioAdminRecorder) UpdateCustomOrder(_ context.Context, _, _, status string, _ *int64, _, _ string) (admin.CustomOrderRequest, error) {
	r.customCalls++
	return admin.CustomOrderRequest{ID: "709991d6-a2be-4916-a4ce-7a79a18387a6", Status: status}, nil
}

type adminIdentityRecorder struct {
	authenticated bool
	allowed       bool
}

func (i adminIdentityRecorder) Authenticate(_ context.Context, token string) (auth.AuthenticatedSession, error) {
	if !i.authenticated || token != "session-token" {
		return auth.AuthenticatedSession{}, auth.ErrUnauthenticated
	}
	return auth.AuthenticatedSession{User: auth.User{ID: "b82d61bc-d4b2-4c83-9aa0-bf97b60b41d0"}}, nil
}
func (i adminIdentityRecorder) HasRole(context.Context, string, string) (bool, error) {
	return i.allowed, nil
}

func adminRequest(router http.Handler, method, path, body string, withSession bool) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if withSession {
		request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-token"})
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestAdminStudioRoutesRequireRoleBeforeReadingRequests(t *testing.T) {
	recorder := &studioAdminRecorder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, test := range []struct {
		name     string
		identity AdminIdentity
		session  bool
		want     int
	}{
		{name: "anonymous", identity: adminIdentityRecorder{authenticated: false, allowed: true}, want: http.StatusUnauthorized},
		{name: "customer", identity: adminIdentityRecorder{authenticated: true, allowed: false}, session: true, want: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := NewRouter(RouterConfig{AdminStudio: recorder, AdminIdentity: test.identity, Logger: logger})
			response := adminRequest(router, http.MethodGet, "/api/v1/admin/appointments", "", test.session)
			if response.Code != test.want {
				t.Fatalf("status=%d body=%s, want %d", response.Code, response.Body.String(), test.want)
			}
		})
	}
}

func TestAdminStudioListUsesBoundedPaginationAndFilters(t *testing.T) {
	recorder := &studioAdminRecorder{}
	identity := adminIdentityRecorder{authenticated: true, allowed: true}
	router := NewRouter(RouterConfig{AdminStudio: recorder, AdminIdentity: identity})
	response := adminRequest(router, http.MethodGet, "/api/v1/admin/appointments?status=requested&search=Shubha&limit=7&offset=21", "", true)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if recorder.filter.Status != "requested" || recorder.filter.Search != "Shubha" || recorder.filter.Limit != 8 || recorder.filter.Offset != 21 {
		t.Fatalf("repository filter=%+v", recorder.filter)
	}
	response = adminRequest(router, http.MethodGet, "/api/v1/admin/appointments?limit=1000", "", true)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unbounded limit status=%d", response.Code)
	}
}

func TestAdminStudioPatchRejectsInvalidTransitionAndRequiresQuote(t *testing.T) {
	recorder := &studioAdminRecorder{}
	identity := adminIdentityRecorder{authenticated: true, allowed: true}
	router := NewRouter(RouterConfig{AdminStudio: recorder, AdminIdentity: identity})
	id := "709991d6-a2be-4916-a4ce-7a79a18387a6"
	response := adminRequest(router, http.MethodPatch, "/api/v1/admin/custom-order-requests/"+id, `{"expectedStatus":"awaiting_approval","status":"approved"}`, true)
	if response.Code != http.StatusBadRequest || recorder.customCalls != 0 {
		t.Fatalf("approval bypass status=%d calls=%d", response.Code, recorder.customCalls)
	}
	response = adminRequest(router, http.MethodPatch, "/api/v1/admin/custom-order-requests/"+id, `{"expectedStatus":"reviewing","status":"quoted"}`, true)
	if response.Code != http.StatusBadRequest || recorder.customCalls != 0 {
		t.Fatalf("missing quote status=%d calls=%d", response.Code, recorder.customCalls)
	}
	response = adminRequest(router, http.MethodPatch, "/api/v1/admin/custom-order-requests/"+id, `{"expectedStatus":"reviewing","status":"quoted","quotationPaise":825000}`, true)
	if response.Code != http.StatusOK || recorder.customCalls != 1 {
		t.Fatalf("valid quote status=%d calls=%d body=%s", response.Code, recorder.customCalls, response.Body.String())
	}
	response = adminRequest(router, http.MethodPatch, "/api/v1/admin/appointments/not-a-uuid", `{}`, true)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid UUID status=%d", response.Code)
	}
}
