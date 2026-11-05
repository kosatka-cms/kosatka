package httputil

import (
	"errors"
	"fmt"
	"net"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"
)

func TestIsLikelyResponseWriteFailureExported(t *testing.T) {
	writeTimeout := &net.OpError{Op: "write", Err: errors.New("i/o timeout")}
	readTimeout := &net.OpError{Op: "read", Err: errors.New("i/o timeout")}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"write timeout op", writeTimeout, true},
		{"wrapped write timeout", fmt.Errorf("template: %w", writeTimeout), true},
		{"read timeout op", readTimeout, false},
		{"EPIPE", syscall.EPIPE, true},
		{"ECONNRESET", syscall.ECONNRESET, true},
		{"broken pipe string", errors.New("write tcp: broken pipe"), true},
		{"template error", errors.New(`template: list.html:12: undefined variable`), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsLikelyResponseWriteFailure(tt.err); got != tt.want {
				t.Errorf("IsLikelyResponseWriteFailure() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTemplateExecutionError(t *testing.T) {
	t.Run("writes html 500 without the error's text", func(t *testing.T) {
		rec := httptest.NewRecorder()
		err := errors.New(`html/template:header.html:51: no such template "x"`)
		TemplateExecutionError(rec, err)
		if rec.Code != 500 {
			t.Fatalf("status = %d, want 500", rec.Code)
		}
		ct := rec.Header().Get("Content-Type")
		if !strings.Contains(ct, "text/html") {
			t.Fatalf("Content-Type = %q, want text/html", ct)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "Something went wrong") {
			t.Fatalf("body missing user message: %q", body)
		}
		if strings.Contains(body, "no such template") || strings.Contains(body, "header.html") {
			t.Fatalf("body shows the error: %q", body)
		}
	})

	t.Run("nil err no-op", func(t *testing.T) {
		rec := httptest.NewRecorder()
		TemplateExecutionError(rec, nil)
		if rec.Code != 200 {
			t.Fatalf("expected default 200 / no write, got code %d body %q", rec.Code, rec.Body.String())
		}
	})

	t.Run("write failure no-op", func(t *testing.T) {
		rec := httptest.NewRecorder()
		TemplateExecutionError(rec, syscall.EPIPE)
		if rec.Code != 200 {
			t.Fatalf("expected no response for EPIPE, got %d", rec.Code)
		}
	})
}
