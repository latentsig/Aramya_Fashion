package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/latentsig/aramya-fashion/backend/internal/appointments"
	"github.com/latentsig/aramya-fashion/backend/internal/orders"
)

type unusedOrderReader struct{}

func (unusedOrderReader) List(context.Context, string, int, int) ([]orders.Summary, error) {
	return nil, errors.New("order repository must not be called without a customer session")
}

type studioRequestRecorder struct {
	appointment      appointments.AppointmentInput
	customOrder      appointments.CustomOrderInput
	appointmentCalls int
	customOrderCalls int
}

func (r *studioRequestRecorder) CreateAppointment(_ context.Context, _, _ string, input appointments.AppointmentInput) (appointments.CreatedRequest, bool, error) {
	r.appointment = input
	r.appointmentCalls++
	return appointments.CreatedRequest{ID: "2b4172b4-2897-4aaa-aa58-9196176f6f03", Status: "requested"}, false, nil
}

func (r *studioRequestRecorder) CreateCustomOrderRequest(_ context.Context, _, _ string, input appointments.CustomOrderInput) (appointments.CreatedRequest, bool, error) {
	r.customOrder = input
	r.customOrderCalls++
	return appointments.CreatedRequest{ID: "709991d6-a2be-4916-a4ce-7a79a18387a6", Status: "requested"}, false, nil
}

func (unusedOrderReader) Get(context.Context, string, string) (orders.Detail, error) {
	return orders.Detail{}, errors.New("order repository must not be called without a customer session")
}

func TestHealthAndReadiness(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		ready  func() error
		status int
	}{
		{name: "health", path: "/health", status: http.StatusOK},
		{name: "readiness rejects missing database check", path: "/ready", status: http.StatusServiceUnavailable},
		{name: "readiness when database is reachable", path: "/ready", ready: func() error { return nil }, status: http.StatusOK},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := NewRouter(RouterConfig{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Ready: test.ready})
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			router.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
			if response.Header().Get("X-Request-ID") == "" {
				t.Fatal("expected request ID header")
			}
		})
	}
}

func TestCorsAllowlist(t *testing.T) {
	router := NewRouter(RouterConfig{AllowedOrigins: []string{"https://shop.example"}})
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set("Origin", "https://shop.example")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "https://shop.example" {
		t.Fatalf("allow origin = %q", got)
	}
	if response.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatal("credentialed requests should be enabled for allowlisted origins")
	}
	request = httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set("Origin", "https://untrusted.example")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("untrusted origin was allowed: %q", got)
	}
}

func TestBoundedInteger(t *testing.T) {
	for _, value := range []string{"0", "101", "abc"} {
		if _, err := parseBoundedInt(value, 24, 1, 100); err == nil {
			t.Errorf("parseBoundedInt(%q) unexpectedly succeeded", value)
		}
	}
	got, err := parseBoundedInt("12", 24, 1, 100)
	if err != nil || got != 12 {
		t.Fatalf("parseBoundedInt(12) = %d, %v", got, err)
	}
}

func TestOrderRoutesRequireCustomerSession(t *testing.T) {
	router := NewRouter(RouterConfig{Orders: unusedOrderReader{}})
	for _, path := range []string{"/api/v1/orders", "/api/v1/orders/123e4567-e89b-12d3-a456-426614174000"} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Errorf("GET %s status = %d, want %d", path, response.Code, http.StatusUnauthorized)
		}
	}
}

func TestStudioRequestsAcceptGuestAppointmentAndCustomOrder(t *testing.T) {
	recorder := &studioRequestRecorder{}
	router := NewRouter(RouterConfig{StudioRequests: recorder})
	tests := []struct {
		path   string
		body   string
		wantID string
	}{
		{path: "/api/v1/appointments", body: `{"name":"Studio Guest","email":"guest@example.com","interest":"Bridal or wedding wear","note":"Wedding in December"}`, wantID: "2b4172b4-2897-4aaa-aa58-9196176f6f03"},
		{path: "/api/v1/custom-order-requests", body: `{"name":"Studio Guest","phone":"+91 98765 43210","garmentType":"Lehenga","occasion":"Wedding","requirements":"Discuss a custom fit"}`, wantID: "709991d6-a2be-4916-a4ce-7a79a18387a6"},
	}
	for _, test := range tests {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Idempotency-Key", "local-test-request-0001")
		router.ServeHTTP(response, request)
		if response.Code != http.StatusCreated {
			t.Fatalf("POST %s status = %d, body=%s", test.path, response.Code, response.Body.String())
		}
		var payload struct {
			Data appointments.CreatedRequest `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode POST %s response: %v", test.path, err)
		}
		if payload.Data.ID != test.wantID || payload.Data.Status != "requested" {
			t.Errorf("POST %s result = %+v", test.path, payload.Data)
		}
	}
	if recorder.appointmentCalls != 1 || recorder.appointment.Email != "guest@example.com" {
		t.Errorf("appointment repository call = %d, input=%+v", recorder.appointmentCalls, recorder.appointment)
	}
	if recorder.customOrderCalls != 1 || recorder.customOrder.Phone != "+91 98765 43210" {
		t.Errorf("custom order repository call = %d, input=%+v", recorder.customOrderCalls, recorder.customOrder)
	}
}

func TestStudioRequestsRejectInvalidContactBeforePersistence(t *testing.T) {
	recorder := &studioRequestRecorder{}
	router := NewRouter(RouterConfig{StudioRequests: recorder})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/appointments", strings.NewReader(`{"name":"Studio Guest","email":"not-an-email"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "local-test-request-0001")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if recorder.appointmentCalls != 0 {
		t.Fatalf("invalid appointment reached persistence %d times", recorder.appointmentCalls)
	}
}
