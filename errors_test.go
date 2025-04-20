package web

import (
	"errors"
	"testing"
)

func TestForbiddenErr(t *testing.T) {
	cause := errors.New("something happened")
	err := ForbiddenErrf("permission denied: %w", cause)

	if err.Error() != "permission denied: something happened" {
		t.Errorf("want error message %q, got %q", "permission denied: something happened", err.Error())
	}

	if !errors.Is(err, cause) {
		t.Error("want error to wrap cause, but it did not")
	}
}
