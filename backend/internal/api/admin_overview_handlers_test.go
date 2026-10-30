package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/latentsig/aramya-fashion/backend/internal/admin"
)

type adminOverviewRecorder struct {
	called bool
	result admin.Overview
}

func (r *adminOverviewRecorder) Get(context.Context) (admin.Overview, error) {
	r.called = true
	return r.result, nil
}

func TestAdminOverviewRequiresAdminAndReportsCapturedSales(t *testing.T) {
	reader := &adminOverviewRecorder{result: admin.Overview{
		ActiveProducts: 39, TotalOrders: 12, PendingPaymentOrders: 4,
		GrossCapturedPaise: 890000, ProcessedRefundsPaise: 10000, NetSalesPaise: 880000, CapturedPayments: 3,
		RecentOrders: []admin.OverviewOrder{{OrderNumber: "AR-TEST", Status: "paid", PaymentStatus: "captured", Currency: "INR", TotalPaise: 890000, CreatedAt: time.Now()}},
	}}
	identity := adminIdentityRecorder{authenticated: true, allowed: true}
	router := NewRouter(RouterConfig{AdminOverview: reader, AdminIdentity: identity})

	response := adminRequest(router, http.MethodGet, "/api/v1/admin/overview", "", false)
	if response.Code != http.StatusUnauthorized || reader.called {
		t.Fatalf("anonymous overview status=%d called=%t body=%s", response.Code, reader.called, response.Body.String())
	}

	response = adminRequest(router, http.MethodGet, "/api/v1/admin/overview", "", true)
	if response.Code != http.StatusOK || !reader.called {
		t.Fatalf("admin overview status=%d called=%t body=%s", response.Code, reader.called, response.Body.String())
	}
	var payload struct {
		Data admin.Overview `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.GrossCapturedPaise != 890000 || payload.Data.ProcessedRefundsPaise != 10000 || payload.Data.NetSalesPaise != 880000 || payload.Data.CapturedPayments != 3 || len(payload.Data.RecentOrders) != 1 {
		t.Fatalf("overview payload=%+v", payload.Data)
	}
}

func TestAdminOverviewRejectsCustomerRole(t *testing.T) {
	reader := &adminOverviewRecorder{}
	identity := adminIdentityRecorder{authenticated: true, allowed: false}
	router := NewRouter(RouterConfig{AdminOverview: reader, AdminIdentity: identity})
	response := adminRequest(router, http.MethodGet, "/api/v1/admin/overview", "", true)
	if response.Code != http.StatusForbidden || reader.called {
		t.Fatalf("customer overview status=%d called=%t body=%s", response.Code, reader.called, response.Body.String())
	}
}
