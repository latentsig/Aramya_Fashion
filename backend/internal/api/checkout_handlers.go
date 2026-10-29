package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/latentsig/aramya-fashion/backend/internal/auth"
	"github.com/latentsig/aramya-fashion/backend/internal/cart"
	"github.com/latentsig/aramya-fashion/backend/internal/checkout"
)

func registerCheckoutRoutes(v1 *gin.RouterGroup, service *checkout.Service, cartService *cart.Service, authService *auth.Service, secure bool, sameSite http.SameSite, logger *slog.Logger) {
	api := v1.Group("/checkout")
	api.POST("/quote", func(c *gin.Context) {
		userID, guestToken, err := cartOwner(c, authService)
		if err != nil {
			writeInternalError(c, logger, err)
			return
		}
		cartID, nextToken, err := cartService.ResolveForCheckout(c.Request.Context(), userID, guestToken)
		if err != nil {
			writeInternalError(c, logger, err)
			return
		}
		quote, err := service.Quote(c.Request.Context(), cartID)
		if writeCheckoutError(c, logger, err) {
			return
		}
		setGuestCartCookie(c, nextToken, secure, sameSite)
		c.JSON(http.StatusOK, gin.H{"data": quote})
	})
	api.POST("/orders", func(c *gin.Context) {
		var details checkout.Details
		if err := bindLimitedJSON(c, &details); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_checkout", "message": "Check your contact and shipping details"}})
			return
		}
		key := c.GetHeader("Idempotency-Key")
		userID, guestToken, err := cartOwner(c, authService)
		if err != nil {
			writeInternalError(c, logger, err)
			return
		}
		cartID, nextToken, err := cartService.ResolveForCheckout(c.Request.Context(), userID, guestToken)
		if err != nil {
			writeInternalError(c, logger, err)
			return
		}
		order, replayed, err := service.CreateOrder(c.Request.Context(), cartID, userID, details.Email, details.Phone, key, details)
		if writeCheckoutError(c, logger, err) {
			return
		}
		setGuestCartCookie(c, nextToken, secure, sameSite)
		status := http.StatusCreated
		if replayed {
			status = http.StatusOK
		}
		c.JSON(status, gin.H{"data": order, "paymentAvailable": false, "notice": "Order saved as pending payment. Online payment is not configured in this environment."})
	})
}

func writeCheckoutError(c *gin.Context, logger *slog.Logger, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, checkout.ErrEmptyCart):
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "empty_cart", "message": "Add a style to your bag before checkout"}})
	case errors.Is(err, checkout.ErrInvalidCheckout):
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_checkout", "message": "Check your contact and shipping details"}})
	case errors.Is(err, checkout.ErrInsufficientStock):
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "insufficient_stock", "message": "A selected quantity is no longer available"}})
	case errors.Is(err, checkout.ErrIdempotencyConflict):
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "idempotency_conflict", "message": "This checkout request key was already used with different details"}})
	case errors.Is(err, cart.ErrOutOfStock):
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "insufficient_stock", "message": "A selected quantity is no longer available"}})
	default:
		writeInternalError(c, logger, err)
	}
	return true
}
