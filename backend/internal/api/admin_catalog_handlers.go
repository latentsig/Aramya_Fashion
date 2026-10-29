package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/latentsig/aramya-fashion/backend/internal/admin"
)

type AdminCatalogRepository interface {
	ListCategories(context.Context) ([]admin.AdminCategory, error)
	ListProducts(context.Context, admin.CatalogFilter) ([]admin.AdminProductSummary, error)
	GetProduct(context.Context, string) (admin.AdminProductDetail, error)
	ListImageOptions(context.Context) ([]admin.ImageOption, error)
	CreateCategory(context.Context, admin.CategoryWrite, string, string) (admin.AdminCategory, error)
	UpdateCategory(context.Context, string, admin.CategoryWrite, string, string) (admin.AdminCategory, error)
	CreateProduct(context.Context, admin.ProductWrite, string, string) (admin.AdminProductDetail, error)
	UpdateProduct(context.Context, string, admin.ProductWrite, string, string) (admin.AdminProductDetail, error)
}

func registerAdminCatalogRoutes(v1 *gin.RouterGroup, repository AdminCatalogRepository, identity AdminIdentity, logger *slog.Logger) {
	group := v1.Group("/admin")
	group.Use(requireAdminSession(identity, logger))
	group.GET("/categories", func(c *gin.Context) {
		items, err := repository.ListCategories(c.Request.Context())
		if err != nil {
			writeInternalError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": items})
	})
	group.POST("/categories", func(c *gin.Context) {
		var request admin.CategoryWrite
		if bindLimitedJSON(c, &request) != nil {
			writeCatalogValidation(c)
			return
		}
		request = admin.NormalizeCategoryWrite(request)
		if admin.ValidateCategoryWrite(request, true) != nil {
			writeCatalogValidation(c)
			return
		}
		item, err := repository.CreateCategory(c.Request.Context(), request, c.GetString("adminActorID"), c.GetString("requestID"))
		if writeAdminCatalogError(c, logger, err) {
			return
		}
		c.JSON(http.StatusCreated, gin.H{"data": item})
	})
	group.PATCH("/categories/:id", func(c *gin.Context) {
		if !uuidPattern.MatchString(c.Param("id")) {
			writeCatalogValidation(c)
			return
		}
		var request admin.CategoryWrite
		if bindLimitedJSON(c, &request) != nil {
			writeCatalogValidation(c)
			return
		}
		request = admin.NormalizeCategoryWrite(request)
		if admin.ValidateCategoryWrite(request, false) != nil {
			writeCatalogValidation(c)
			return
		}
		item, err := repository.UpdateCategory(c.Request.Context(), c.Param("id"), request, c.GetString("adminActorID"), c.GetString("requestID"))
		if writeAdminCatalogError(c, logger, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": item})
	})
	group.GET("/catalog/image-options", func(c *gin.Context) {
		items, err := repository.ListImageOptions(c.Request.Context())
		if err != nil {
			writeInternalError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": items})
	})
	group.GET("/products", func(c *gin.Context) {
		limit, err := parseBoundedInt(c.DefaultQuery("limit", "20"), 20, 1, 50)
		if err != nil {
			writeCatalogValidation(c)
			return
		}
		offset, err := parseBoundedInt(c.DefaultQuery("offset", "0"), 0, 0, 100000)
		if err != nil {
			writeCatalogValidation(c)
			return
		}
		status := strings.TrimSpace(c.Query("status"))
		if status != "" && status != "draft" && status != "active" && status != "archived" {
			writeCatalogValidation(c)
			return
		}
		categoryID := strings.TrimSpace(c.Query("categoryId"))
		if categoryID != "" && !uuidPattern.MatchString(categoryID) {
			writeCatalogValidation(c)
			return
		}
		search := strings.TrimSpace(c.Query("search"))
		if !utf8.ValidString(search) || len(search) > 100 {
			writeCatalogValidation(c)
			return
		}
		items, err := repository.ListProducts(c.Request.Context(), admin.CatalogFilter{Search: search, Status: status, CategoryID: categoryID, Limit: limit + 1, Offset: offset})
		if err != nil {
			writeInternalError(c, logger, err)
			return
		}
		hasMore := len(items) > limit
		if hasMore {
			items = items[:limit]
		}
		c.JSON(http.StatusOK, gin.H{"data": items, "pagination": gin.H{"limit": limit, "offset": offset, "count": len(items), "hasMore": hasMore}})
	})
	group.GET("/products/:id", func(c *gin.Context) {
		if !uuidPattern.MatchString(c.Param("id")) {
			writeCatalogValidation(c)
			return
		}
		item, err := repository.GetProduct(c.Request.Context(), c.Param("id"))
		if writeAdminCatalogError(c, logger, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": item})
	})
	group.POST("/products", func(c *gin.Context) {
		var request admin.ProductWrite
		if bindLimitedJSON(c, &request) != nil {
			writeCatalogValidation(c)
			return
		}
		request = admin.NormalizeProductWrite(request)
		if admin.ValidateProductWrite(request, true) != nil {
			writeCatalogValidation(c)
			return
		}
		item, err := repository.CreateProduct(c.Request.Context(), request, c.GetString("adminActorID"), c.GetString("requestID"))
		if writeAdminCatalogError(c, logger, err) {
			return
		}
		c.JSON(http.StatusCreated, gin.H{"data": item})
	})
	group.PATCH("/products/:id", func(c *gin.Context) {
		if !uuidPattern.MatchString(c.Param("id")) {
			writeCatalogValidation(c)
			return
		}
		var request admin.ProductWrite
		if bindLimitedJSON(c, &request) != nil {
			writeCatalogValidation(c)
			return
		}
		request = admin.NormalizeProductWrite(request)
		if admin.ValidateProductWrite(request, false) != nil {
			writeCatalogValidation(c)
			return
		}
		item, err := repository.UpdateProduct(c.Request.Context(), c.Param("id"), request, c.GetString("adminActorID"), c.GetString("requestID"))
		if writeAdminCatalogError(c, logger, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": item})
	})
}

func writeCatalogValidation(c *gin.Context) {
	c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_catalog_input", "message": "Review the product or category details and try again"}})
}

func writeAdminCatalogError(c *gin.Context, logger *slog.Logger, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, admin.ErrCatalogNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "catalog_record_not_found", "message": "Product or category was not found"}})
	case errors.Is(err, admin.ErrCatalogStale):
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "catalog_record_changed", "message": "This record changed while you were editing it. Reload and try again"}})
	case errors.Is(err, admin.ErrCatalogConflict):
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "catalog_conflict", "message": "That SKU or URL is already in use"}})
	case errors.Is(err, admin.ErrCatalogImage):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": gin.H{"code": "catalog_image_unavailable", "message": "Choose an image from the existing approved catalog assets"}})
	case errors.Is(err, admin.ErrCategoryInactive):
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "category_inactive", "message": "Activate the category before publishing a product"}})
	case errors.Is(err, admin.ErrCategoryHasProducts):
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "category_in_use", "message": "Archive or move active products before deactivating this category"}})
	case errors.Is(err, admin.ErrInvalidCatalog):
		writeCatalogValidation(c)
	default:
		writeInternalError(c, logger, err)
	}
	return true
}
