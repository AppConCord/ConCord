// Package apperror defines transport-independent errors returned by application services.
package apperror

import (
	"errors"
	"net/http"
)

const (
	// CodeConflict identifies a request that conflicts with existing state.
	CodeConflict = "conflict"
	// CodeInternal identifies an unexpected server-side failure.
	CodeInternal = "internal_error"
	// CodeInvalidRequest identifies a malformed request.
	CodeInvalidRequest = "invalid_request"
	// CodeNotFound identifies a resource that does not exist.
	CodeNotFound = "not_found"
	// CodeUnauthorized identifies absent or invalid authentication.
	CodeUnauthorized = "unauthorized"
	// CodeValidation identifies field-level validation failures.
	CodeValidation = "validation_error"
)

// Error carries the stable API code, HTTP status, field errors, and internal cause for a failure.
// The underlying cause is intentionally excluded from JSON responses by the HTTP adapter.
type Error struct {
	status int
	code   string
	fields map[string]string
	cause  error
}

// New creates an application error. Callers should prefer the specialized constructors below.
func New(status int, code string, fields map[string]string, cause error) *Error {
	return &Error{status: status, code: code, fields: cloneFields(fields), cause: cause}
}

// Conflict creates a conflict error associated with a single request field.
func Conflict(field, detail string, cause error) *Error {
	fields := map[string]string{}
	if field != "" && detail != "" {
		fields[field] = detail
	}
	return New(http.StatusConflict, CodeConflict, fields, cause)
}

// Internal wraps an unexpected failure without exposing its details to clients.
func Internal(cause error) *Error {
	return New(http.StatusInternalServerError, CodeInternal, nil, cause)
}

// InvalidRequest creates an error for malformed input that cannot be bound or parsed.
func InvalidRequest(cause error) *Error {
	return New(http.StatusBadRequest, CodeInvalidRequest, nil, cause)
}

// NotFound creates an error for a missing resource.
func NotFound(cause error) *Error {
	return New(http.StatusNotFound, CodeNotFound, nil, cause)
}

// Unauthorized creates an error for missing, expired, or otherwise invalid credentials.
func Unauthorized(cause error) *Error {
	return New(http.StatusUnauthorized, CodeUnauthorized, nil, cause)
}

// Validation creates a field validation error. The map contains at most one message per field.
func Validation(fields map[string]string) *Error {
	return New(http.StatusBadRequest, CodeValidation, fields, nil)
}

// Error implements the error interface without revealing the wrapped cause.
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	return e.code
}

// Unwrap exposes the internal cause to errors.Is and errors.As.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Status returns the HTTP status assigned to the error.
func (e *Error) Status() int {
	if e == nil || e.status == 0 {
		return http.StatusInternalServerError
	}
	return e.status
}

// StatusCode lets Echo and its middleware observe the intended response status.
func (e *Error) StatusCode() int {
	return e.Status()
}

// Code returns the stable, machine-readable error code.
func (e *Error) Code() string {
	if e == nil || e.code == "" {
		return CodeInternal
	}
	return e.code
}

// Fields returns a defensive copy of the field validation details.
func (e *Error) Fields() map[string]string {
	if e == nil {
		return map[string]string{}
	}
	return cloneFields(e.fields)
}

// As returns err as an application error or converts unknown errors to an internal error.
func As(err error) *Error {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	return Internal(err)
}

func cloneFields(fields map[string]string) map[string]string {
	cloned := make(map[string]string, len(fields))
	for key, value := range fields {
		cloned[key] = value
	}
	return cloned
}
