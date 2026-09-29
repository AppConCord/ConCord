package store

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"

	"concord/backend/internal/database"
	"concord/backend/internal/domain"
	"concord/backend/internal/model"
)

func TestStoreAgainstSQLite(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	if err := db.AutoMigrate(&model.User{}, &model.Session{}, &model.Message{}); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	persistence := New(db)
	ctx := context.Background()
	tokenHash := bytes.Repeat([]byte{9}, 32)
	user := &model.User{ID: 1, Username: "test_user", PasswordHash: "hash", CreatedAt: 100, UpdatedAt: 100}
	session := &model.Session{
		ID: 2, UserID: user.ID, TokenHash: tokenHash, ExpiresAt: 1000, CreatedAt: 100, UpdatedAt: 100,
	}
	if err := persistence.CreateUserAndSession(ctx, user, session); err != nil {
		t.Fatal(err)
	}
	duplicate := *user
	duplicate.ID = 3
	duplicateSession := *session
	duplicateSession.ID, duplicateSession.UserID = 4, 3
	if err := persistence.CreateUserAndSession(ctx, &duplicate, &duplicateSession); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate error = %v, want domain.ErrConflict", err)
	}

	byUsername, err := persistence.UserByUsername(ctx, user.Username)
	if err != nil || byUsername.ID != user.ID {
		t.Fatalf("UserByUsername() = (%v, %v)", byUsername, err)
	}
	byToken, err := persistence.UserByTokenHash(ctx, tokenHash, 999)
	if err != nil || byToken.ID != user.ID {
		t.Fatalf("UserByTokenHash() = (%v, %v)", byToken, err)
	}
	if _, err := persistence.UserByTokenHash(ctx, tokenHash, 1000); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expired token error = %v", err)
	}

	for id := int64(10); id <= 12; id++ {
		message := &model.Message{ID: id, UserID: user.ID, Content: "message", CreatedAt: id, UpdatedAt: id}
		if err := persistence.CreateMessage(ctx, message); err != nil {
			t.Fatal(err)
		}
	}
	items, hasMore, err := persistence.ListMessages(ctx, nil, nil, 2)
	if err != nil || !hasMore || len(items) != 2 || items[0].ID != 11 || items[1].ID != 12 {
		t.Fatalf("latest messages = (%v, %v, %v)", items, hasMore, err)
	}
	before := int64(12)
	items, hasMore, err = persistence.ListMessages(ctx, &before, nil, 2)
	if err != nil || hasMore || len(items) != 2 || items[0].ID != 10 || items[1].ID != 11 {
		t.Fatalf("older messages = (%v, %v, %v)", items, hasMore, err)
	}
	after := int64(10)
	items, hasMore, err = persistence.ListMessages(ctx, nil, &after, 1)
	if err != nil || !hasMore || len(items) != 1 || items[0].ID != 11 {
		t.Fatalf("newer messages = (%v, %v, %v)", items, hasMore, err)
	}
	if items[0].Author.Username != user.Username {
		t.Fatal("message author was not preloaded")
	}

	if err := persistence.DeleteSessionByTokenHash(ctx, tokenHash); err != nil {
		t.Fatal(err)
	}
	if _, err := persistence.UserByTokenHash(ctx, tokenHash, 0); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("revoked session error = %v", err)
	}
}
