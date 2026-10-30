package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/latentsig/aramya-fashion/backend/internal/admin"
)

type AdminOverviewReader interface {
	Get(context.Context) (admin.Overview, error)
}

func registerAdminOverviewRoute(v1 *gin.RouterGroup, reader AdminOverviewReader, identity AdminIdentity, logger *slog.Logger) {
	group := v1.Group("/admin")
	group.Use(requireAdminSession(identity, logger))
	group.GET("/overview", func(c *gin.Context) {
		result, err := reader.Get(c.Request.Context())
		if err != nil {
			writeInternalError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": result})
	})
}
