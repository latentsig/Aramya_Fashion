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
	"github.com/latentsig/aramya-fashion/backend/internal/auth"
)

type AdminStudioRepository interface {
	ListAppointments(context.Context, admin.ListFilter) ([]admin.Appointment, error)
	ListCustomOrders(context.Context, admin.ListFilter) ([]admin.CustomOrderRequest, error)
	UpdateAppointment(context.Context, string, string, string, string, string) (admin.Appointment, error)
	UpdateCustomOrder(context.Context, string, string, string, *int64, string, string) (admin.CustomOrderRequest, error)
}

type AdminIdentity interface {
	Authenticate(context.Context, string) (auth.AuthenticatedSession, error)
	HasRole(context.Context, string, string) (bool, error)
}

func registerAdminStudioRoutes(v1 *gin.RouterGroup, repository AdminStudioRepository, identity AdminIdentity, logger *slog.Logger) {
	group := v1.Group("/admin")
	group.Use(requireAdminSession(identity, logger))
	group.GET("/appointments", func(c *gin.Context) {
		filter, ok := adminListFilter(c, appointmentStatusFilter)
		if !ok {
			return
		}
		items, err := repository.ListAppointments(c.Request.Context(), filter.withExtraItem())
		if err != nil {
			writeInternalError(c, logger, err)
			return
		}
		more := len(items) > filter.Limit
		if more {
			items = items[:filter.Limit]
		}
		c.JSON(http.StatusOK, gin.H{"data": items, "pagination": gin.H{"limit": filter.Limit, "offset": filter.Offset, "count": len(items), "hasMore": more}})
	})
	group.PATCH("/appointments/:id", func(c *gin.Context) {
		if !uuidPattern.MatchString(c.Param("id")) {
			writeValidationError(c, "Appointment ID is invalid")
			return
		}
		var request struct {
			ExpectedStatus string `json:"expectedStatus"`
			Status         string `json:"status"`
		}
		if err := bindLimitedJSON(c, &request); err != nil || !admin.ValidAppointmentStatus(request.ExpectedStatus) || !admin.ValidAppointmentStatus(request.Status) || !admin.CanTransitionAppointment(request.ExpectedStatus, request.Status) {
			writeValidationError(c, "Choose a valid next appointment status")
			return
		}
		item, err := repository.UpdateAppointment(c.Request.Context(), c.Param("id"), request.ExpectedStatus, request.Status, c.GetString("adminActorID"), c.GetString("requestID"))
		if writeStudioAdminError(c, logger, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": item})
	})
	group.GET("/custom-order-requests", func(c *gin.Context) {
		filter, ok := adminListFilter(c, customOrderStatusFilter)
		if !ok {
			return
		}
		items, err := repository.ListCustomOrders(c.Request.Context(), filter.withExtraItem())
		if err != nil {
			writeInternalError(c, logger, err)
			return
		}
		more := len(items) > filter.Limit
		if more {
			items = items[:filter.Limit]
		}
		c.JSON(http.StatusOK, gin.H{"data": items, "pagination": gin.H{"limit": filter.Limit, "offset": filter.Offset, "count": len(items), "hasMore": more}})
	})
	group.PATCH("/custom-order-requests/:id", func(c *gin.Context) {
		if !uuidPattern.MatchString(c.Param("id")) {
			writeValidationError(c, "Custom order request ID is invalid")
			return
		}
		var request struct {
			ExpectedStatus string `json:"expectedStatus"`
			Status         string `json:"status"`
			QuotationPaise *int64 `json:"quotationPaise"`
		}
		if err := bindLimitedJSON(c, &request); err != nil || !admin.ValidCustomOrderStatus(request.ExpectedStatus) || !admin.ValidCustomOrderStatus(request.Status) || !admin.CanTransitionCustomOrder(request.ExpectedStatus, request.Status) || request.Status == "quoted" && (request.QuotationPaise == nil || *request.QuotationPaise <= 0) || request.Status != "quoted" && request.QuotationPaise != nil {
			writeValidationError(c, "Choose a valid next custom-order status and provide a positive quotation in paise when quoting")
			return
		}
		item, err := repository.UpdateCustomOrder(c.Request.Context(), c.Param("id"), request.ExpectedStatus, request.Status, request.QuotationPaise, c.GetString("adminActorID"), c.GetString("requestID"))
		if writeStudioAdminError(c, logger, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": item})
	})
}

type studioListFilter struct {
	Status, Search string
	Limit, Offset  int
}

func (f studioListFilter) withExtraItem() admin.ListFilter {
	return admin.ListFilter{Status: f.Status, Search: f.Search, Limit: f.Limit + 1, Offset: f.Offset}
}

func adminListFilter(c *gin.Context, validStatus func(string) bool) (studioListFilter, bool) {
	limit, err := parseBoundedInt(c.DefaultQuery("limit", "20"), 20, 1, 50)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_limit", "message": "limit must be between 1 and 50"}})
		return studioListFilter{}, false
	}
	offset, err := parseBoundedInt(c.DefaultQuery("offset", "0"), 0, 0, 100000)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_offset", "message": "offset must be between 0 and 100000"}})
		return studioListFilter{}, false
	}
	status := strings.TrimSpace(c.Query("status"))
	if status != "" && !validStatus(status) {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_status", "message": "Status filter is invalid"}})
		return studioListFilter{}, false
	}
	search := strings.TrimSpace(c.Query("search"))
	if !utf8.ValidString(search) || len(search) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_search", "message": "Search must be at most 100 characters"}})
		return studioListFilter{}, false
	}
	return studioListFilter{Status: status, Search: search, Limit: limit, Offset: offset}, true
}

func appointmentStatusFilter(value string) bool { return admin.ValidAppointmentStatus(value) }
func customOrderStatusFilter(value string) bool { return admin.ValidCustomOrderStatus(value) }

func requireAdminSession(identity AdminIdentity, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if identity == nil {
			unauthenticatedAdmin(c)
			return
		}
		token, err := c.Cookie(sessionCookieName)
		if err != nil {
			unauthenticatedAdmin(c)
			return
		}
		session, err := identity.Authenticate(c.Request.Context(), token)
		if errors.Is(err, auth.ErrUnauthenticated) {
			unauthenticatedAdmin(c)
			return
		}
		if err != nil {
			writeInternalError(c, logger, err)
			c.Abort()
			return
		}
		allowed, err := identity.HasRole(c.Request.Context(), session.User.ID, "admin")
		if err != nil {
			writeInternalError(c, logger, err)
			c.Abort()
			return
		}
		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "forbidden", "message": "Administrator access is required"}})
			c.Abort()
			return
		}
		c.Set("adminActorID", session.User.ID)
		c.Next()
	}
}

func unauthenticatedAdmin(c *gin.Context) {
	c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "unauthenticated", "message": "Sign in with an administrator account"}})
	c.Abort()
}

func writeStudioAdminError(c *gin.Context, logger *slog.Logger, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, admin.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "studio_request_not_found", "message": "Studio request not found"}})
	case errors.Is(err, admin.ErrInvalidStatus), errors.Is(err, admin.ErrInvalidQuote):
		writeValidationError(c, "The requested status or quotation is invalid")
	case errors.Is(err, admin.ErrInvalidTransition), errors.Is(err, admin.ErrStaleRequest):
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "studio_request_conflict", "message": "This request has changed or cannot move to that status. Refresh the queue and retry"}})
	default:
		writeInternalError(c, logger, err)
	}
	return true
}
