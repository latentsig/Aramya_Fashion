package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailTaken         = errors.New("email is already registered")
	ErrUnauthenticated    = errors.New("unauthenticated")
	ErrInvalidEmail       = errors.New("a valid email address is required")
	ErrInvalidPassword    = errors.New("password must be between 12 and 128 characters")
	ErrInvalidName        = errors.New("name must be between 1 and 120 characters")
)

type User struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
}

type Session struct {
	ID            string
	Token         string
	CSRFToken     string
	CSRFTokenHash []byte
	ExpiresAt     time.Time
	User          User
}

type AuthenticatedSession struct {
	ID            string
	User          User
	CSRFTokenHash []byte
	ExpiresAt     time.Time
}

type Service struct {
	pool          *pgxpool.Pool
	sessionTTL    time.Duration
	queryTimeout  time.Duration
	dummyHash     string
	passwordSlots chan struct{}
}

func NewService(pool *pgxpool.Pool, sessionTTL, queryTimeout time.Duration) (*Service, error) {
	if pool == nil {
		return nil, errors.New("database pool is required")
	}
	if sessionTTL <= 0 {
		return nil, errors.New("session lifetime must be positive")
	}
	if queryTimeout <= 0 {
		return nil, errors.New("authentication query timeout must be positive")
	}
	dummy, err := HashPassword("internal-dummy-password-used-only-for-timing")
	if err != nil {
		return nil, fmt.Errorf("initialize authentication verifier: %w", err)
	}
	return &Service{pool: pool, sessionTTL: sessionTTL, queryTimeout: queryTimeout, dummyHash: dummy, passwordSlots: make(chan struct{}, 4)}, nil
}

func (s *Service) Register(ctx context.Context, email, password, displayName, rotateFrom string) (Session, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return Session{}, err
	}
	displayName = strings.TrimSpace(displayName)
	if len(displayName) < 1 || len(displayName) > 120 {
		return Session{}, ErrInvalidName
	}
	passwordHash, err := s.hashPassword(ctx, password)
	if err != nil {
		return Session{}, err
	}
	dbCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()
	tx, err := s.pool.Begin(dbCtx)
	if err != nil {
		return Session{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var user User
	if err := tx.QueryRow(dbCtx, `INSERT INTO users(email,password_hash,display_name) VALUES($1,$2,$3) RETURNING id::text,email,display_name`, email, passwordHash, displayName).Scan(&user.ID, &user.Email, &user.DisplayName); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Session{}, ErrEmailTaken
		}
		return Session{}, err
	}
	if _, err := tx.Exec(dbCtx, `INSERT INTO user_roles(user_id,role_id) SELECT $1,id FROM roles WHERE name='customer'`, user.ID); err != nil {
		return Session{}, err
	}
	if rotateFrom != "" {
		if _, err := tx.Exec(dbCtx, `UPDATE sessions SET revoked_at=now() WHERE id=$1 AND revoked_at IS NULL`, rotateFrom); err != nil {
			return Session{}, err
		}
	}
	session, err := s.issueSession(dbCtx, tx, user, rotateFrom)
	if err != nil {
		return Session{}, err
	}
	if err := tx.Commit(dbCtx); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s *Service) Login(ctx context.Context, email, password, rotateFrom string) (Session, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		_ = s.verifyPassword(ctx, s.dummyHash, password)
		return Session{}, ErrInvalidCredentials
	}
	if len(password) == 0 || len(password) > 128 {
		_ = s.verifyPassword(ctx, s.dummyHash, password)
		return Session{}, ErrInvalidCredentials
	}
	var user User
	var passwordHash *string
	var disabledAt *time.Time
	dbCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()
	err = s.pool.QueryRow(dbCtx, `SELECT id::text,email,display_name,password_hash,disabled_at FROM users WHERE lower(email)=lower($1)`, email).Scan(&user.ID, &user.Email, &user.DisplayName, &passwordHash, &disabledAt)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && (passwordHash == nil || !s.verifyPassword(ctx, *passwordHash, password) || disabledAt != nil) {
		if passwordHash == nil || err != nil {
			_ = s.verifyPassword(ctx, s.dummyHash, password)
		}
		return Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return Session{}, err
	}
	tx, err := s.pool.Begin(dbCtx)
	if err != nil {
		return Session{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if rotateFrom != "" {
		if _, err := tx.Exec(dbCtx, `UPDATE sessions SET revoked_at=now() WHERE id=$1 AND revoked_at IS NULL`, rotateFrom); err != nil {
			return Session{}, err
		}
	}
	session, err := s.issueSession(dbCtx, tx, user, rotateFrom)
	if err != nil {
		return Session{}, err
	}
	if err := tx.Commit(dbCtx); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s *Service) hashPassword(ctx context.Context, password string) (string, error) {
	select {
	case s.passwordSlots <- struct{}{}:
		defer func() { <-s.passwordSlots }()
		return HashPassword(password)
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (s *Service) verifyPassword(ctx context.Context, encoded, password string) bool {
	select {
	case s.passwordSlots <- struct{}{}:
		defer func() { <-s.passwordSlots }()
		return VerifyPassword(encoded, password)
	case <-ctx.Done():
		return false
	}
}

func (s *Service) Authenticate(ctx context.Context, token string) (AuthenticatedSession, error) {
	if token == "" || len(token) > 128 {
		return AuthenticatedSession{}, ErrUnauthenticated
	}
	hash := sha256.Sum256([]byte(token))
	dbCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()
	var session AuthenticatedSession
	err := s.pool.QueryRow(dbCtx, `SELECT s.id::text,u.id::text,u.email,u.display_name,s.csrf_token_hash,s.expires_at
		FROM sessions s JOIN users u ON u.id=s.user_id
		WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>now() AND u.disabled_at IS NULL`, hash[:]).Scan(
		&session.ID, &session.User.ID, &session.User.Email, &session.User.DisplayName, &session.CSRFTokenHash, &session.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return AuthenticatedSession{}, ErrUnauthenticated
	}
	return session, err
}

func (s *Service) HasRole(ctx context.Context, userID, role string) (bool, error) {
	dbCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()
	var allowed bool
	err := s.pool.QueryRow(dbCtx, `SELECT EXISTS (
		SELECT 1 FROM user_roles ur JOIN roles r ON r.id=ur.role_id
		WHERE ur.user_id=$1 AND r.name=$2
	)`, userID, role).Scan(&allowed)
	return allowed, err
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	hash := sha256.Sum256([]byte(token))
	dbCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()
	_, err := s.pool.Exec(dbCtx, `UPDATE sessions SET revoked_at=now() WHERE token_hash=$1 AND revoked_at IS NULL`, hash[:])
	return err
}

func (s *Service) RotateCSRF(ctx context.Context, sessionID string) (string, []byte, error) {
	token, err := randomToken()
	if err != nil {
		return "", nil, err
	}
	hash := sha256.Sum256([]byte(token))
	dbCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()
	result, err := s.pool.Exec(dbCtx, `UPDATE sessions SET csrf_token_hash=$2 WHERE id=$1 AND revoked_at IS NULL AND expires_at>now()`, sessionID, hash[:])
	if err != nil {
		return "", nil, err
	}
	if result.RowsAffected() != 1 {
		return "", nil, ErrUnauthenticated
	}
	return token, hash[:], nil
}

func (s *Service) issueSession(ctx context.Context, tx pgx.Tx, user User, rotateFrom string) (Session, error) {
	token, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	csrfToken, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	tokenHash := sha256.Sum256([]byte(token))
	csrfHash := sha256.Sum256([]byte(csrfToken))
	expiresAt := time.Now().UTC().Add(s.sessionTTL)
	var session Session
	err = tx.QueryRow(ctx, `INSERT INTO sessions(user_id,token_hash,csrf_token_hash,expires_at,rotated_from)
		VALUES($1,$2,$3,$4,NULLIF($5,'')::uuid) RETURNING id::text`, user.ID, tokenHash[:], csrfHash[:], expiresAt, rotateFrom).Scan(&session.ID)
	if err != nil {
		return Session{}, err
	}
	session.Token = token
	session.CSRFToken = csrfToken
	session.CSRFTokenHash = csrfHash[:]
	session.ExpiresAt = expiresAt
	session.User = user
	return session, nil
}

func normalizeEmail(email string) (string, error) {
	email = strings.TrimSpace(email)
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || len(email) > 254 || !strings.Contains(email, "@") {
		return "", ErrInvalidEmail
	}
	return strings.ToLower(email), nil
}

func randomToken() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func NewCSRFToken() (string, error) {
	return randomToken()
}
