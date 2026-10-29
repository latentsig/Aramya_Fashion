package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/latentsig/aramya-fashion/backend/internal/admin"
)

type catalogAdminRecorder struct {
	filter      admin.CatalogFilter
	created     admin.ProductWrite
	createCalls int
	createError error
}

func (r *catalogAdminRecorder) ListCategories(context.Context) ([]admin.AdminCategory, error) {
	return []admin.AdminCategory{}, nil
}
func (r *catalogAdminRecorder) ListProducts(_ context.Context, filter admin.CatalogFilter) ([]admin.AdminProductSummary, error) {
	r.filter = filter
	return []admin.AdminProductSummary{}, nil
}
func (r *catalogAdminRecorder) GetProduct(context.Context, string) (admin.AdminProductDetail, error) {
	return admin.AdminProductDetail{}, admin.ErrCatalogNotFound
}
func (r *catalogAdminRecorder) ListImageOptions(context.Context) ([]admin.ImageOption, error) {
	return []admin.ImageOption{}, nil
}
func (r *catalogAdminRecorder) CreateCategory(context.Context, admin.CategoryWrite, string, string) (admin.AdminCategory, error) {
	return admin.AdminCategory{}, nil
}
func (r *catalogAdminRecorder) UpdateCategory(context.Context, string, admin.CategoryWrite, string, string) (admin.AdminCategory, error) {
	return admin.AdminCategory{}, nil
}
func (r *catalogAdminRecorder) CreateProduct(_ context.Context, input admin.ProductWrite, _, _ string) (admin.AdminProductDetail, error) {
	r.createCalls++
	r.created = input
	if r.createError != nil {
		return admin.AdminProductDetail{}, r.createError
	}
	return admin.AdminProductDetail{AdminProductSummary: admin.AdminProductSummary{ID: "2b4172b4-2897-4aaa-aa58-9196176f6f03", Version: 1, Name: input.Name, UpdatedAt: time.Now()}}, nil
}
func (r *catalogAdminRecorder) UpdateProduct(context.Context, string, admin.ProductWrite, string, string) (admin.AdminProductDetail, error) {
	return admin.AdminProductDetail{}, admin.ErrCatalogStale
}

func TestAdminCatalogRoutesRequireRoleAndBoundProductList(t *testing.T) {
	recorder := &catalogAdminRecorder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, test := range []struct {
		name     string
		identity AdminIdentity
		session  bool
		want     int
	}{
		{name: "anonymous", identity: adminIdentityRecorder{}, want: http.StatusUnauthorized},
		{name: "customer", identity: adminIdentityRecorder{authenticated: true}, session: true, want: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := NewRouter(RouterConfig{AdminCatalog: recorder, AdminIdentity: test.identity, Logger: logger})
			response := adminRequest(router, http.MethodGet, "/api/v1/admin/products", "", test.session)
			if response.Code != test.want {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
	identity := adminIdentityRecorder{authenticated: true, allowed: true}
	router := NewRouter(RouterConfig{AdminCatalog: recorder, AdminIdentity: identity, Logger: logger})
	response := adminRequest(router, http.MethodGet, "/api/v1/admin/products?status=draft&categoryId=fd5a5719-8447-4a0b-9d7b-bcf609e00ad1&search=silk&limit=7&offset=14", "", true)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if recorder.filter.Status != "draft" || recorder.filter.CategoryID != "fd5a5719-8447-4a0b-9d7b-bcf609e00ad1" || recorder.filter.Search != "silk" || recorder.filter.Limit != 8 || recorder.filter.Offset != 14 {
		t.Fatalf("filter=%+v", recorder.filter)
	}
	response = adminRequest(router, http.MethodGet, "/api/v1/admin/products?status=everything", "", true)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid status code=%d", response.Code)
	}
}

func TestAdminProductCreateValidatesVariantsAndMapsSKUConflict(t *testing.T) {
	recorder := &catalogAdminRecorder{}
	identity := adminIdentityRecorder{authenticated: true, allowed: true}
	router := NewRouter(RouterConfig{AdminCatalog: recorder, AdminIdentity: identity})
	body := `{"sku":" AR-TEST-01 ","slug":"test-saree","name":" Test Saree ","description":"","categoryId":"fd5a5719-8447-4a0b-9d7b-bcf609e00ad1","pricePaise":1250000,"imageKey":"image assests/saree1.jpg","status":"draft","inventoryPolicy":"made_to_order","variants":[{"sku":"AR-TEST-01-MTO","optionName":"Fit","optionValue":" Made to measure ","priceAdjustmentPaise":0,"active":true,"isDefault":true}]}`
	response := adminRequest(router, http.MethodPost, "/api/v1/admin/products", `{"sku":"bad"}`, true)
	if response.Code != http.StatusBadRequest || recorder.createCalls != 0 {
		t.Fatalf("invalid product status=%d calls=%d", response.Code, recorder.createCalls)
	}
	response = adminRequest(router, http.MethodPost, "/api/v1/admin/products", body, true)
	if response.Code != http.StatusCreated || recorder.createCalls != 1 {
		t.Fatalf("create status=%d calls=%d body=%s", response.Code, recorder.createCalls, response.Body.String())
	}
	if recorder.created.Name != "Test Saree" || recorder.created.Variants[0].OptionValue != "Made to measure" {
		t.Fatalf("input was not normalized: %+v", recorder.created)
	}
	recorder.createError = admin.ErrCatalogConflict
	response = adminRequest(router, http.MethodPost, "/api/v1/admin/products", body, true)
	if response.Code != http.StatusConflict {
		t.Fatalf("duplicate SKU status=%d", response.Code)
	}
}
