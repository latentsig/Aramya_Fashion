package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/latentsig/aramya-fashion/backend/internal/admin"
)

type orderAdminRecorder struct {
	filter      admin.OrderFilter
	update      admin.OrderUpdate
	updateCalls int
	updateErr   error
}

func (r *orderAdminRecorder) List(_ context.Context, filter admin.OrderFilter) ([]admin.AdminOrder, error) {
	r.filter = filter
	return []admin.AdminOrder{}, nil
}
func (r *orderAdminRecorder) Get(_ context.Context, id string) (admin.AdminOrderDetail, error) {
	return admin.AdminOrderDetail{AdminOrder: admin.AdminOrder{ID: id, OrderNumber: "AR-100", Status: "paid"}}, nil
}
func (r *orderAdminRecorder) Update(_ context.Context, _ string, update admin.OrderUpdate, _, _ string) (admin.AdminOrderDetail, error) {
	r.updateCalls++
	r.update = update
	if r.updateErr != nil {
		return admin.AdminOrderDetail{}, r.updateErr
	}
	return admin.AdminOrderDetail{AdminOrder: admin.AdminOrder{Status: update.Status}}, nil
}

func TestAdminOrderRoutesRequireRoleAndFilterOrders(t *testing.T) {
	recorder := &orderAdminRecorder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, test := range []struct {
		name     string
		identity AdminIdentity
		session  bool
		want     int
	}{
		{name: "anonymous", want: http.StatusUnauthorized},
		{name: "customer", identity: adminIdentityRecorder{authenticated: true}, session: true, want: http.StatusForbidden},
	} {
		router := NewRouter(RouterConfig{AdminOrders: recorder, AdminIdentity: test.identity, Logger: logger})
		response := adminRequest(router, http.MethodGet, "/api/v1/admin/orders", "", test.session)
		if response.Code != test.want {
			t.Fatalf("%s status=%d body=%s", test.name, response.Code, response.Body.String())
		}
	}
	identity := adminIdentityRecorder{authenticated: true, allowed: true}
	router := NewRouter(RouterConfig{AdminOrders: recorder, AdminIdentity: identity, Logger: logger})
	response := adminRequest(router, http.MethodGet, "/api/v1/admin/orders?status=shipped&search=AR-100&limit=8&offset=16", "", true)
	if response.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", response.Code, response.Body.String())
	}
	if recorder.filter.Status != "shipped" || recorder.filter.Search != "AR-100" || recorder.filter.Limit != 9 || recorder.filter.Offset != 16 {
		t.Fatalf("filter=%+v", recorder.filter)
	}
	response = adminRequest(router, http.MethodGet, "/api/v1/admin/orders?status=bogus", "", true)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid status filter=%d", response.Code)
	}
	response = adminRequest(router, http.MethodGet, "/api/v1/admin/orders/not-a-uuid", "", true)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid ID=%d", response.Code)
	}
}

func TestAdminOrderUpdateValidatesShipmentAndMapsTransitionConflicts(t *testing.T) {
	recorder := &orderAdminRecorder{}
	identity := adminIdentityRecorder{authenticated: true, allowed: true}
	router := NewRouter(RouterConfig{AdminOrders: recorder, AdminIdentity: identity})
	orderID := "2b4172b4-2897-4aaa-aa58-9196176f6f03"
	invalid := `{"expectedVersion":3,"status":"shipped","reason":"Parcel collected"}`
	response := adminRequest(router, http.MethodPatch, "/api/v1/admin/orders/"+orderID, invalid, true)
	if response.Code != http.StatusBadRequest || recorder.updateCalls != 0 {
		t.Fatalf("invalid shipment status=%d calls=%d", response.Code, recorder.updateCalls)
	}
	valid := `{"expectedVersion":3,"status":"shipped","reason":"Parcel collected","shipmentProvider":"Aramya Logistics","trackingNumber":"TRK-123","trackingUrl":"https://track.example.test/TRK-123"}`
	recorder.updateErr = admin.ErrOrderState
	response = adminRequest(router, http.MethodPatch, "/api/v1/admin/orders/"+orderID, valid, true)
	if response.Code != http.StatusConflict || recorder.updateCalls != 1 || recorder.update.TrackingNumber != "TRK-123" {
		t.Fatalf("transition response=%d calls=%d body=%s update=%+v", response.Code, recorder.updateCalls, response.Body.String(), recorder.update)
	}
}
