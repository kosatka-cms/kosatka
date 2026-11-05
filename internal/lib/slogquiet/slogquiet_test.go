package slogquiet

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

func TestClientDisconnectsLowered(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))

	log.Error("loading field values", "error", fmt.Errorf("query: %w", context.Canceled))
	log.Error("rendering page", "error", errors.New("write tcp 127.0.0.1:3333->127.0.0.1:1: write: broken pipe"))
	log.With("component", "x").Error("real failure", "error", errors.New("table missing"))

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("lines: %q", lines)
	}
	for _, l := range lines[:2] {
		if !strings.Contains(l, "level=DEBUG") {
			t.Errorf("client disconnect not lowered: %s", l)
		}
	}
	if !strings.Contains(lines[2], "level=ERROR") || !strings.Contains(lines[2], "component=x") {
		t.Errorf("real error changed: %s", lines[2])
	}

	buf.Reset()
	info := slog.New(New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	info.Error("cancelled", "error", context.Canceled)
	if buf.Len() != 0 {
		t.Errorf("lowered record logged above DEBUG: %s", buf.String())
	}
}
