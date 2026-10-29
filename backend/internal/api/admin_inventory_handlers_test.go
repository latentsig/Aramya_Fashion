package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/latentsig/aramya-fashion/backend/internal/admin"
)

type inventoryAdminRecorder struct {
	filter admin.ListFilter
	item   admin.InventoryItem
	args   []any
}

func (r *inventoryAdminRecorder) ListInventory(_ context.Context, filter admin.ListFilter) ([]admin.InventoryItem, error) {
	r.filter = filter
	return []admin.InventoryItem{r.item}, nil
}

func (r *inventoryAdminRecorder) SetOnHand(_ context.Context, variantID string, expected, target int, reason, actorID, requestID string) (admin.InventoryItem, error) {
	if target < r.item.Reserved {
		return admin.InventoryItem{}, admin.ErrInventoryReserved
	}
	r.args = []any{variantID, expected, target, reason, actorID, requestID}
	r.item.OnHand = target
	r.item.Available = target - r.item.Reserved
	r.item.UpdatedAt = time.Now()
	return r.item, nil
}

func TestAdminInventoryListRequiresAdminAndBoundsSearch(t *testing.T) {
	recorder := &inventoryAdminRecorder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, test := range []struct {
		name        string
		identity    AdminIdentity
		withSession bool
		want        int
	}{
		{name: "anonymous", identity: adminIdentityRecorder{}, want: http.StatusUnauthorized},
		{name: "customer", identity: adminIdentityRecorder{authenticated: true}, withSession: true, want: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := NewRouter(RouterConfig{AdminInventory: recorder, AdminIdentity: test.identity, Logger: logger})
			response := adminRequest(router, http.MethodGet, "/api/v1/admin/inventory", "", test.withSession)
			if response.Code != test.want {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
	identity := adminIdentityRecorder{authenticated: true, allowed: true}
	router := NewRouter(RouterConfig{AdminInventory: recorder, AdminIdentity: identity, Logger: logger})
	response := adminRequest(router, http.MethodGet, "/api/v1/admin/inventory?search=Silk&limit=8&offset=16", "", true)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if recorder.filter.Search != "Silk" || recorder.filter.Limit != 9 || recorder.filter.Offset != 16 {
		t.Fatalf("filter=%+v", recorder.filter)
	}
	response = adminRequest(router, http.MethodGet, "/api/v1/admin/inventory?search="+strings.Repeat("a", 101), "", true)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("oversized search status=%d", response.Code)
	}
}

func TestAdminInventoryAdjustmentValidatesAndForwardsExpectedQuantity(t *testing.T) {
	recorder := &inventoryAdminRecorder{item: admin.InventoryItem{VariantID: "2b4172b4-2897-4aaa-aa58-9196176f6f03", Reserved: 2}}
	identity := adminIdentityRecorder{authenticated: true, allowed: true}
	router := NewRouter(RouterConfig{AdminInventory: recorder, AdminIdentity: identity})
	path := "/api/v1/admin/inventory/" + recorder.item.VariantID
	response := adminRequest(router, http.MethodPatch, path, `{"expectedOnHand":5,"onHand":1,"reason":"Count correction"}`, true)
	if response.Code != http.StatusConflict || len(recorder.args) != 0 {
		t.Fatalf("reserved stock status=%d args=%v", response.Code, recorder.args)
	}
	response = adminRequest(router, http.MethodPatch, path, `{"expectedOnHand":5,"onHand":9,"reason":"Restocked after delivery"}`, true)
	if response.Code != http.StatusOK || len(recorder.args) != 6 {
		t.Fatalf("status=%d args=%v body=%s", response.Code, recorder.args, response.Body.String())
	}
	if recorder.args[1] != 5 || recorder.args[2] != 9 || recorder.args[3] != "Restocked after delivery" {
		t.Fatalf("args=%v", recorder.args)
	}
	response = adminRequest(router, http.MethodPatch, path, `{"expectedOnHand":5,"onHand":9,"reason":""}`, true)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("missing reason status=%d", response.Code)
	}
	response = adminRequest(router, http.MethodPatch, "/api/v1/admin/inventory/not-a-uuid", `{}`, true)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid variant ID status=%d", response.Code)
	}
}
