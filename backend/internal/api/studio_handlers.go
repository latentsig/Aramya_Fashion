package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/latentsig/aramya-fashion/backend/internal/appointments"
	"github.com/latentsig/aramya-fashion/backend/internal/auth"
)

type StudioRequestWriter interface {
	CreateAppointment(context.Context, string, string, appointments.AppointmentInput) (appointments.CreatedRequest, bool, error)
	CreateCustomOrderRequest(context.Context, string, string, appointments.CustomOrderInput) (appointments.CreatedRequest, bool, error)
}

func registerStudioRoutes(v1 *gin.RouterGroup, writer StudioRequestWriter, authService *auth.Service, logger *slog.Logger) {
	requestLimits := newRateLimiter(8, time.Minute)
	v1.POST("/appointments", requestLimits.Middleware(), func(c *gin.Context) {
		idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
		if len(idempotencyKey) < 16 || len(idempotencyKey) > 128 {
			writeValidationError(c, "A valid Idempotency-Key header is required")
			return
		}
		var request struct {
			Name     string `json:"name"`
			Email    string `json:"email"`
			Phone    string `json:"phone"`
			Interest string `json:"interest"`
			Note     string `json:"note"`
		}
		if err := bindLimitedJSON(c, &request); err != nil {
			writeValidationError(c, "Check the appointment details and try again")
			return
		}
		input := appointments.AppointmentInput{
			Name: strings.TrimSpace(request.Name), Email: strings.TrimSpace(request.Email), Phone: strings.TrimSpace(request.Phone),
			Interest: strings.TrimSpace(request.Interest), Note: strings.TrimSpace(request.Note),
		}
		if !validRequestName(input.Name) || !validContact(input.Email, input.Phone) || !validOptionalText(input.Interest, 120) || !validOptionalText(input.Note, 1000) {
			writeValidationError(c, "Add your name and a valid way for the studio to reach you")
			return
		}
		customerID, err := optionalCustomerID(c, authService)
		if err != nil {
			writeInternalError(c, logger, err)
			return
		}
		created, replayed, err := writer.CreateAppointment(c.Request.Context(), customerID, idempotencyKey, input)
		if errors.Is(err, appointments.ErrIdempotencyConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "idempotency_conflict", "message": "This request key was already used with different details"}})
			return
		}
		if err != nil {
			writeInternalError(c, logger, err)
			return
		}
		status := http.StatusCreated
		if replayed {
			status = http.StatusOK
		}
		c.JSON(status, gin.H{"data": created})
	})

	v1.POST("/custom-order-requests", requestLimits.Middleware(), func(c *gin.Context) {
		idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
		if len(idempotencyKey) < 16 || len(idempotencyKey) > 128 {
			writeValidationError(c, "A valid Idempotency-Key header is required")
			return
		}
		var request struct {
			Name         string `json:"name"`
			Email        string `json:"email"`
			Phone        string `json:"phone"`
			GarmentType  string `json:"garmentType"`
			Occasion     string `json:"occasion"`
			Requirements string `json:"requirements"`
		}
		if err := bindLimitedJSON(c, &request); err != nil {
			writeValidationError(c, "Check the custom order details and try again")
			return
		}
		input := appointments.CustomOrderInput{
			Name: strings.TrimSpace(request.Name), Email: strings.TrimSpace(request.Email), Phone: strings.TrimSpace(request.Phone),
			GarmentType: strings.TrimSpace(request.GarmentType), Occasion: strings.TrimSpace(request.Occasion), Requirements: strings.TrimSpace(request.Requirements),
		}
		if !validRequestName(input.Name) || !validContact(input.Email, input.Phone) || !validRequiredText(input.GarmentType, 120) || !validOptionalText(input.Occasion, 160) || !validOptionalText(input.Requirements, 3000) {
			writeValidationError(c, "Add your name, garment type and a valid way for the studio to reach you")
			return
		}
		customerID, err := optionalCustomerID(c, authService)
		if err != nil {
			writeInternalError(c, logger, err)
			return
		}
		created, replayed, err := writer.CreateCustomOrderRequest(c.Request.Context(), customerID, idempotencyKey, input)
		if errors.Is(err, appointments.ErrIdempotencyConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "idempotency_conflict", "message": "This request key was already used with different details"}})
			return
		}
		if err != nil {
			writeInternalError(c, logger, err)
			return
		}
		status := http.StatusCreated
		if replayed {
			status = http.StatusOK
		}
		c.JSON(status, gin.H{"data": created})
	})
}

func optionalCustomerID(c *gin.Context, service *auth.Service) (string, error) {
	if service == nil {
		return "", nil
	}
	token, err := c.Cookie(sessionCookieName)
	if err != nil {
		return "", nil
	}
	session, err := service.Authenticate(c.Request.Context(), token)
	if errors.Is(err, auth.ErrUnauthenticated) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return session.User.ID, nil
}

func validRequestName(name string) bool {
	return validRequiredText(name, 120)
}

func validRequiredText(value string, max int) bool {
	return strings.TrimSpace(value) != "" && utf8.ValidString(value) && len(value) <= max
}

func validOptionalText(value string, max int) bool {
	return utf8.ValidString(value) && len(value) <= max
}

func validContact(email, phone string) bool {
	if email == "" && phone == "" {
		return false
	}
	if email != "" {
		if len(email) > 254 {
			return false
		}
		address, err := mail.ParseAddress(email)
		if err != nil || address.Address != email {
			return false
		}
	}
	if phone != "" {
		if len(phone) < 7 || len(phone) > 32 {
			return false
		}
		digits := 0
		for _, char := range phone {
			switch {
			case char >= '0' && char <= '9':
				digits++
			case char == '+' || char == '(' || char == ')' || char == '-' || char == '.' || char == ' ':
			default:
				return false
			}
		}
		if digits < 7 || digits > 15 {
			return false
		}
	}
	return true
}

func writeValidationError(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": message}})
}
