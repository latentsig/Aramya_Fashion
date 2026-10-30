package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/latentsig/aramya-fashion/backend/internal/auth"
	"github.com/latentsig/aramya-fashion/backend/internal/cart"
	"github.com/latentsig/aramya-fashion/backend/internal/payments"
)

type CheckoutPaymentGateway interface {
	Start(ctx context.Context, orderID, customerID, guestCartID, idempotencyKey string) (payments.CheckoutSession, error)
	Verify(ctx context.Context, orderID, customerID, guestCartID, providerOrderID, paymentID, signature string) (payments.PaymentResult, error)
}

func registerCheckoutPaymentRoutes(v1 *gin.RouterGroup, gateway CheckoutPaymentGateway, cartService *cart.Service, authService *auth.Service, logger *slog.Logger) {
	v1.POST("/checkout/orders/:id/payment", func(c *gin.Context) {
		if !uuidPattern.MatchString(c.Param("id")) {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_order_id", "message": "Order reference is invalid"}})
			return
		}
		userID, guestToken, err := cartOwner(c, authService)
		if err != nil {
			writeInternalError(c, logger, err)
			return
		}
		cartID, _, err := cartService.ResolveForCheckout(c.Request.Context(), userID, guestToken)
		if err != nil {
			writeInternalError(c, logger, err)
			return
		}
		key := c.GetHeader("Idempotency-Key")
		if key == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "idempotency_key_required", "message": "Retry the payment request safely"}})
			return
		}
		session, err := gateway.Start(c.Request.Context(), c.Param("id"), userID, cartID, key)
		if writePaymentError(c, logger, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": session})
	})
	v1.POST("/payments/razorpay/verify", func(c *gin.Context) {
		var request struct {
			OrderID         string `json:"orderId"`
			ProviderOrderID string `json:"razorpayOrderId"`
			PaymentID       string `json:"razorpayPaymentId"`
			Signature       string `json:"razorpaySignature"`
		}
		if err := bindLimitedJSON(c, &request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_payment_callback", "message": "Payment confirmation is invalid"}})
			return
		}
		if !uuidPattern.MatchString(request.OrderID) {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_order_id", "message": "Order reference is invalid"}})
			return
		}
		userID, guestToken, err := cartOwner(c, authService)
		if err != nil {
			writeInternalError(c, logger, err)
			return
		}
		cartID, _, err := cartService.ResolveForCheckout(c.Request.Context(), userID, guestToken)
		if err != nil {
			writeInternalError(c, logger, err)
			return
		}
		result, err := gateway.Verify(c.Request.Context(), request.OrderID, userID, cartID, request.ProviderOrderID, request.PaymentID, request.Signature)
		if writePaymentError(c, logger, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": result})
	})
}

func writePaymentError(c *gin.Context, logger *slog.Logger, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, payments.ErrOrderUnavailable):
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "order_unavailable", "message": "This order is not available for payment"}})
	case errors.Is(err, payments.ErrAttemptPending):
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "payment_reconciliation_required", "message": "A previous payment attempt is still being confirmed. Please contact the studio before retrying."}})
	case errors.Is(err, payments.ErrPaymentMismatch):
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "payment_verification_failed", "message": "Payment details could not be verified"}})
	case errors.Is(err, payments.ErrPaymentNotReady):
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "payment_not_captured", "message": "Payment is not captured yet"}})
	case errors.Is(err, payments.ErrProviderUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": "payment_provider_unavailable", "message": "Payment status could not be confirmed. Check the order before retrying."}})
	default:
		writeInternalError(c, logger, err)
		return true
	}
	return true
}
