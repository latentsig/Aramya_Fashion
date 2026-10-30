package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/latentsig/aramya-fashion/backend/internal/admin"
	"github.com/latentsig/aramya-fashion/backend/internal/media"
	"github.com/latentsig/aramya-fashion/backend/internal/storage"
)

type AdminMediaService interface {
	CreateUploadIntent(ctx context.Context, input media.UploadRequest, actorID, requestID string) (media.UploadIntent, error)
	CompleteUpload(ctx context.Context, intentID, actorID, requestID string) (admin.MediaObject, error)
}

func registerAdminMediaRoutes(v1 *gin.RouterGroup, service AdminMediaService, identity AdminIdentity, logger *slog.Logger) {
	group := v1.Group("/admin")
	group.Use(requireAdminSession(identity, logger))
	group.POST("/media/upload-intent", newRateLimiter(20, time.Minute).Middleware(), func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": "media_storage_unavailable", "message": "Product media storage is not configured in this environment"}})
			return
		}
		var request media.UploadRequest
		if bindLimitedJSON(c, &request) != nil {
			writeMediaValidation(c)
			return
		}
		intent, err := service.CreateUploadIntent(c.Request.Context(), request, c.GetString("adminActorID"), c.GetString("requestID"))
		if writeMediaError(c, logger, err) {
			return
		}
		c.JSON(http.StatusCreated, gin.H{"data": intent})
	})
	group.POST("/media/complete", newRateLimiter(30, time.Minute).Middleware(), func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": "media_storage_unavailable", "message": "Product media storage is not configured in this environment"}})
			return
		}
		var request struct {
			IntentID string `json:"intentId"`
		}
		if bindLimitedJSON(c, &request) != nil || !uuidPattern.MatchString(strings.TrimSpace(request.IntentID)) {
			writeMediaValidation(c)
			return
		}
		item, err := service.CompleteUpload(c.Request.Context(), strings.TrimSpace(request.IntentID), c.GetString("adminActorID"), c.GetString("requestID"))
		if writeMediaError(c, logger, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": item})
	})
}

func writeMediaValidation(c *gin.Context) {
	c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_media_request", "message": "Check the filename, image type, size, checksum, and category"}})
}

func writeMediaError(c *gin.Context, logger *slog.Logger, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, media.ErrInvalidRequest), errors.Is(err, storage.ErrInvalidUpload):
		writeMediaValidation(c)
	case errors.Is(err, admin.ErrMediaNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "media_upload_not_found", "message": "Media upload was not found"}})
	case errors.Is(err, storage.ErrUploadNotReady):
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "media_upload_missing", "message": "Upload the image using the signed URL before completing it"}})
	case errors.Is(err, storage.ErrObjectMismatch):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": gin.H{"code": "media_verification_failed", "message": "The uploaded image did not match its approved file type, size, or checksum"}})
	default:
		writeInternalError(c, logger, err)
	}
	return true
}
