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

type AdminOrderRepository interface {
	List(context.Context, admin.OrderFilter) ([]admin.AdminOrder, error)
	Get(context.Context, string) (admin.AdminOrderDetail, error)
	Update(context.Context, string, admin.OrderUpdate, string, string) (admin.AdminOrderDetail, error)
}

func registerAdminOrderRoutes(v1 *gin.RouterGroup, repository AdminOrderRepository, identity AdminIdentity, logger *slog.Logger) {
	group := v1.Group("/admin")
	group.Use(requireAdminSession(identity, logger))
	group.GET("/orders", func(c *gin.Context) {
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
		status := strings.TrimSpace(c.Query("status"))
		if status != "" && !admin.ValidOrderStatus(status) {
			writeValidationError(c, "Order status filter is invalid")
			return
		}
		search := strings.TrimSpace(c.Query("search"))
		if len(search) > 100 || !utf8.ValidString(search) {
			writeValidationError(c, "Search must be at most 100 characters")
			return
		}
		items, err := repository.List(c.Request.Context(), admin.OrderFilter{Status: status, Search: search, Limit: limit + 1, Offset: offset})
		if err != nil {
			if errors.Is(err, admin.ErrInvalidCatalog) {
				writeValidationError(c, "Order filters are invalid")
				return
			}
			writeInternalError(c, logger, err)
			return
		}
		hasMore := len(items) > limit
		if hasMore {
			items = items[:limit]
		}
		c.JSON(http.StatusOK, gin.H{"data": items, "pagination": gin.H{"limit": limit, "offset": offset, "count": len(items), "hasMore": hasMore}})
	})
	group.GET("/orders/:id", func(c *gin.Context) {
		if !uuidPattern.MatchString(c.Param("id")) {
			writeValidationError(c, "Order ID is invalid")
			return
		}
		item, err := repository.Get(c.Request.Context(), c.Param("id"))
		if writeAdminOrderError(c, logger, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": item})
	})
	group.PATCH("/orders/:id", func(c *gin.Context) {
		if !uuidPattern.MatchString(c.Param("id")) {
			writeValidationError(c, "Order ID is invalid")
			return
		}
		var request admin.OrderUpdate
		if bindLimitedJSON(c, &request) != nil || admin.ValidateOrderUpdate(request) != nil {
			writeValidationError(c, "Provide a valid next order status, current version, and reason. Shipping requires a carrier and tracking number.")
			return
		}
		item, err := repository.Update(c.Request.Context(), c.Param("id"), request, c.GetString("adminActorID"), c.GetString("requestID"))
		if writeAdminOrderError(c, logger, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": item})
	})
}

func writeAdminOrderError(c *gin.Context, logger *slog.Logger, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, admin.ErrOrderNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "order_not_found", "message": "Order was not found"}})
	case errors.Is(err, admin.ErrOrderStale):
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "order_changed", "message": "Order changed since it was loaded. Refresh and retry"}})
	case errors.Is(err, admin.ErrOrderState):
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "order_transition_not_allowed", "message": "This order cannot move to the requested state"}})
	case errors.Is(err, admin.ErrOrderPayment):
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "order_payment_pending", "message": "An unresolved payment attempt prevents cancellation"}})
	case errors.Is(err, admin.ErrInvalidCatalog):
		writeValidationError(c, "Order update fields are invalid")
	default:
		writeInternalError(c, logger, err)
	}
	return true
}
