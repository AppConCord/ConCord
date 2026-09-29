package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"concord/backend/internal/apperror"
	"concord/backend/internal/auth"
	"concord/backend/internal/model"
)

type fakeAuthService struct {
	result        *auth.Result
	user          *model.User
	err           error
	authenticated string
	loggedOut     string
}

func (f *fakeAuthService) Register(context.Context, auth.RegisterInput) (*auth.Result, error) {
	return f.result, f.err
}

func (f *fakeAuthService) Login(context.Context, auth.LoginInput) (*auth.Result, error) {
	return f.result, f.err
}

func (f *fakeAuthService) Authenticate(_ context.Context, token string) (*model.User, error) {
	f.authenticated = token
	return f.user, f.err
}

func (f *fakeAuthService) Logout(_ context.Context, token string) error {
	f.loggedOut = token
	return f.err
}

type fakeMessageService struct {
	created *model.Message
	items   []model.Message
	hasMore bool
	err     error
}

func (f *fakeMessageService) Create(context.Context, *model.User, string) (*model.Message, error) {
	return f.created, f.err
}

func (f *fakeMessageService) List(context.Context, *int64, *int64, int) ([]model.Message, bool, error) {
	return f.items, f.hasMore, f.err
}

func TestRegisterSerializesStringIDsAndISOTimestamps(t *testing.T) {
	t.Parallel()
	user := &model.User{ID: 9_007_199_254_740_993, Username: "test_user", CreatedAt: 1_800_000_000_123, UpdatedAt: 1_800_000_000_123}
	authService := &fakeAuthService{result: &auth.Result{Token: "secret", User: user}}
	server := newTestServer(authService, &fakeMessageService{})

	response := performRequest(server, http.MethodPost, "/auth/register", `{"username":"test_user","password":"Pass1234"}`, "")
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		Token string       `json:"token"`
		User  userResponse `json:"user"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Token != "secret" || body.User.ID != "9007199254740993" {
		t.Fatalf("unexpected response: %#v", body)
	}
	if body.User.CreatedAt != "2027-01-15T08:00:00.123Z" {
		t.Fatalf("created_at = %q", body.User.CreatedAt)
	}
}

func TestProtectedRoutesUseBearerToken(t *testing.T) {
	t.Parallel()
	user := &model.User{ID: 1, Username: "test_user", CreatedAt: 1, UpdatedAt: 1}
	authService := &fakeAuthService{user: user}
	server := newTestServer(authService, &fakeMessageService{})

	response := performRequest(server, http.MethodGet, "/users/me", "", "Bearer opaque-token")
	if response.Code != http.StatusOK || authService.authenticated != "opaque-token" {
		t.Fatalf("status=%d token=%q body=%s", response.Code, authService.authenticated, response.Body.String())
	}
	response = performRequest(server, http.MethodPost, "/auth/logout", "", "Bearer opaque-token")
	if response.Code != http.StatusNoContent || authService.loggedOut != "opaque-token" {
		t.Fatalf("status=%d logout token=%q", response.Code, authService.loggedOut)
	}
	response = performRequest(server, http.MethodGet, "/users/me", "", "not-a-bearer")
	assertError(t, response, http.StatusUnauthorized, apperror.CodeUnauthorized)
}

func TestHTTPValidationAndMessageResponse(t *testing.T) {
	t.Parallel()
	user := &model.User{ID: 1, Username: "author", CreatedAt: 1, UpdatedAt: 1}
	message := model.Message{ID: 2, UserID: 1, Content: "hello", CreatedAt: 2, UpdatedAt: 2, Author: *user}
	authService := &fakeAuthService{user: user}
	messageService := &fakeMessageService{items: []model.Message{message}, hasMore: true, created: &message}
	server := newTestServer(authService, messageService)

	response := performRequest(server, http.MethodGet, "/messages?before=invalid", "", "Bearer token")
	assertError(t, response, http.StatusBadRequest, apperror.CodeValidation)
	response = performRequest(server, http.MethodGet, "/messages?limit=1", "", "Bearer token")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"id":"2"`) || !strings.Contains(response.Body.String(), `"has_more":true`) {
		t.Fatalf("unexpected list response: %d %s", response.Code, response.Body.String())
	}
	response = performRequest(server, http.MethodPost, "/messages", `{"content":"hello"}`, "Bearer token")
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), `"content":"hello"`) {
		t.Fatalf("unexpected create response: %d %s", response.Code, response.Body.String())
	}
}

func TestMalformedJSONAndUnknownRouteUseErrorContract(t *testing.T) {
	t.Parallel()
	server := newTestServer(&fakeAuthService{}, &fakeMessageService{})
	response := performRequest(server, http.MethodPost, "/auth/register", `{"unknown":true}`, "")
	assertError(t, response, http.StatusBadRequest, apperror.CodeInvalidRequest)
	response = performRequest(server, http.MethodGet, "/does-not-exist", "", "")
	assertError(t, response, http.StatusNotFound, apperror.CodeNotFound)
}

func newTestServer(authService AuthService, messageService MessageService) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(authService, messageService, logger).Handler()
}

func performRequest(handler http.Handler, method, target, body, authorization string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		request.Header.Set(echoHeaderContentType, "application/json")
	}
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

const echoHeaderContentType = "Content-Type"

func assertError(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, status, response.Body.String())
	}
	var body struct {
		Error struct {
			Code   string            `json:"code"`
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != code || body.Error.Fields == nil {
		t.Fatalf("unexpected error body: %#v", body)
	}
}
