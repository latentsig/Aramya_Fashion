package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/latentsig/aramya-fashion/backend/internal/auth"
)

const (
	sessionCookieName = "aramya_session"
	csrfCookieName    = "aramya_csrf"
)

func registerAuthRoutes(v1 *gin.RouterGroup, service *auth.Service, csrfSecret []byte, secure bool, sameSite http.SameSite, logger *slog.Logger) {
	authRoutes := v1.Group("/auth")
	authAttempts := newRateLimiter(10, time.Minute)
	authRoutes.GET("/csrf", func(c *gin.Context) {
		token, err := ensureCSRF(c, service, csrfSecret, secure, sameSite)
		if err != nil {
			writeInternalError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"csrfToken": token})
	})
	authRoutes.POST("/register", authAttempts.Middleware(), func(c *gin.Context) {
		var request struct {
			Email       string `json:"email"`
			Password    string `json:"password"`
			DisplayName string `json:"displayName"`
		}
		if err := bindLimitedJSON(c, &request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": "Check the registration details and try again"}})
			return
		}
		rotateFrom := authenticatedSessionID(c, service)
		session, err := service.Register(c.Request.Context(), request.Email, request.Password, request.DisplayName, rotateFrom)
		if errors.Is(err, auth.ErrEmailTaken) {
			c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "email_unavailable", "message": "An account with this email already exists"}})
			return
		}
		if err != nil {
			if errors.Is(err, auth.ErrInvalidEmail) || errors.Is(err, auth.ErrInvalidPassword) || errors.Is(err, auth.ErrInvalidName) {
				c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_registration", "message": err.Error()}})
				return
			}
			writeInternalError(c, logger, err)
			return
		}
		setSessionCookies(c, session, secure, sameSite)
		c.JSON(http.StatusCreated, gin.H{"user": session.User, "csrfToken": session.CSRFToken})
	})
	authRoutes.POST("/login", authAttempts.Middleware(), func(c *gin.Context) {
		var request struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := bindLimitedJSON(c, &request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": "Check the email and password and try again"}})
			return
		}
		session, err := service.Login(c.Request.Context(), request.Email, request.Password, authenticatedSessionID(c, service))
		if errors.Is(err, auth.ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "invalid_credentials", "message": "Email or password is incorrect"}})
			return
		}
		if err != nil {
			writeInternalError(c, logger, err)
			return
		}
		setSessionCookies(c, session, secure, sameSite)
		c.JSON(http.StatusOK, gin.H{"user": session.User, "csrfToken": session.CSRFToken})
	})
	authRoutes.POST("/logout", func(c *gin.Context) {
		token, _ := c.Cookie(sessionCookieName)
		if err := service.Logout(c.Request.Context(), token); err != nil {
			writeInternalError(c, logger, err)
			return
		}
		clearSessionCookies(c, secure, sameSite)
		c.Status(http.StatusNoContent)
	})
	authRoutes.GET("/me", func(c *gin.Context) {
		token, _ := c.Cookie(sessionCookieName)
		session, err := service.Authenticate(c.Request.Context(), token)
		if errors.Is(err, auth.ErrUnauthenticated) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "unauthenticated", "message": "Sign in to continue"}})
			return
		}
		if err != nil {
			writeInternalError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"user": session.User})
	})
}

func csrfGuard(service *auth.Service, csrfSecret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead || c.Request.Method == http.MethodOptions || c.Request.Method == http.MethodTrace {
			c.Next()
			return
		}
		cookie, err := c.Cookie(csrfCookieName)
		header := c.GetHeader("X-CSRF-Token")
		if err != nil || len(cookie) != len(header) || subtle.ConstantTimeCompare([]byte(cookie), []byte(header)) != 1 {
			c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "csrf_rejected", "message": "Refresh the page and try again"}})
			c.Abort()
			return
		}
		if token, err := c.Cookie(sessionCookieName); err == nil {
			session, authErr := service.Authenticate(c.Request.Context(), token)
			if authErr == nil {
				hash := sha256.Sum256([]byte(header))
				if len(session.CSRFTokenHash) != len(hash) || subtle.ConstantTimeCompare(session.CSRFTokenHash, hash[:]) != 1 {
					c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "csrf_rejected", "message": "Refresh the page and try again"}})
					c.Abort()
					return
				}
			} else if errors.Is(authErr, auth.ErrUnauthenticated) {
				if !verifyAnonymousCSRF(header, csrfSecret, time.Now()) {
					c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "csrf_rejected", "message": "Refresh the page and try again"}})
					c.Abort()
					return
				}
			} else {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": "auth_unavailable", "message": "Authentication is temporarily unavailable"}})
				c.Abort()
				return
			}
		} else if !verifyAnonymousCSRF(header, csrfSecret, time.Now()) {
			c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "csrf_rejected", "message": "Refresh the page and try again"}})
			c.Abort()
			return
		}
		c.Next()
	}
}

func ensureCSRF(c *gin.Context, service *auth.Service, csrfSecret []byte, secure bool, sameSite http.SameSite) (string, error) {
	current, _ := c.Cookie(csrfCookieName)
	if token, err := c.Cookie(sessionCookieName); err == nil {
		if session, authErr := service.Authenticate(c.Request.Context(), token); authErr == nil {
			hash := sha256.Sum256([]byte(current))
			if len(current) > 0 && len(session.CSRFTokenHash) == len(hash) && subtle.ConstantTimeCompare(session.CSRFTokenHash, hash[:]) == 1 {
				return current, nil
			}
			rotated, _, rotateErr := service.RotateCSRF(c.Request.Context(), session.ID)
			if rotateErr != nil {
				return "", rotateErr
			}
			setCSRFCookie(c, rotated, secure, sameSite)
			return rotated, nil
		} else if authErr != nil && !errors.Is(authErr, auth.ErrUnauthenticated) {
			return "", authErr
		}
	}
	if verifyAnonymousCSRF(current, csrfSecret, time.Now()) {
		return current, nil
	}
	token, err := newAnonymousCSRF(csrfSecret, time.Now())
	if err != nil {
		return "", err
	}
	setCSRFCookie(c, token, secure, sameSite)
	return token, nil
}

func authenticatedSessionID(c *gin.Context, service *auth.Service) string {
	token, err := c.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	session, err := service.Authenticate(c.Request.Context(), token)
	if err != nil {
		return ""
	}
	return session.ID
}

func setSessionCookies(c *gin.Context, session auth.Session, secure bool, sameSite http.SameSite) {
	maxAge := int(time.Until(session.ExpiresAt).Seconds())
	http.SetCookie(c.Writer, &http.Cookie{Name: sessionCookieName, Value: session.Token, Path: "/api/v1", MaxAge: maxAge, Expires: session.ExpiresAt, Secure: secure, HttpOnly: true, SameSite: sameSite})
	cookie := csrfCookie(session.CSRFToken, secure, sameSite)
	if maxAge > 0 {
		cookie.MaxAge = maxAge
		cookie.Expires = session.ExpiresAt
	}
	http.SetCookie(c.Writer, cookie)
}

func clearSessionCookies(c *gin.Context, secure bool, sameSite http.SameSite) {
	http.SetCookie(c.Writer, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/api/v1", MaxAge: -1, Expires: time.Unix(1, 0), Secure: secure, HttpOnly: true, SameSite: sameSite})
	http.SetCookie(c.Writer, &http.Cookie{Name: csrfCookieName, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(1, 0), Secure: secure, SameSite: sameSite})
}

func setCSRFCookie(c *gin.Context, token string, secure bool, sameSite http.SameSite) {
	http.SetCookie(c.Writer, csrfCookie(token, secure, sameSite))
}

func csrfCookie(token string, secure bool, sameSite http.SameSite) *http.Cookie {
	return &http.Cookie{Name: csrfCookieName, Value: token, Path: "/", MaxAge: 15 * 60, Expires: time.Now().Add(15 * time.Minute), Secure: secure, HttpOnly: false, SameSite: sameSite}
}

func cookieSameSite(value string) http.SameSite {
	switch value {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

func newAnonymousCSRF(secret []byte, now time.Time) (string, error) {
	expires := now.Add(15 * time.Minute).Unix()
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	payload := strconv.FormatInt(expires, 10) + "." + base64.RawURLEncoding.EncodeToString(nonce)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func verifyAnonymousCSRF(token string, secret []byte, now time.Time) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || len(secret) < 32 {
		return false
	}
	expires, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || now.Unix() > expires || expires > now.Add(16*time.Minute).Unix() {
		return false
	}
	nonce, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(nonce) != 16 {
		return false
	}
	supplied, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	return hmac.Equal(supplied, mac.Sum(nil))
}

func bindLimitedJSON(c *gin.Context, destination any) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024)
	err := c.ShouldBindJSON(destination)
	if errors.Is(err, io.EOF) {
		return errors.New("request body is required")
	}
	return err
}
