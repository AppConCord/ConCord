package validation

import (
	"strings"
	"testing"
)

func TestUsername(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		value string
		want  string
	}{
		"valid":         {"user_01", ""},
		"required":      {"", "This field is required"},
		"short":         {"abc", "This value must be at least 4 characters long"},
		"long":          {strings.Repeat("a", 17), "This value must be at most 16 characters long"},
		"invalid chars": {"User", "This value may contain only lowercase letters, numbers, and underscores"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := Username(test.value); got != test.want {
				t.Fatalf("Username(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func TestDisplayName(t *testing.T) {
	t.Parallel()
	short, valid, long := "x", "🦆 Duck", strings.Repeat("x", 33)
	tests := []struct {
		value *string
		want  string
	}{
		{nil, ""},
		{&short, "This value must be at least 2 characters long"},
		{&valid, ""},
		{&long, "This value must be at most 32 characters long"},
	}
	for _, test := range tests {
		if got := DisplayName(test.value); got != test.want {
			t.Fatalf("DisplayName(%v) = %q, want %q", test.value, got, test.want)
		}
	}
}

func TestPassword(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"":         "This field is required",
		"Aa1":      "This value must be exactly 8 characters long",
		"abcdefg1": "This value must contain an uppercase letter",
		"ABCDEFG1": "This value must contain a lowercase letter",
		"Abcdefgh": "This value must contain a number",
		"Pass1234": "",
	}
	for value, want := range tests {
		if got := Password(value); got != want {
			t.Fatalf("Password(%q) = %q, want %q", value, got, want)
		}
	}
}

func TestMessageContent(t *testing.T) {
	t.Parallel()
	if got := MessageContent(" \n\t"); got != "This field is required" {
		t.Fatalf("whitespace error = %q", got)
	}
	if got := MessageContent(strings.Repeat("x", 2001)); got == "" {
		t.Fatal("expected oversized content to fail")
	}
	if got := MessageContent("hello"); got != "" {
		t.Fatalf("valid content failed: %q", got)
	}
}
