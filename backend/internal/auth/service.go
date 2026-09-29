// Package auth implements registration, login, bearer-session authentication, and logout.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"time"

	"concord/backend/internal/apperror"
	"concord/backend/internal/domain"
	"concord/backend/internal/model"
	"concord/backend/internal/validation"
	"golang.org/x/crypto/bcrypt"
)

const tokenBytes = 32

// Repository describes the persistence operations required by authentication use cases.
type Repository interface {
	CreateUserAndSession(context.Context, *model.User, *model.Session) error
	CreateSession(context.Context, *model.Session) error
	DeleteSessionByTokenHash(context.Context, []byte) error
	UserByTokenHash(context.Context, []byte, int64) (*model.User, error)
	UserByUsername(context.Context, string) (*model.User, error)
}

// IDGenerator produces public and internal Snowflake identifiers.
type IDGenerator interface {
	Next() (int64, error)
}

// RegisterInput contains values accepted by account registration.
type RegisterInput struct {
	Username    string
	DisplayName *string
	Password    string
}

// LoginInput contains credentials accepted by login.
type LoginInput struct {
	Username string
	Password string
}

// Result is returned after registration or login and contains the one-time raw bearer token.
type Result struct {
	Token string
	User  *model.User
}

// Service coordinates password hashing and opaque session lifecycle operations.
type Service struct {
	repository Repository
	ids        IDGenerator
	bcryptCost int
	sessionTTL time.Duration
	now        func() time.Time
	random     io.Reader
}

// NewService creates an authentication service with cryptographically secure token generation.
func NewService(repository Repository, ids IDGenerator, bcryptCost int, sessionTTL time.Duration) *Service {
	return &Service{
		repository: repository,
		ids:        ids,
		bcryptCost: bcryptCost,
		sessionTTL: sessionTTL,
		now:        time.Now,
		random:     rand.Reader,
	}
}

// Register validates credentials and atomically creates a user and initial session.
func (s *Service) Register(ctx context.Context, input RegisterInput) (*Result, error) {
	if fields := validateRegistration(input); len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), s.bcryptCost)
	if err != nil {
		return nil, apperror.Internal(fmt.Errorf("hash password: %w", err))
	}
	userID, err := s.ids.Next()
	if err != nil {
		return nil, apperror.Internal(fmt.Errorf("generate user ID: %w", err))
	}

	now := s.now().UTC()
	user := &model.User{
		ID:           userID,
		Username:     input.Username,
		DisplayName:  input.DisplayName,
		PasswordHash: string(passwordHash),
		CreatedAt:    now.UnixMilli(),
		UpdatedAt:    now.UnixMilli(),
	}
	session, token, err := s.newSession(user.ID, now)
	if err != nil {
		return nil, err
	}
	if err := s.repository.CreateUserAndSession(ctx, user, session); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return nil, apperror.Conflict("username", "This username is already taken", err)
		}
		return nil, apperror.Internal(fmt.Errorf("create user and session: %w", err))
	}

	return &Result{Token: token, User: user}, nil
}

// Login verifies credentials and creates a new independent bearer session.
func (s *Service) Login(ctx context.Context, input LoginInput) (*Result, error) {
	fields := make(map[string]string)
	if input.Username == "" {
		fields["username"] = "This field is required"
	}
	if input.Password == "" {
		fields["password"] = "This field is required"
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}

	user, err := s.repository.UserByUsername(ctx, input.Username)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, apperror.Unauthorized(err)
		}
		return nil, apperror.Internal(fmt.Errorf("find user for login: %w", err))
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return nil, apperror.Unauthorized(err)
		}
		return nil, apperror.Internal(fmt.Errorf("verify password hash: %w", err))
	}

	now := s.now().UTC()
	session, token, err := s.newSession(user.ID, now)
	if err != nil {
		return nil, err
	}
	if err := s.repository.CreateSession(ctx, session); err != nil {
		return nil, apperror.Internal(fmt.Errorf("create login session: %w", err))
	}
	return &Result{Token: token, User: user}, nil
}

// Authenticate resolves an unexpired raw bearer token to its user.
func (s *Service) Authenticate(ctx context.Context, token string) (*model.User, error) {
	if token == "" {
		return nil, apperror.Unauthorized(nil)
	}
	hash := sha256.Sum256([]byte(token))
	user, err := s.repository.UserByTokenHash(ctx, hash[:], s.now().UTC().UnixMilli())
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, apperror.Unauthorized(err)
		}
		return nil, apperror.Internal(fmt.Errorf("authenticate session: %w", err))
	}
	return user, nil
}

// Logout revokes the supplied bearer token. Revoking a missing token is intentionally idempotent.
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return apperror.Unauthorized(nil)
	}
	hash := sha256.Sum256([]byte(token))
	if err := s.repository.DeleteSessionByTokenHash(ctx, hash[:]); err != nil {
		return apperror.Internal(fmt.Errorf("delete session: %w", err))
	}
	return nil
}

func (s *Service) newSession(userID int64, now time.Time) (*model.Session, string, error) {
	sessionID, err := s.ids.Next()
	if err != nil {
		return nil, "", apperror.Internal(fmt.Errorf("generate session ID: %w", err))
	}
	raw := make([]byte, tokenBytes)
	if _, err := io.ReadFull(s.random, raw); err != nil {
		return nil, "", apperror.Internal(fmt.Errorf("generate session token: %w", err))
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	return &model.Session{
		ID:        sessionID,
		UserID:    userID,
		TokenHash: hash[:],
		ExpiresAt: now.Add(s.sessionTTL).UnixMilli(),
		CreatedAt: now.UnixMilli(),
		UpdatedAt: now.UnixMilli(),
	}, token, nil
}

func validateRegistration(input RegisterInput) map[string]string {
	fields := make(map[string]string)
	if detail := validation.Username(input.Username); detail != "" {
		fields["username"] = detail
	}
	if detail := validation.DisplayName(input.DisplayName); detail != "" {
		fields["display_name"] = detail
	}
	if detail := validation.Password(input.Password); detail != "" {
		fields["password"] = detail
	}
	return fields
}
