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

type AdminInventoryRepository interface {
	ListInventory(context.Context, admin.ListFilter) ([]admin.InventoryItem, error)
	SetOnHand(context.Context, string, int, int, string, string, string) (admin.InventoryItem, error)
}

func registerAdminInventoryRoutes(v1 *gin.RouterGroup, repository AdminInventoryRepository, identity AdminIdentity, logger *slog.Logger) {
	group := v1.Group("/admin")
	group.Use(requireAdminSession(identity, logger))
	group.GET("/inventory", func(c *gin.Context) {
		limit, err := parseBoundedInt(c.DefaultQuery("limit", "20"), 20, 1, 50)
		if err != nil {
			writeValidationError(c, "Limit must be between 1 and 50")
			return
		}
		offset, err := parseBoundedInt(c.DefaultQuery("offset", "0"), 0, 0, 100000)
		if err != nil {
			writeValidationError(c, "Offset must be between 0 and 100000")
			return
		}
		search := strings.TrimSpace(c.Query("search"))
		if !utf8.ValidString(search) || len(search) > 100 {
			writeValidationError(c, "Search must be at most 100 characters")
			return
		}
		items, err := repository.ListInventory(c.Request.Context(), admin.ListFilter{Search: search, Limit: limit + 1, Offset: offset})
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
	group.PATCH("/inventory/:variantID", func(c *gin.Context) {
		if !uuidPattern.MatchString(c.Param("variantID")) {
			writeValidationError(c, "Product variant ID is invalid")
			return
		}
		var request struct {
			ExpectedOnHand int    `json:"expectedOnHand"`
			OnHand         int    `json:"onHand"`
			Reason         string `json:"reason"`
		}
		if err := bindLimitedJSON(c, &request); err != nil || admin.ValidateInventoryQuantity(request.ExpectedOnHand) != nil || admin.ValidateInventoryQuantity(request.OnHand) != nil || len(strings.TrimSpace(request.Reason)) < 3 || len(request.Reason) > 500 || !utf8.ValidString(request.Reason) {
			writeValidationError(c, "Provide valid stock quantities and an adjustment reason between 3 and 500 characters")
			return
		}
		item, err := repository.SetOnHand(c.Request.Context(), c.Param("variantID"), request.ExpectedOnHand, request.OnHand, request.Reason, c.GetString("adminActorID"), c.GetString("requestID"))
		if writeInventoryAdminError(c, logger, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": item})
	})
}

func writeInventoryAdminError(c *gin.Context, logger *slog.Logger, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, admin.ErrInventoryNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "inventory_variant_not_found", "message": "Product variant was not found"}})
	case errors.Is(err, admin.ErrInventoryPolicy):
		writeValidationError(c, "Only products configured with tracked inventory can be adjusted")
	case errors.Is(err, admin.ErrInventoryQuantity):
		writeValidationError(c, "Inventory quantities must be between 0 and 1000000")
	case errors.Is(err, admin.ErrInventoryReserved):
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "inventory_reserved", "message": "On-hand quantity cannot be lower than units reserved for open orders"}})
	case errors.Is(err, admin.ErrInventoryStale):
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "inventory_changed", "message": "Stock changed since this page loaded. Refresh inventory and retry"}})
	case errors.Is(err, admin.ErrInventoryReason):
		writeValidationError(c, "An adjustment reason is required")
	default:
		writeInternalError(c, logger, err)
	}
	return true
}
