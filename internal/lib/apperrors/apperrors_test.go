package apperrors

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsNotFound(t *testing.T) {
	if IsNotFound(nil) {
		t.Fatal("nil should not be not-found")
	}
	wrapped := fmt.Errorf("ctx: %w", ErrNotFound)
	if !IsNotFound(wrapped) {
		t.Fatal("wrapped ErrNotFound should match")
	}
	if IsNotFound(errors.New("other")) {
		t.Fatal("unrelated error should not match")
	}
}
