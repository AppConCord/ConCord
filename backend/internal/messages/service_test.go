package messages

import (
	"context"
	"errors"
	"testing"
	"time"

	"concord/backend/internal/apperror"
	"concord/backend/internal/model"
)

type fixedIDs struct {
	id  int64
	err error
}

func (f fixedIDs) Next() (int64, error) { return f.id, f.err }

type fakeRepository struct {
	created *model.Message
	items   []model.Message
	hasMore bool
	err     error
}

func (f *fakeRepository) CreateMessage(_ context.Context, message *model.Message) error {
	f.created = message
	return f.err
}

func (f *fakeRepository) ListMessages(context.Context, *int64, *int64, int) ([]model.Message, bool, error) {
	return f.items, f.hasMore, f.err
}

func TestCreateMessage(t *testing.T) {
	t.Parallel()
	repository := &fakeRepository{}
	service := NewService(repository, fixedIDs{id: 123})
	fixed := time.Date(2026, time.September, 29, 1, 2, 3, 456_000_000, time.UTC)
	service.now = func() time.Time { return fixed }
	author := &model.User{ID: 99, Username: "author"}

	message, err := service.Create(context.Background(), author, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if message.ID != 123 || message.UserID != 99 || message.Author.Username != "author" {
		t.Fatalf("unexpected message: %#v", message)
	}
	if message.CreatedAt != fixed.UnixMilli() || repository.created != message {
		t.Fatal("message timestamps or persistence call are incorrect")
	}
}

func TestMessageValidationAndRepositoryErrors(t *testing.T) {
	t.Parallel()
	service := NewService(&fakeRepository{}, fixedIDs{id: 1})
	if _, err := service.Create(context.Background(), &model.User{}, "  "); appCode(err) != apperror.CodeValidation {
		t.Fatalf("validation error = %v", err)
	}
	service = NewService(&fakeRepository{err: errors.New("write failed")}, fixedIDs{id: 1})
	if _, err := service.Create(context.Background(), &model.User{}, "valid"); appCode(err) != apperror.CodeInternal {
		t.Fatalf("repository error = %v", err)
	}
}

func TestListMessages(t *testing.T) {
	t.Parallel()
	repository := &fakeRepository{items: []model.Message{{ID: 1}}, hasMore: true}
	service := NewService(repository, fixedIDs{})
	items, hasMore, err := service.List(context.Background(), nil, nil, DefaultLimit)
	if err != nil || len(items) != 1 || !hasMore {
		t.Fatalf("List() = (%v, %v, %v)", items, hasMore, err)
	}
	id := int64(1)
	if _, _, err := service.List(context.Background(), &id, &id, 1); appCode(err) != apperror.CodeValidation {
		t.Fatalf("combined cursor error = %v", err)
	}
	if _, _, err := service.List(context.Background(), nil, nil, MaximumLimit+1); appCode(err) != apperror.CodeValidation {
		t.Fatalf("limit error = %v", err)
	}
}

func appCode(err error) string {
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		return ""
	}
	return appErr.Code()
}
