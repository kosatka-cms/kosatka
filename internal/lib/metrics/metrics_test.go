package metrics

import (
	"math"
	"strings"
	"sync"
	"testing"
)

func write(t *testing.T, r *Registry) string {
	t.Helper()
	var b strings.Builder
	if err := r.Write(&b); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestCounter(t *testing.T) {
	r := NewRegistry()
	c := r.Counter("app_requests_total", "Requests served.", "method", "code")
	c.Inc("GET", "200")
	c.Inc("GET", "200")
	c.Add(3, "POST", "500")
	c.Inc("GET", "404")
	want := `# HELP app_requests_total Requests served.
# TYPE app_requests_total counter
app_requests_total{method="GET",code="200"} 2
app_requests_total{method="GET",code="404"} 1
app_requests_total{method="POST",code="500"} 3
`
	if got := write(t, r); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestCounterWithoutLabels(t *testing.T) {
	r := NewRegistry()
	r.Counter("app_total", "Total.").Inc()
	want := "# HELP app_total Total.\n# TYPE app_total counter\napp_total 1\n"
	if got := write(t, r); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestHistogram(t *testing.T) {
	r := NewRegistry()
	h := r.Histogram("app_duration_seconds", "Time taken.", []float64{0.1, 0.5, 1}, "route")
	h.Observe(0.05, "/a")
	h.Observe(0.3, "/a")
	h.Observe(2, "/a")
	want := `# HELP app_duration_seconds Time taken.
# TYPE app_duration_seconds histogram
app_duration_seconds_bucket{route="/a",le="0.1"} 1
app_duration_seconds_bucket{route="/a",le="0.5"} 2
app_duration_seconds_bucket{route="/a",le="1"} 2
app_duration_seconds_bucket{route="/a",le="+Inf"} 3
app_duration_seconds_sum{route="/a"} 2.35
app_duration_seconds_count{route="/a"} 3
`
	if got := write(t, r); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestFuncMetrics(t *testing.T) {
	r := NewRegistry()
	n := 4.0
	r.GaugeFunc("app_open", "Open things.", func() float64 { return n })
	r.CounterFunc("app_waits_total", "Waits.", func() float64 { return 7 })
	r.GaugeSamples("app_info", "The build.", []string{"version"}, func() []Sample {
		return []Sample{{Labels: []string{"v1.2.0"}, Value: 1}}
	})
	n = 5
	want := `# HELP app_open Open things.
# TYPE app_open gauge
app_open 5
# HELP app_waits_total Waits.
# TYPE app_waits_total counter
app_waits_total 7
# HELP app_info The build.
# TYPE app_info gauge
app_info{version="v1.2.0"} 1
`
	if got := write(t, r); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestEscaping(t *testing.T) {
	r := NewRegistry()
	r.Counter("app_total", "Line one\nback\\slash.", "path").Inc("a\"b\\c\nd")
	got := write(t, r)
	for _, want := range []string{
		`# HELP app_total Line one\nback\\slash.`,
		`app_total{path="a\"b\\c\nd"} 1`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("no %q in\n%s", want, got)
		}
	}
}

func TestFormatFloat(t *testing.T) {
	for v, want := range map[float64]string{1: "1", 0.25: "0.25", 1e-9: "1e-09", math.Inf(1): "+Inf", math.Inf(-1): "-Inf"} {
		if got := formatFloat(v); got != want {
			t.Errorf("formatFloat(%v) = %q, want %q", v, got, want)
		}
	}
	if got := formatFloat(math.NaN()); got != "NaN" {
		t.Errorf("NaN: %q", got)
	}
}

func TestAddedTwicePanics(t *testing.T) {
	r := NewRegistry()
	r.Counter("app_total", "x")
	defer func() {
		if recover() == nil {
			t.Error("no panic")
		}
	}()
	r.GaugeFunc("app_total", "x", func() float64 { return 0 })
}

func TestWrongLabelCountPanics(t *testing.T) {
	c := NewRegistry().Counter("app_total", "x", "a")
	defer func() {
		if recover() == nil {
			t.Error("no panic")
		}
	}()
	c.Inc("1", "2")
}

func TestConcurrentUse(t *testing.T) {
	r := NewRegistry()
	c := r.Counter("app_total", "x", "k")
	h := r.Histogram("app_seconds", "x", []float64{1}, "k")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				c.Inc("a")
				h.Observe(0.5, "a")
				_ = r.Write(&strings.Builder{})
			}
		}()
	}
	wg.Wait()
	got := write(t, r)
	if !strings.Contains(got, `app_total{k="a"} 8000`) || !strings.Contains(got, `app_seconds_count{k="a"} 8000`) {
		t.Errorf("lost updates:\n%s", got)
	}
}
