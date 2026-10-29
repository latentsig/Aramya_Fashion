package api

import (
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/latentsig/aramya-fashion/backend/internal/auth"
	"github.com/latentsig/aramya-fashion/backend/internal/cart"
)

const guestCartCookieName = "aramya_cart_guest"

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func registerCartRoutes(v1 *gin.RouterGroup, service *cart.Service, authService *auth.Service, secure bool, sameSite http.SameSite, logger *slog.Logger) {
	api := v1.Group("/cart")
	api.GET("", func(c *gin.Context) {
		userID, guestToken, err := cartOwner(c, authService)
		if err != nil {
			writeInternalError(c, logger, err)
			return
		}
		snapshot, nextToken, err := service.Get(c.Request.Context(), userID, guestToken)
		if err != nil {
			writeInternalError(c, logger, err)
			return
		}
		setGuestCartCookie(c, nextToken, secure, sameSite)
		c.JSON(http.StatusOK, gin.H{"data": snapshot})
	})
	api.POST("/items", func(c *gin.Context) {
		var input struct {
			VariantID string `json:"variantId"`
			Quantity  int    `json:"quantity"`
		}
		if err := bindLimitedJSON(c, &input); err != nil || !uuidPattern.MatchString(input.VariantID) || input.Quantity < 1 || input.Quantity > 20 {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_cart_item", "message": "Choose a valid product and quantity"}})
			return
		}
		userID, guestToken, err := cartOwner(c, authService)
		if err != nil {
			writeInternalError(c, logger, err)
			return
		}
		snapshot, nextToken, err := service.Add(c.Request.Context(), userID, guestToken, input.VariantID, input.Quantity)
		if writeCartError(c, logger, err) {
			return
		}
		setGuestCartCookie(c, nextToken, secure, sameSite)
		c.JSON(http.StatusOK, gin.H{"data": snapshot})
	})
	api.PATCH("/items/:id", func(c *gin.Context) {
		if !uuidPattern.MatchString(c.Param("id")) {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_cart_item", "message": "Choose a valid cart item"}})
			return
		}
		var input struct {
			Quantity int `json:"quantity"`
		}
		if err := bindLimitedJSON(c, &input); err != nil || input.Quantity < 0 || input.Quantity > 20 {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_quantity", "message": "Quantity must be between 0 and 20"}})
			return
		}
		userID, guestToken, err := cartOwner(c, authService)
		if err != nil {
			writeInternalError(c, logger, err)
			return
		}
		snapshot, nextToken, err := service.SetQuantity(c.Request.Context(), userID, guestToken, c.Param("id"), input.Quantity)
		if writeCartError(c, logger, err) {
			return
		}
		setGuestCartCookie(c, nextToken, secure, sameSite)
		c.JSON(http.StatusOK, gin.H{"data": snapshot})
	})
	api.DELETE("/items/:id", func(c *gin.Context) {
		if !uuidPattern.MatchString(c.Param("id")) {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_cart_item", "message": "Choose a valid cart item"}})
			return
		}
		userID, guestToken, err := cartOwner(c, authService)
		if err != nil {
			writeInternalError(c, logger, err)
			return
		}
		snapshot, nextToken, err := service.Remove(c.Request.Context(), userID, guestToken, c.Param("id"))
		if writeCartError(c, logger, err) {
			return
		}
		setGuestCartCookie(c, nextToken, secure, sameSite)
		c.JSON(http.StatusOK, gin.H{"data": snapshot})
	})
}

func cartOwner(c *gin.Context, authService *auth.Service) (string, string, error) {
	guestToken, _ := c.Cookie(guestCartCookieName)
	token, err := c.Cookie(sessionCookieName)
	if err != nil || authService == nil {
		return "", guestToken, nil
	}
	session, err := authService.Authenticate(c.Request.Context(), token)
	if errors.Is(err, auth.ErrUnauthenticated) {
		return "", guestToken, nil
	}
	if err != nil {
		return "", "", err
	}
	return session.User.ID, guestToken, nil
}

func writeCartError(c *gin.Context, logger *slog.Logger, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, cart.ErrQuantity):
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_quantity", "message": "Quantity must be between 1 and 20"}})
	case errors.Is(err, cart.ErrProductUnavailable):
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "product_unavailable", "message": "This product is not available"}})
	case errors.Is(err, cart.ErrOutOfStock):
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "insufficient_stock", "message": "The requested quantity is not available"}})
	case errors.Is(err, cart.ErrItemNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "cart_item_not_found", "message": "Cart item not found"}})
	default:
		writeInternalError(c, logger, err)
	}
	return true
}

func setGuestCartCookie(c *gin.Context, token string, secure bool, sameSite http.SameSite) {
	if token == "" {
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{Name: guestCartCookieName, Value: token, Path: "/api/v1", MaxAge: 30 * 24 * 60 * 60, Expires: time.Now().Add(30 * 24 * time.Hour), Secure: secure, HttpOnly: true, SameSite: sameSite})
}
