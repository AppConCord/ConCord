package apperror

import (
	"errors"
	"net/http"
	"testing"
)

func TestErrorPreservesCauseAndDefensivelyCopiesFields(t *testing.T) {
	t.Parallel()
	cause := errors.New("database exploded")
	fields := map[string]string{"username": "invalid"}
	err := New(http.StatusBadRequest, CodeValidation, fields, cause)
	fields["username"] = "mutated"
	returned := err.Fields()
	returned["username"] = "also mutated"
	if err.Fields()["username"] != "invalid" {
		t.Fatal("fields were not defensively copied")
	}
	if !errors.Is(err, cause) || err.StatusCode() != http.StatusBadRequest || err.Code() != CodeValidation {
		t.Fatalf("unexpected error metadata: %#v", err)
	}
}

func TestAsWrapsUnknownError(t *testing.T) {
	t.Parallel()
	cause := errors.New("unknown")
	converted := As(cause)
	if converted.Code() != CodeInternal || !errors.Is(converted, cause) {
		t.Fatalf("unexpected conversion: %#v", converted)
	}
	existing := Validation(map[string]string{"field": "bad"})
	if As(existing) != existing {
		t.Fatal("existing application error was replaced")
	}
}
