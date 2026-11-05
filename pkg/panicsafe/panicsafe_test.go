package panicsafe

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRecoverStopsPanicFromCrashingProcess(t *testing.T) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer Recover(nil, "test")
		panic("boom")
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("goroutine did not complete; panic was not recovered")
	}
}

func TestRecoverLogsThePanic(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	var wg sync.WaitGroup
	wg.Go(func() {
		defer Recover(logger, "my.component")
		panic("something broke")
	})
	wg.Wait()

	out := buf.String()
	if !strings.Contains(out, "recovered from panic") {
		t.Fatalf("expected log message, got %q", out)
	}
	if !strings.Contains(out, "my.component") {
		t.Fatalf("expected component label in log, got %q", out)
	}
	if !strings.Contains(out, "something broke") {
		t.Fatalf("expected panic value in log, got %q", out)
	}
}

func TestRecoverNoopWhenNoPanic(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	func() {
		defer Recover(logger, "test")
	}()

	if buf.Len() != 0 {
		t.Fatalf("expected no log output when nothing panicked, got %q", buf.String())
	}
}
