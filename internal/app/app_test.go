package app

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/config"
	"github.com/kosatka-cms/kosatka/internal/lib/requestid"
)

// TestSetupLoggerLevel - a development machine (env local) logs DEBUG; a
// live site logs from INFO up.
func TestSetupLoggerLevel(t *testing.T) {
	ctx := context.Background()
	for _, local := range []bool{true, false} {
		log := setupLogger(local, config.LogFormatText)
		if got := log.Enabled(ctx, slog.LevelDebug); got != local {
			t.Errorf("local %v: DEBUG enabled %v", local, got)
		}
		if !log.Enabled(ctx, slog.LevelInfo) {
			t.Errorf("local %v: INFO disabled", local)
		}
	}
}

// TestLogFormat - text lines by default, a JSON object a line with format
// json; a line written with a request's context carries its request_id.
func TestLogFormat(t *testing.T) {
	ctx := requestid.WithID(context.Background(), "r1")
	var text bytes.Buffer
	slog.New(newLogHandler(&text, false, config.LogFormatText)).InfoContext(ctx, "hello", "n", 1)
	if got := text.String(); !strings.Contains(got, "level=INFO msg=hello n=1 request_id=r1") {
		t.Errorf("text: %q", got)
	}
	var js bytes.Buffer
	slog.New(newLogHandler(&js, false, config.LogFormatJSON)).InfoContext(ctx, "hello", "n", 1)
	var line map[string]any
	if err := json.Unmarshal(js.Bytes(), &line); err != nil {
		t.Fatalf("json: %v: %q", err, js.String())
	}
	if line["msg"] != "hello" || line["level"] != "INFO" || line["n"] != float64(1) || line["request_id"] != "r1" {
		t.Errorf("json: %v", line)
	}
}
