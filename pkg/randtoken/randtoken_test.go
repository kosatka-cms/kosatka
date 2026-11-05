package randtoken

import (
	"strings"
	"testing"
)

func TestGenerateLengthAndHex(t *testing.T) {
	token, err := Generate(32)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(token) != 64 {
		t.Fatalf("expected 64 hex chars for 32 bytes, got %d (%q)", len(token), token)
	}
	for _, r := range token {
		if !strings.ContainsRune("0123456789abcdef", r) {
			t.Fatalf("token contains non-hex character %q: %s", r, token)
		}
	}
}

func TestGenerateUnique(t *testing.T) {
	a, err := Generate(16)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := Generate(16)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a == b {
		t.Fatal("expected two generated tokens to differ")
	}
}

func TestGenerateZeroLength(t *testing.T) {
	token, err := Generate(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "" {
		t.Fatalf("expected empty token for n=0, got %q", token)
	}
}
