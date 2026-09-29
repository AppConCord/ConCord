// Package httpapi adapts Echo HTTP requests and responses to Concord application services.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"concord/backend/internal/apperror"
	"concord/backend/internal/auth"
	"concord/backend/internal/messages"
	"concord/backend/internal/model"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

const (
	authTokenContextKey = "concord.auth_token"
	currentUserKey      = "concord.current_user"
	maxRequestBodyBytes = 64 * 1024
)

// AuthService describes authentication operations exposed to the HTTP adapter.
type AuthService interface {
	Register(context.Context, auth.RegisterInput) (*auth.Result, error)
	Login(context.Context, auth.LoginInput) (*auth.Result, error)
	Authenticate(context.Context, string) (*model.User, error)
	Logout(context.Context, string) error
}

// MessageService describes global-message operations exposed to the HTTP adapter.
type MessageService interface {
	Create(context.Context, *model.User, string) (*model.Message, error)
	List(context.Context, *int64, *int64, int) ([]model.Message, bool, error)
}

// Server owns the Echo router and translates HTTP concerns into application calls.
type Server struct {
	echo     *echo.Echo
	auth     AuthService
	messages MessageService
	logger   *slog.Logger
}

// New creates a fully configured Echo server and registers all API routes.
func New(authService AuthService, messageService MessageService, logger *slog.Logger) *Server {
	e := echo.New()
	e.Logger = logger
	server := &Server{echo: e, auth: authService, messages: messageService, logger: logger}
	e.HTTPErrorHandler = server.handleError
	e.Use(middleware.RequestID())
	e.Use(middleware.RecoverWithConfig(middleware.RecoverConfig{DisablePrintStack: true}))
	e.Use(server.requestLogger())

	e.POST("/auth/register", server.register)
	e.POST("/auth/login", server.login)

	e.POST("/auth/logout", server.logout, server.requireAuthentication)
	e.GET("/users/me", server.me, server.requireAuthentication)
	e.GET("/messages", server.listMessages, server.requireAuthentication)
	e.POST("/messages", server.createMessage, server.requireAuthentication)

	return server
}

// Handler returns the configured Echo instance as an HTTP handler.
func (s *Server) Handler() *echo.Echo {
	return s.echo
}

func (s *Server) register(c *echo.Context) error {
	var request struct {
		Username    string  `json:"username"`
		DisplayName *string `json:"display_name"`
		Password    string  `json:"password"`
	}
	if err := decodeJSON(c, &request); err != nil {
		return err
	}
	result, err := s.auth.Register(c.Request().Context(), auth.RegisterInput{
		Username: request.Username, DisplayName: request.DisplayName, Password: request.Password,
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, map[string]any{
		"token": result.Token,
		"user":  newUserResponse(result.User),
	})
}

func (s *Server) login(c *echo.Context) error {
	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(c, &request); err != nil {
		return err
	}
	result, err := s.auth.Login(c.Request().Context(), auth.LoginInput(request))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{
		"token": result.Token,
		"user":  newUserResponse(result.User),
	})
}

func (s *Server) logout(c *echo.Context) error {
	token, _ := c.Get(authTokenContextKey).(string)
	if err := s.auth.Logout(c.Request().Context(), token); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (s *Server) me(c *echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, newUserResponse(user))
}

func (s *Server) createMessage(c *echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return err
	}
	var request struct {
		Content string `json:"content"`
	}
	if err := decodeJSON(c, &request); err != nil {
		return err
	}
	message, err := s.messages.Create(c.Request().Context(), user, request.Content)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, newMessageResponse(message))
}

func (s *Server) listMessages(c *echo.Context) error {
	before, err := parseOptionalID(c.QueryParam("before"), "before")
	if err != nil {
		return err
	}
	after, err := parseOptionalID(c.QueryParam("after"), "after")
	if err != nil {
		return err
	}
	limit := messages.DefaultLimit
	if raw := c.QueryParam("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil {
			return apperror.Validation(map[string]string{"limit": "This value must be an integer"})
		}
	}
	items, hasMore, err := s.messages.List(c.Request().Context(), before, after, limit)
	if err != nil {
		return err
	}
	responses := make([]messageResponse, len(items))
	for index := range items {
		responses[index] = newMessageResponse(&items[index])
	}
	return c.JSON(http.StatusOK, map[string]any{"messages": responses, "has_more": hasMore})
}

func (s *Server) requireAuthentication(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		token, err := bearerToken(c.Request().Header.Get(echo.HeaderAuthorization))
		if err != nil {
			return err
		}
		user, err := s.auth.Authenticate(c.Request().Context(), token)
		if err != nil {
			return err
		}
		c.Set(authTokenContextKey, token)
		c.Set(currentUserKey, user)
		return next(c)
	}
}

func (s *Server) handleError(c *echo.Context, err error) {
	response, _ := echo.UnwrapResponse(c.Response())
	if response != nil && response.Committed {
		return
	}
	appErr := normalizeError(err)
	if appErr.Status() >= http.StatusInternalServerError {
		s.logger.ErrorContext(c.Request().Context(), "request failed", "error", err)
	}
	body := map[string]any{"error": map[string]any{
		"code": appErr.Code(), "fields": appErr.Fields(),
	}}
	if writeErr := c.JSON(appErr.Status(), body); writeErr != nil {
		s.logger.ErrorContext(c.Request().Context(), "write error response", "error", errors.Join(err, writeErr))
	}
}

func (s *Server) requestLogger() echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogLatency: true, LogMethod: true, LogURI: true, LogStatus: true,
		LogRemoteIP: true, LogRequestID: true,
		LogValuesFunc: func(c *echo.Context, values middleware.RequestLoggerValues) error {
			attributes := []any{
				"method", values.Method, "uri", values.URI, "status", values.Status,
				"latency", values.Latency, "remote_ip", values.RemoteIP, "request_id", values.RequestID,
			}
			if values.Error != nil {
				s.logger.WarnContext(c.Request().Context(), "http request", append(attributes, "error", values.Error)...)
			} else {
				s.logger.InfoContext(c.Request().Context(), "http request", attributes...)
			}
			return nil
		},
	})
}

func decodeJSON(c *echo.Context, destination any) error {
	request := c.Request()
	request.Body = http.MaxBytesReader(c.Response(), request.Body, maxRequestBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return apperror.InvalidRequest(fmt.Errorf("decode request body: %w", err))
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return apperror.InvalidRequest(errors.New("request body must contain exactly one JSON object"))
	}
	return nil
}

func bearerToken(header string) (string, error) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", apperror.Unauthorized(nil)
	}
	return parts[1], nil
}

func currentUser(c *echo.Context) (*model.User, error) {
	user, ok := c.Get(currentUserKey).(*model.User)
	if !ok || user == nil {
		return nil, apperror.Unauthorized(nil)
	}
	return user, nil
}

func parseOptionalID(raw, field string) (*int64, error) {
	if raw == "" {
		return nil, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return nil, apperror.Validation(map[string]string{field: "This value must be a positive int64 string"})
	}
	return &id, nil
}

func normalizeError(err error) *apperror.Error {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr
	}
	status := echo.StatusCode(err)
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusRequestEntityTooLarge, http.StatusUnsupportedMediaType:
		return apperror.New(status, apperror.CodeInvalidRequest, nil, err)
	case http.StatusUnauthorized:
		return apperror.Unauthorized(err)
	case http.StatusNotFound:
		return apperror.NotFound(err)
	default:
		return apperror.Internal(err)
	}
}
