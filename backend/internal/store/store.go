// Package store implements application persistence ports with GORM.
package store

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"concord/backend/internal/domain"
	"concord/backend/internal/model"
	"gorm.io/gorm"
)

// Store persists Concord entities through GORM.
type Store struct {
	db *gorm.DB
}

// New creates a GORM-backed store.
func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

// CreateUserAndSession atomically persists a user and their initial session.
func (s *Store) CreateUserAndSession(ctx context.Context, user *model.User, session *model.Session) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		return tx.Create(session).Error
	})
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return domain.ErrConflict
	}
	if err != nil {
		return fmt.Errorf("create user and session transaction: %w", err)
	}
	return nil
}

// CreateSession persists an additional bearer session.
func (s *Store) CreateSession(ctx context.Context, session *model.Session) error {
	if err := s.db.WithContext(ctx).Create(session).Error; err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// DeleteSessionByTokenHash permanently revokes sessions matching tokenHash.
func (s *Store) DeleteSessionByTokenHash(ctx context.Context, tokenHash []byte) error {
	if err := s.db.WithContext(ctx).Where("token_hash = ?", tokenHash).Delete(&model.Session{}).Error; err != nil {
		return fmt.Errorf("delete session by token hash: %w", err)
	}
	return nil
}

// UserByTokenHash returns the user attached to an unexpired session.
func (s *Store) UserByTokenHash(ctx context.Context, tokenHash []byte, nowMillis int64) (*model.User, error) {
	var user model.User
	err := s.db.WithContext(ctx).
		Table("users").
		Select("users.*").
		Joins("JOIN sessions ON sessions.user_id = users.id").
		Where("sessions.token_hash = ? AND sessions.expires_at > ?", tokenHash, nowMillis).
		First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user by token hash: %w", err)
	}
	return &user, nil
}

// UserByUsername returns a user with its password hash for credential verification.
func (s *Store) UserByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	err := s.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user by username: %w", err)
	}
	return &user, nil
}

// CreateMessage persists one global-feed message.
func (s *Store) CreateMessage(ctx context.Context, message *model.Message) error {
	if err := s.db.WithContext(ctx).Omit("Author").Create(message).Error; err != nil {
		return fmt.Errorf("create message: %w", err)
	}
	return nil
}

// ListMessages returns a bounded page with authors preloaded and chronological output ordering.
func (s *Store) ListMessages(
	ctx context.Context,
	before *int64,
	after *int64,
	limit int,
) ([]model.Message, bool, error) {
	query := s.db.WithContext(ctx).Model(&model.Message{}).Preload("Author").Limit(limit + 1)
	ascendingQuery := after != nil
	switch {
	case before != nil:
		query = query.Where("messages.id < ?", *before).Order("messages.id DESC")
	case after != nil:
		query = query.Where("messages.id > ?", *after).Order("messages.id ASC")
	default:
		query = query.Order("messages.id DESC")
	}

	var items []model.Message
	if err := query.Find(&items).Error; err != nil {
		return nil, false, fmt.Errorf("query messages: %w", err)
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	if !ascendingQuery {
		slices.Reverse(items)
	}
	return items, hasMore, nil
}
