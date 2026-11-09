package metrics

import (
	"database/sql"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"
)

// RegisterRuntime adds the Go runtime's and the process's metrics, under
// the names the usual Go dashboards read (go_goroutines,
// go_memstats_heap_alloc_bytes, process_start_time_seconds…). The memory
// statistics are read once a scrape.
func RegisterRuntime(r *Registry) {
	start := float64(time.Now().UnixNano()) / 1e9
	var (
		mu   sync.Mutex
		ms   runtime.MemStats
		read time.Time
	)
	// mem - the memory statistics, read again when older than a second
	// (one scrape writes several of them).
	mem := func() runtime.MemStats {
		mu.Lock()
		defer mu.Unlock()
		if time.Since(read) > time.Second {
			runtime.ReadMemStats(&ms)
			read = time.Now()
		}
		return ms
	}
	r.GaugeSamples("go_info", "The Go version the binary was built with.", []string{"version"}, func() []Sample {
		return []Sample{{Labels: []string{runtime.Version()}, Value: 1}}
	})
	r.GaugeFunc("go_goroutines", "Goroutines that currently exist.", func() float64 { return float64(runtime.NumGoroutine()) })
	threads := pprof.Lookup("threadcreate")
	r.GaugeFunc("go_threads", "OS threads created.", func() float64 { return float64(threads.Count()) })
	r.GaugeFunc("go_memstats_heap_alloc_bytes", "Heap bytes allocated and still in use.", func() float64 { return float64(mem().HeapAlloc) })
	r.GaugeFunc("go_memstats_heap_inuse_bytes", "Heap bytes in in-use spans.", func() float64 { return float64(mem().HeapInuse) })
	r.GaugeFunc("go_memstats_sys_bytes", "Bytes obtained from the system.", func() float64 { return float64(mem().Sys) })
	r.GaugeFunc("go_memstats_next_gc_bytes", "Heap size at which the next garbage collection runs.", func() float64 { return float64(mem().NextGC) })
	r.CounterFunc("go_memstats_alloc_bytes_total", "Bytes allocated, even if freed.", func() float64 { return float64(mem().TotalAlloc) })
	r.CounterFunc("go_gc_cycles_total", "Garbage collection cycles completed.", func() float64 { return float64(mem().NumGC) })
	r.CounterFunc("go_gc_pause_seconds_total", "Time the garbage collector stopped the program.", func() float64 { return float64(mem().PauseTotalNs) / 1e9 })
	r.GaugeFunc("process_start_time_seconds", "Start time of the process since the Unix epoch, in seconds.", func() float64 { return start })
}

// RegisterDBStats adds a database pool's metrics, named prefix_db_…
// (open, in use and idle connections, the limit, waits for one).
func RegisterDBStats(r *Registry, prefix string, stats func() sql.DBStats) {
	r.GaugeFunc(prefix+"_db_max_open_connections", "The limit of open connections to the database.", func() float64 { return float64(stats().MaxOpenConnections) })
	r.GaugeFunc(prefix+"_db_open_connections", "Open connections to the database, in use and idle.", func() float64 { return float64(stats().OpenConnections) })
	r.GaugeFunc(prefix+"_db_in_use_connections", "Connections to the database in use.", func() float64 { return float64(stats().InUse) })
	r.GaugeFunc(prefix+"_db_idle_connections", "Idle connections to the database.", func() float64 { return float64(stats().Idle) })
	r.CounterFunc(prefix+"_db_wait_count_total", "Times a query waited for a free connection.", func() float64 { return float64(stats().WaitCount) })
	r.CounterFunc(prefix+"_db_wait_duration_seconds_total", "Time queries waited for a free connection.", func() float64 { return stats().WaitDuration.Seconds() })
}
