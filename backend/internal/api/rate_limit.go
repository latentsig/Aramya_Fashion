package api

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateWindow struct {
	started time.Time
	count   int
}

type rateLimiter struct {
	mu        sync.Mutex
	limit     int
	window    time.Duration
	clients   map[string]rateWindow
	nextSweep time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{limit: limit, window: window, clients: make(map[string]rateWindow), nextSweep: time.Now().Add(window)}
}

func (r *rateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		now := time.Now()
		client := remoteClientIP(c.Request.RemoteAddr)
		r.mu.Lock()
		if now.After(r.nextSweep) {
			for key, entry := range r.clients {
				if now.Sub(entry.started) >= r.window {
					delete(r.clients, key)
				}
			}
			r.nextSweep = now.Add(r.window)
		}
		entry := r.clients[client]
		if entry.started.IsZero() || now.Sub(entry.started) >= r.window {
			entry = rateWindow{started: now}
		}
		entry.count++
		r.clients[client] = entry
		allowed := entry.count <= r.limit
		remaining := time.Until(entry.started.Add(r.window))
		r.mu.Unlock()
		if !allowed {
			seconds := int(remaining.Seconds()) + 1
			c.Header("Retry-After", strconv.Itoa(seconds))
			c.JSON(http.StatusTooManyRequests, gin.H{"error": gin.H{"code": "rate_limited", "message": "Please wait before trying again"}})
			c.Abort()
			return
		}
		c.Next()
	}
}

func remoteClientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}
	if strings.TrimSpace(remoteAddr) == "" {
		return "unknown"
	}
	return remoteAddr
}
