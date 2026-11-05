package fallback

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestLog - a fallback's error: a missing record at debug level,
// anything else a warning, nil nothing.
func TestLog(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	Log(logger, nil, "nothing")
	Log(logger, fmt.Errorf("loading: %w", entity.ErrEntityNotFound), "missing")
	Log(logger, errors.New("db down"), "failed", "id", 3)
	Log(nil, errors.New("no logger"), "ignored")
	out := buf.String()
	if strings.Contains(out, "nothing") || !strings.Contains(out, "level=DEBUG msg=missing") || !strings.Contains(out, "level=WARN msg=failed id=3") {
		t.Errorf("log:\n%s", out)
	}
}
