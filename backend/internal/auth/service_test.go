package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"concord/backend/internal/apperror"
	"concord/backend/internal/domain"
	"concord/backend/internal/model"
	"golang.org/x/crypto/bcrypt"
)

type fakeIDs struct{ next int64 }

func (f *fakeIDs) Next() (int64, error) {
	f.next++
	return f.next, nil
}

type fakeRepository struct {
	createdUser    *model.User
	createdSession *model.Session
	user           *model.User
	findErr        error
	createErr      error
	deletedHash    []byte
	lookupHash     []byte
	lookupNow      int64
}

func (f *fakeRepository) CreateUserAndSession(_ context.Context, user *model.User, session *model.Session) error {
	f.createdUser, f.createdSession = user, session
	return f.createErr
}

func (f *fakeRepository) CreateSession(_ context.Context, session *model.Session) error {
	f.createdSession = session
	return f.createErr
}

func (f *fakeRepository) DeleteSessionByTokenHash(_ context.Context, hash []byte) error {
	f.deletedHash = append([]byte(nil), hash...)
	return f.createErr
}

func (f *fakeRepository) UserByTokenHash(_ context.Context, hash []byte, now int64) (*model.User, error) {
	f.lookupHash, f.lookupNow = append([]byte(nil), hash...), now
	return f.user, f.findErr
}

func (f *fakeRepository) UserByUsername(context.Context, string) (*model.User, error) {
	return f.user, f.findErr
}

func TestRegisterCreatesHashedUserAndSession(t *testing.T) {
	t.Parallel()
	repository := &fakeRepository{}
	service := NewService(repository, &fakeIDs{next: 40}, bcrypt.MinCost, time.Hour)
	fixed := time.Date(2026, time.September, 29, 12, 0, 0, 123_000_000, time.UTC)
	service.now = func() time.Time { return fixed }
	service.random = bytes.NewReader(bytes.Repeat([]byte{7}, tokenBytes))
	displayName := "Leonardo"

	result, err := service.Register(context.Background(), RegisterInput{
		Username: "leonardo_01", DisplayName: &displayName, Password: "Pass1234",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.User.ID != 41 || repository.createdSession.ID != 42 {
		t.Fatalf("unexpected generated IDs: user=%d session=%d", result.User.ID, repository.createdSession.ID)
	}
	if result.User.CreatedAt != fixed.UnixMilli() || repository.createdSession.ExpiresAt != fixed.Add(time.Hour).UnixMilli() {
		t.Fatal("timestamps were not stored as epoch milliseconds")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(result.User.PasswordHash), []byte("Pass1234")); err != nil {
		t.Fatalf("password was not hashed correctly: %v", err)
	}
	wantHash := sha256.Sum256([]byte(result.Token))
	if !bytes.Equal(repository.createdSession.TokenHash, wantHash[:]) {
		t.Fatal("session did not store the bearer token hash")
	}
}

func TestRegisterReportsValidationAndConflict(t *testing.T) {
	t.Parallel()
	service := NewService(&fakeRepository{}, &fakeIDs{}, bcrypt.MinCost, time.Hour)
	if _, err := service.Register(context.Background(), RegisterInput{}); errorCode(err) != apperror.CodeValidation {
		t.Fatalf("validation error code = %q", errorCode(err))
	}
	repository := &fakeRepository{createErr: domain.ErrConflict}
	service = NewService(repository, &fakeIDs{}, bcrypt.MinCost, time.Hour)
	service.random = bytes.NewReader(make([]byte, tokenBytes))
	_, err := service.Register(context.Background(), RegisterInput{Username: "valid_1", Password: "Pass1234"})
	if errorCode(err) != apperror.CodeConflict {
		t.Fatalf("conflict error code = %q", errorCode(err))
	}
}

func TestLoginAuthenticateAndLogout(t *testing.T) {
	t.Parallel()
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("Pass1234"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	user := &model.User{ID: 99, Username: "leonardo", PasswordHash: string(passwordHash)}
	repository := &fakeRepository{user: user}
	service := NewService(repository, &fakeIDs{next: 100}, bcrypt.MinCost, time.Hour)
	fixed := time.UnixMilli(1_800_000_000_123)
	service.now = func() time.Time { return fixed }
	service.random = bytes.NewReader(bytes.Repeat([]byte{3}, tokenBytes))

	result, err := service.Login(context.Background(), LoginInput{Username: "leonardo", Password: "Pass1234"})
	if err != nil {
		t.Fatal(err)
	}
	if result.User != user || repository.createdSession.UserID != user.ID {
		t.Fatal("login did not create a session for the authenticated user")
	}
	repository.user = user
	authenticated, err := service.Authenticate(context.Background(), result.Token)
	if err != nil || authenticated != user {
		t.Fatalf("Authenticate() = (%v, %v)", authenticated, err)
	}
	if repository.lookupNow != fixed.UnixMilli() {
		t.Fatal("authentication did not use current epoch milliseconds")
	}
	if err := service.Logout(context.Background(), result.Token); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(repository.deletedHash, repository.lookupHash) {
		t.Fatal("logout and authentication hashed the token differently")
	}
}

func TestLoginRejectsCredentials(t *testing.T) {
	t.Parallel()
	service := NewService(&fakeRepository{findErr: domain.ErrNotFound}, &fakeIDs{}, bcrypt.MinCost, time.Hour)
	if _, err := service.Login(context.Background(), LoginInput{}); errorCode(err) != apperror.CodeValidation {
		t.Fatalf("empty login error = %v", err)
	}
	if _, err := service.Login(context.Background(), LoginInput{Username: "missing", Password: "Pass1234"}); errorCode(err) != apperror.CodeUnauthorized {
		t.Fatalf("missing user error = %v", err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("Pass1234"), bcrypt.MinCost)
	service = NewService(&fakeRepository{user: &model.User{PasswordHash: string(hash)}}, &fakeIDs{}, bcrypt.MinCost, time.Hour)
	if _, err := service.Login(context.Background(), LoginInput{Username: "user", Password: "Nope1234"}); errorCode(err) != apperror.CodeUnauthorized {
		t.Fatalf("wrong password error = %v", err)
	}
	service = NewService(&fakeRepository{user: &model.User{PasswordHash: "corrupt"}}, &fakeIDs{}, bcrypt.MinCost, time.Hour)
	if _, err := service.Login(context.Background(), LoginInput{Username: "user", Password: "Pass1234"}); errorCode(err) != apperror.CodeInternal {
		t.Fatalf("corrupt password hash error = %v", err)
	}
}

func TestAuthenticateMapsMissingSession(t *testing.T) {
	t.Parallel()
	service := NewService(&fakeRepository{findErr: domain.ErrNotFound}, &fakeIDs{}, bcrypt.MinCost, time.Hour)
	if _, err := service.Authenticate(context.Background(), "token"); errorCode(err) != apperror.CodeUnauthorized {
		t.Fatalf("error = %v", err)
	}
	if err := service.Logout(context.Background(), ""); errorCode(err) != apperror.CodeUnauthorized {
		t.Fatalf("empty logout error = %v", err)
	}
}

func errorCode(err error) string {
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		return ""
	}
	return appErr.Code()
}
