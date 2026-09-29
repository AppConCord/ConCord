// Package validation contains deterministic request validation built from govalidator primitives.
package validation

import (
	"strings"
	"unicode/utf8"

	"github.com/asaskevich/govalidator/v12"
)

// Username returns the first validation failure for a username, or an empty string when valid.
func Username(value string) string {
	if value == "" {
		return "This field is required"
	}
	length := utf8.RuneCountInString(value)
	if length < 4 {
		return "This value must be at least 4 characters long"
	}
	if length > 16 {
		return "This value must be at most 16 characters long"
	}
	if !govalidator.Matches(value, `^[a-z0-9_]+$`) {
		return "This value may contain only lowercase letters, numbers, and underscores"
	}
	return ""
}

// DisplayName returns the first validation failure for an optional display name.
func DisplayName(value *string) string {
	if value == nil {
		return ""
	}
	length := utf8.RuneCountInString(*value)
	if length < 2 {
		return "This value must be at least 2 characters long"
	}
	if length > 32 {
		return "This value must be at most 32 characters long"
	}
	return ""
}

// Password returns the first validation failure for a registration password.
func Password(value string) string {
	if value == "" {
		return "This field is required"
	}
	if utf8.RuneCountInString(value) != 8 {
		return "This value must be exactly 8 characters long"
	}
	if !govalidator.HasUpperCase(value) {
		return "This value must contain an uppercase letter"
	}
	if !govalidator.HasLowerCase(value) {
		return "This value must contain a lowercase letter"
	}
	if !govalidator.Matches(value, `[0-9]`) {
		return "This value must contain a number"
	}
	return ""
}

// MessageContent returns the first validation failure for message content.
func MessageContent(value string) string {
	if strings.TrimSpace(value) == "" {
		return "This field is required"
	}
	if utf8.RuneCountInString(value) > 2000 {
		return "This value must be at most 2000 characters long"
	}
	return ""
}
