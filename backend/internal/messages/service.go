// Package messages implements use cases for Concord's single global message feed.
package messages

import (
	"context"
	"fmt"
	"time"

	"concord/backend/internal/apperror"
	"concord/backend/internal/model"
	"concord/backend/internal/validation"
)

const (
	// DefaultLimit is used when a message list request does not specify a page size.
	DefaultLimit = 50
	// MaximumLimit bounds database work for one message list request.
	MaximumLimit = 100
)

// Repository describes persistence operations required by message use cases.
type Repository interface {
	CreateMessage(context.Context, *model.Message) error
	ListMessages(context.Context, *int64, *int64, int) ([]model.Message, bool, error)
}

// IDGenerator produces Snowflake message identifiers.
type IDGenerator interface {
	Next() (int64, error)
}

// Service creates and lists messages in the global feed.
type Service struct {
	repository Repository
	ids        IDGenerator
	now        func() time.Time
}

// NewService creates a global-message service.
func NewService(repository Repository, ids IDGenerator) *Service {
	return &Service{repository: repository, ids: ids, now: time.Now}
}

// Create validates and persists a new message for author.
func (s *Service) Create(ctx context.Context, author *model.User, content string) (*model.Message, error) {
	if detail := validation.MessageContent(content); detail != "" {
		return nil, apperror.Validation(map[string]string{"content": detail})
	}
	id, err := s.ids.Next()
	if err != nil {
		return nil, apperror.Internal(fmt.Errorf("generate message ID: %w", err))
	}
	now := s.now().UTC().UnixMilli()
	message := &model.Message{
		ID:        id,
		UserID:    author.ID,
		Content:   content,
		CreatedAt: now,
		UpdatedAt: now,
		Author:    *author,
	}
	if err := s.repository.CreateMessage(ctx, message); err != nil {
		return nil, apperror.Internal(fmt.Errorf("create message: %w", err))
	}
	return message, nil
}

// List returns messages oldest-to-newest and whether more exist in the requested direction.
func (s *Service) List(ctx context.Context, before, after *int64, limit int) ([]model.Message, bool, error) {
	if before != nil && after != nil {
		return nil, false, apperror.Validation(map[string]string{
			"before": "This parameter cannot be combined with after",
		})
	}
	if limit < 1 || limit > MaximumLimit {
		return nil, false, apperror.Validation(map[string]string{
			"limit": "This value must be between 1 and 100",
		})
	}
	items, hasMore, err := s.repository.ListMessages(ctx, before, after, limit)
	if err != nil {
		return nil, false, apperror.Internal(fmt.Errorf("list messages: %w", err))
	}
	return items, hasMore, nil
}
