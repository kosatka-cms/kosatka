// Package metrics - the service's metrics in the Prometheus text format
// (version 0.0.4), the one Prometheus, Grafana Alloy, VictoriaMetrics and
// the Elastic Agent read: counters, histograms and values read when the
// metrics are asked for. No outside dependency; only what the engine
// uses.
package metrics

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// ContentType - the Content-Type of Registry.Write's output.
const ContentType = "text/plain; version=0.0.4; charset=utf-8"

// Registry - a set of metrics, written in the order they were added.
type Registry struct {
	mu      sync.Mutex
	metrics []metric
	names   map[string]bool
}

// NewRegistry - an empty set.
func NewRegistry() *Registry {
	return &Registry{names: map[string]bool{}}
}

// metric - one metric family: writes its HELP, TYPE and samples.
type metric interface {
	write(w io.Writer) error
}

func (r *Registry) add(name string, m metric) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.names[name] {
		panic("metrics: " + name + " added twice")
	}
	r.names[name] = true
	r.metrics = append(r.metrics, m)
}

// Write writes every metric in the text format.
func (r *Registry) Write(w io.Writer) error {
	r.mu.Lock()
	ms := append([]metric(nil), r.metrics...)
	r.mu.Unlock()
	for _, m := range ms {
		if err := m.write(w); err != nil {
			return err
		}
	}
	return nil
}

// header - a family's HELP and TYPE lines.
func header(w io.Writer, name, help, typ string) error {
	_, err := fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n", name, escapeHelp(help), name, typ)
	return err
}

// CounterVec - a counter for each set of its labels' values.
type CounterVec struct {
	name, help string
	labels     []string
	mu         sync.Mutex
	values     map[string]*counterValue
}

type counterValue struct {
	labels []string
	value  float64
}

// Counter adds a counter (a value that only grows) with the given labels.
func (r *Registry) Counter(name, help string, labels ...string) *CounterVec {
	c := &CounterVec{name: name, help: help, labels: labels, values: map[string]*counterValue{}}
	r.add(name, c)
	return c
}

// Add adds v to the counter of the labels' values (in the order of the
// labels given to Counter).
func (c *CounterVec) Add(v float64, values ...string) {
	if len(values) != len(c.labels) {
		panic(fmt.Sprintf("metrics: %s takes %d labels, got %d", c.name, len(c.labels), len(values)))
	}
	key := strings.Join(values, "\xff")
	c.mu.Lock()
	cv := c.values[key]
	if cv == nil {
		cv = &counterValue{labels: append([]string(nil), values...)}
		c.values[key] = cv
	}
	cv.value += v
	c.mu.Unlock()
}

// Inc adds one.
func (c *CounterVec) Inc(values ...string) { c.Add(1, values...) }

func (c *CounterVec) write(w io.Writer) error {
	if err := header(w, c.name, c.help, "counter"); err != nil {
		return err
	}
	c.mu.Lock()
	rows := make([]counterValue, 0, len(c.values))
	for _, v := range c.values {
		rows = append(rows, *v)
	}
	c.mu.Unlock()
	sort.Slice(rows, func(i, j int) bool { return lessLabels(rows[i].labels, rows[j].labels) })
	for _, row := range rows {
		if _, err := fmt.Fprintf(w, "%s%s %s\n", c.name, labelPairs(c.labels, row.labels, "", ""), formatFloat(row.value)); err != nil {
			return err
		}
	}
	return nil
}

// HistogramVec - a histogram (counts of observed values under each
// bound, their sum and count) for each set of its labels' values.
type HistogramVec struct {
	name, help string
	labels     []string
	buckets    []float64
	mu         sync.Mutex
	values     map[string]*histogramValue
}

type histogramValue struct {
	labels []string
	counts []uint64
	sum    float64
	count  uint64
}

// Histogram adds a histogram with the given upper bounds (ascending)
// and labels.
func (r *Registry) Histogram(name, help string, buckets []float64, labels ...string) *HistogramVec {
	if !sort.Float64sAreSorted(buckets) {
		panic("metrics: " + name + ": the buckets are not ascending")
	}
	h := &HistogramVec{name: name, help: help, labels: labels, buckets: append([]float64(nil), buckets...), values: map[string]*histogramValue{}}
	r.add(name, h)
	return h
}

// Observe counts v for the labels' values.
func (h *HistogramVec) Observe(v float64, values ...string) {
	if len(values) != len(h.labels) {
		panic(fmt.Sprintf("metrics: %s takes %d labels, got %d", h.name, len(h.labels), len(values)))
	}
	key := strings.Join(values, "\xff")
	h.mu.Lock()
	hv := h.values[key]
	if hv == nil {
		hv = &histogramValue{labels: append([]string(nil), values...), counts: make([]uint64, len(h.buckets))}
		h.values[key] = hv
	}
	for i, b := range h.buckets {
		if v <= b {
			hv.counts[i]++
		}
	}
	hv.sum += v
	hv.count++
	h.mu.Unlock()
}

func (h *HistogramVec) write(w io.Writer) error {
	if err := header(w, h.name, h.help, "histogram"); err != nil {
		return err
	}
	h.mu.Lock()
	rows := make([]histogramValue, 0, len(h.values))
	for _, v := range h.values {
		rows = append(rows, histogramValue{labels: v.labels, counts: append([]uint64(nil), v.counts...), sum: v.sum, count: v.count})
	}
	h.mu.Unlock()
	sort.Slice(rows, func(i, j int) bool { return lessLabels(rows[i].labels, rows[j].labels) })
	for _, row := range rows {
		for i, b := range h.buckets {
			if _, err := fmt.Fprintf(w, "%s_bucket%s %d\n", h.name, labelPairs(h.labels, row.labels, "le", formatFloat(b)), row.counts[i]); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "%s_bucket%s %d\n%s_sum%s %s\n%s_count%s %d\n",
			h.name, labelPairs(h.labels, row.labels, "le", "+Inf"), row.count,
			h.name, labelPairs(h.labels, row.labels, "", ""), formatFloat(row.sum),
			h.name, labelPairs(h.labels, row.labels, "", ""), row.count); err != nil {
			return err
		}
	}
	return nil
}

// Sample - one value of a metric read on demand, with its labels' values.
type Sample struct {
	Labels []string
	Value  float64
}

// funcMetric - a gauge or counter whose samples are read when written.
type funcMetric struct {
	name, help, typ string
	labels          []string
	read            func() []Sample
}

// GaugeFunc adds a gauge (a value that goes up and down) read by fn.
func (r *Registry) GaugeFunc(name, help string, fn func() float64) {
	r.add(name, &funcMetric{name: name, help: help, typ: "gauge", read: func() []Sample { return []Sample{{Value: fn()}} }})
}

// CounterFunc adds a counter kept elsewhere (a total that only grows),
// read by fn.
func (r *Registry) CounterFunc(name, help string, fn func() float64) {
	r.add(name, &funcMetric{name: name, help: help, typ: "counter", read: func() []Sample { return []Sample{{Value: fn()}} }})
}

// GaugeSamples adds a gauge with labels, its samples read by fn (e.g. an
// info metric: one sample of 1 labelled with what it describes).
func (r *Registry) GaugeSamples(name, help string, labels []string, fn func() []Sample) {
	r.add(name, &funcMetric{name: name, help: help, typ: "gauge", labels: labels, read: fn})
}

func (f *funcMetric) write(w io.Writer) error {
	samples := f.read()
	if err := header(w, f.name, f.help, f.typ); err != nil {
		return err
	}
	for _, s := range samples {
		if len(s.Labels) != len(f.labels) {
			return fmt.Errorf("metrics: %s takes %d labels, a sample has %d", f.name, len(f.labels), len(s.Labels))
		}
		if _, err := fmt.Fprintf(w, "%s%s %s\n", f.name, labelPairs(f.labels, s.Labels, "", ""), formatFloat(s.Value)); err != nil {
			return err
		}
	}
	return nil
}

// labelPairs - {a="1",b="2"} (and extra="value" when extra is set), ""
// with no labels.
func labelPairs(names, values []string, extra, extraValue string) string {
	if len(names) == 0 && extra == "" {
		return ""
	}
	var b strings.Builder
	b.WriteByte('{')
	for i, n := range names {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(n)
		b.WriteString(`="`)
		b.WriteString(escapeLabel(values[i]))
		b.WriteByte('"')
	}
	if extra != "" {
		if len(names) > 0 {
			b.WriteByte(',')
		}
		b.WriteString(extra)
		b.WriteString(`="`)
		b.WriteString(extraValue)
		b.WriteByte('"')
	}
	b.WriteByte('}')
	return b.String()
}

func lessLabels(a, b []string) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

var (
	labelEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
	helpEscaper  = strings.NewReplacer(`\`, `\\`, "\n", `\n`)
)

func escapeLabel(s string) string { return labelEscaper.Replace(s) }
func escapeHelp(s string) string  { return helpEscaper.Replace(s) }

// formatFloat - a sample value: +Inf, -Inf, NaN or the shortest decimal.
func formatFloat(v float64) string {
	switch {
	case math.IsInf(v, 1):
		return "+Inf"
	case math.IsInf(v, -1):
		return "-Inf"
	case math.IsNaN(v):
		return "NaN"
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}
