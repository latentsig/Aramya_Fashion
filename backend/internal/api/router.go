package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/latentsig/aramya-fashion/backend/internal/auth"
	"github.com/latentsig/aramya-fashion/backend/internal/cart"
	"github.com/latentsig/aramya-fashion/backend/internal/catalog"
	"github.com/latentsig/aramya-fashion/backend/internal/checkout"
	"github.com/latentsig/aramya-fashion/backend/internal/observability"
)

type RouterConfig struct {
	Catalog               *catalog.Repository
	Auth                  *auth.Service
	Cart                  *cart.Service
	Checkout              *checkout.Service
	Orders                OrderReader
	StudioRequests        StudioRequestWriter
	AdminStudio           AdminStudioRepository
	AdminInventory        AdminInventoryRepository
	AdminCatalog          AdminCatalogRepository
	AdminOrders           AdminOrderRepository
	AdminMedia            AdminMediaService
	AdminIdentity         AdminIdentity
	Ready                 func() error
	AllowedOrigins        []string
	SessionCookieSecure   bool
	SessionCookieSameSite string
	CSRFSecret            []byte
	Metrics               *observability.Metrics
	MetricsBearerToken    string
	Logger                *slog.Logger
}

func NewRouter(cfg RouterConfig) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(requestID(), accessLog(cfg.Logger), gin.Recovery(), cors(cfg.AllowedOrigins), newRateLimiter(120, time.Minute).Middleware())
	if cfg.Metrics != nil {
		router.Use(cfg.Metrics.Middleware())
	}
	router.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	router.GET("/ready", func(c *gin.Context) {
		ready := cfg.Ready != nil && cfg.Ready() == nil
		if cfg.Metrics != nil {
			cfg.Metrics.SetDatabaseReady(ready)
		}
		if !ready {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
	if cfg.Metrics != nil && cfg.MetricsBearerToken != "" {
		router.GET("/metrics", func(c *gin.Context) {
			authorization := c.GetHeader("Authorization")
			const prefix = "Bearer "
			provided := sha256.Sum256([]byte(strings.TrimPrefix(authorization, prefix)))
			expected := sha256.Sum256([]byte(cfg.MetricsBearerToken))
			if !strings.HasPrefix(authorization, prefix) || subtle.ConstantTimeCompare(provided[:], expected[:]) != 1 {
				c.Header("WWW-Authenticate", `Bearer realm="metrics"`)
				c.Status(http.StatusUnauthorized)
				return
			}
			cfg.Metrics.Handler().ServeHTTP(c.Writer, c.Request)
		})
	}

	v1 := router.Group("/api/v1")
	if cfg.Auth != nil {
		v1.Use(csrfGuard(cfg.Auth, cfg.CSRFSecret))
		registerAuthRoutes(v1, cfg.Auth, cfg.CSRFSecret, cfg.SessionCookieSecure, cookieSameSite(cfg.SessionCookieSameSite), cfg.Logger)
	}
	if cfg.Cart != nil {
		registerCartRoutes(v1, cfg.Cart, cfg.Auth, cfg.SessionCookieSecure, cookieSameSite(cfg.SessionCookieSameSite), cfg.Logger)
	}
	if cfg.Checkout != nil && cfg.Cart != nil {
		registerCheckoutRoutes(v1, cfg.Checkout, cfg.Cart, cfg.Auth, cfg.SessionCookieSecure, cookieSameSite(cfg.SessionCookieSameSite), cfg.Logger, cfg.Metrics)
	}
	if cfg.Orders != nil {
		registerOrderRoutes(v1, cfg.Orders, cfg.Auth, cfg.Logger)
	}
	if cfg.StudioRequests != nil {
		registerStudioRoutes(v1, cfg.StudioRequests, cfg.Auth, cfg.Logger)
	}
	if cfg.AdminStudio != nil {
		registerAdminStudioRoutes(v1, cfg.AdminStudio, cfg.AdminIdentity, cfg.Logger)
	}
	if cfg.AdminInventory != nil {
		registerAdminInventoryRoutes(v1, cfg.AdminInventory, cfg.AdminIdentity, cfg.Logger)
	}
	if cfg.AdminCatalog != nil {
		registerAdminCatalogRoutes(v1, cfg.AdminCatalog, cfg.AdminIdentity, cfg.Logger)
	}
	if cfg.AdminOrders != nil {
		registerAdminOrderRoutes(v1, cfg.AdminOrders, cfg.AdminIdentity, cfg.Logger)
	}
	registerAdminMediaRoutes(v1, cfg.AdminMedia, cfg.AdminIdentity, cfg.Logger)
	v1.GET("/categories", func(c *gin.Context) {
		items, err := cfg.Catalog.ListCategories(c.Request.Context())
		if err != nil {
			writeInternalError(c, cfg.Logger, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": items})
	})
	v1.GET("/products", func(c *gin.Context) {
		limit, err := parseBoundedInt(c.DefaultQuery("limit", "24"), 24, 1, 100)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_limit", "message": "limit must be between 1 and 100"}})
			return
		}
		offset, err := parseBoundedInt(c.DefaultQuery("offset", "0"), 0, 0, 100000)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_offset", "message": "offset must be non-negative"}})
			return
		}
		items, err := cfg.Catalog.ListProducts(c.Request.Context(), c.Query("category"), c.Query("search"), limit, offset)
		if err != nil {
			writeInternalError(c, cfg.Logger, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": items, "pagination": gin.H{"limit": limit, "offset": offset, "count": len(items)}})
	})
	v1.GET("/products/:slug", func(c *gin.Context) {
		item, err := cfg.Catalog.GetProductDetail(c.Request.Context(), c.Param("slug"))
		if err == catalog.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "product_not_found", "message": "Product not found"}})
			return
		}
		if err != nil {
			writeInternalError(c, cfg.Logger, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": item})
	})
	return router
}

func parseBoundedInt(value string, fallback, min, max int) (int, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < min || parsed > max {
		return 0, strconv.ErrSyntax
	}
	return parsed, nil
}

func requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if id == "" || len(id) > 128 || !utf8.ValidString(id) {
			id = strconv.FormatInt(time.Now().UnixNano(), 36)
		}
		c.Set("requestID", id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

func accessLog(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		if logger != nil {
			logger.Info("http_request", "request_id", c.GetString("requestID"), "method", c.Request.Method,
				"path", c.FullPath(), "status", c.Writer.Status(), "duration_ms", time.Since(started).Milliseconds())
		}
	}
}

func cors(origins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		allowed[origin] = struct{}{}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			if _, ok := allowed[origin]; ok {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Vary", "Origin")
				c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				c.Header("Access-Control-Allow-Headers", "Content-Type, X-Request-ID, X-CSRF-Token, Idempotency-Key")
				c.Header("Access-Control-Allow-Credentials", "true")
			}
			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
		}
		c.Next()
	}
}

func writeInternalError(c *gin.Context, logger *slog.Logger, err error) {
	if logger != nil {
		logger.Error("request_failed", "request_id", c.GetString("requestID"), "error_type", fmt.Sprintf("%T", err))
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal_error", "message": "The request could not be completed"}})
}
