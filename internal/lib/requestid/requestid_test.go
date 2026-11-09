package requestid

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	a, b := New(), New()
	if len(a) != 16 || !Valid(a) || a == b {
		t.Errorf("New: %q, %q", a, b)
	}
}

func TestValid(t *testing.T) {
	for s, want := range map[string]bool{
		"abc-123_X.y":           true,
		"":                      false,
		"a b":                   false,
		"a\nb":                  false,
		"a\"b":                  false,
		strings.Repeat("a", 64): true,
		strings.Repeat("a", 65): false,
	} {
		if got := Valid(s); got != want {
			t.Errorf("Valid(%q) = %v, want %v", s, got, want)
		}
	}
}

func TestContext(t *testing.T) {
	if FromContext(context.Background()) != "" {
		t.Error("an id without one")
	}
	if got := FromContext(WithID(context.Background(), "x1")); got != "x1" {
		t.Errorf("got %q", got)
	}
}

func TestHandler(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(NewHandler(slog.NewTextHandler(&buf, nil))).With("site", "s")
	log.ErrorContext(WithID(context.Background(), "abc"), "failed")
	log.Error("plain")
	out := buf.String()
	if !strings.Contains(out, "msg=failed site=s request_id=abc") {
		t.Errorf("no request_id on the context's line:\n%s", out)
	}
	if strings.Contains(strings.Split(out, "\n")[1], "request_id") {
		t.Errorf("request_id without a context:\n%s", out)
	}
}
