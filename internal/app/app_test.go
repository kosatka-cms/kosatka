package app

import (
	"context"
	"log/slog"
	"testing"
)

// TestSetupLoggerLevel - a development machine (env local) logs DEBUG; a
// live site logs from INFO up.
func TestSetupLoggerLevel(t *testing.T) {
	ctx := context.Background()
	for _, local := range []bool{true, false} {
		log := setupLogger(local)
		if got := log.Enabled(ctx, slog.LevelDebug); got != local {
			t.Errorf("local %v: DEBUG enabled %v", local, got)
		}
		if !log.Enabled(ctx, slog.LevelInfo) {
			t.Errorf("local %v: INFO disabled", local)
		}
	}
}
