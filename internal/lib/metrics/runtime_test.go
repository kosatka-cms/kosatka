package metrics

import (
	"database/sql"
	"strings"
	"testing"
	"time"
)

func TestRegisterRuntime(t *testing.T) {
	r := NewRegistry()
	RegisterRuntime(r)
	got := write(t, r)
	for _, name := range []string{
		"go_info{version=", "go_goroutines ", "go_threads ", "go_memstats_heap_alloc_bytes ",
		"go_memstats_sys_bytes ", "go_gc_cycles_total ", "process_start_time_seconds ",
	} {
		if !strings.Contains(got, "\n"+name) {
			t.Errorf("no %s in\n%s", name, got)
		}
	}
}

func TestRegisterDBStats(t *testing.T) {
	r := NewRegistry()
	RegisterDBStats(r, "app", func() sql.DBStats {
		return sql.DBStats{MaxOpenConnections: 25, OpenConnections: 3, InUse: 1, Idle: 2, WaitCount: 4, WaitDuration: 1500 * time.Millisecond}
	})
	got := write(t, r)
	for _, want := range []string{
		"app_db_max_open_connections 25", "app_db_open_connections 3", "app_db_in_use_connections 1",
		"app_db_idle_connections 2", "app_db_wait_count_total 4", "app_db_wait_duration_seconds_total 1.5",
	} {
		if !strings.Contains(got, want+"\n") {
			t.Errorf("no %q in\n%s", want, got)
		}
	}
}
