package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/latentsig/aramya-fashion/backend/internal/auth"
	"github.com/latentsig/aramya-fashion/backend/internal/orders"
)

type OrderReader interface {
	List(ctx context.Context, userID string, limit, offset int) ([]orders.Summary, error)
	Get(ctx context.Context, userID, orderID string) (orders.Detail, error)
}

func registerOrderRoutes(v1 *gin.RouterGroup, repository OrderReader, authService *auth.Service, logger *slog.Logger) {
	api := v1.Group("/orders")
	api.Use(requireCustomerSession(authService, logger))
	api.GET("", func(c *gin.Context) {
		limit, err := parseBoundedInt(c.DefaultQuery("limit", "20"), 20, 1, 50)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_limit", "message": "limit must be between 1 and 50"}})
			return
		}
		offset, err := parseBoundedInt(c.DefaultQuery("offset", "0"), 0, 0, 100000)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_offset", "message": "offset must be between 0 and 100000"}})
			return
		}

		items, err := repository.List(c.Request.Context(), c.GetString("customerID"), limit+1, offset)
		if err != nil {
			writeInternalError(c, logger, err)
			return
		}
		hasMore := len(items) > limit
		if hasMore {
			items = items[:limit]
		}
		c.JSON(http.StatusOK, gin.H{
			"data":       items,
			"pagination": gin.H{"limit": limit, "offset": offset, "count": len(items), "hasMore": hasMore},
		})
	})
	api.GET("/:id", func(c *gin.Context) {
		orderID := c.Param("id")
		if !uuidPattern.MatchString(orderID) {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_order_id", "message": "Order ID is invalid"}})
			return
		}
		item, err := repository.Get(c.Request.Context(), c.GetString("customerID"), orderID)
		if errors.Is(err, orders.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "order_not_found", "message": "Order not found"}})
			return
		}
		if err != nil {
			writeInternalError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": item})
	})
}

func requireCustomerSession(service *auth.Service, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(sessionCookieName)
		if err != nil || service == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "unauthenticated", "message": "Sign in to view your orders"}})
			c.Abort()
			return
		}
		session, err := service.Authenticate(c.Request.Context(), token)
		if errors.Is(err, auth.ErrUnauthenticated) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "unauthenticated", "message": "Sign in to view your orders"}})
			c.Abort()
			return
		}
		if err != nil {
			writeInternalError(c, logger, err)
			c.Abort()
			return
		}
		c.Set("customerID", session.User.ID)
		c.Next()
	}
}
