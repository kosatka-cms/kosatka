// Package sitetime holds the site's time zone
// (entity.SiteInformation.Timezone): dates are stored in UTC and shown in
// this zone. Set at startup and whenever the setting is saved; UTC until
// then.
package sitetime

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	mu  sync.RWMutex
	loc = time.UTC
)

// Location - the site's time zone.
func Location() *time.Location {
	mu.RLock()
	defer mu.RUnlock()
	return loc
}

// In - t in the site's time zone.
func In(t time.Time) time.Time {
	return t.In(Location())
}

// Format - t in the site's time zone in layout; "-" for no time (zero or
// before 1970).
func Format(t time.Time, layout string) string {
	if t.IsZero() || t.Year() < 1970 {
		return "-"
	}
	return In(t).Format(layout)
}

// Now - the current time in the site's time zone.
func Now() time.Time {
	return In(time.Now())
}

// Set makes name (an IANA zone like "Europe/Moscow") the site's time
// zone; an empty or unknown name is an error and changes nothing.
func Set(name string) error {
	l, err := Load(name)
	if err != nil {
		return err
	}
	mu.Lock()
	loc = l
	mu.Unlock()
	return nil
}

// Load - the zone named name; an empty name is invalid (unlike
// time.LoadLocation, which reads it as UTC).
func Load(name string) (*time.Location, error) {
	if name == "" {
		return nil, errEmpty
	}
	return time.LoadLocation(name)
}

var errEmpty = errors.New("sitetime: empty time zone name")

//go:generate go run ./internal/zonesgen

// Zones - the time zones offered in the site and user settings: UTC, then
// every canonical IANA zone of the world's regions (zones_gen.go). A stored
// zone outside the list is offered too (ZoneOptions).
var Zones = append([]string{"UTC"}, zones...)

// ZoneOption - a zone for a select: Group - its region ("Europe",
// "America"…; "" for UTC), for an <optgroup>; Label - the rest of its
// name, readable, with its current UTC offset ("Buenos Aires, Argentina
// (UTC-3)" in "America"; "UTC").
type ZoneOption struct {
	Name  string
	Label string
	Group string
}

var (
	locationsOnce sync.Once
	locations     map[string]*time.Location
)

// location - a zone of Zones, loaded once; any other name is loaded as asked.
func location(name string) (*time.Location, error) {
	locationsOnce.Do(func() {
		locations = make(map[string]*time.Location, len(Zones))
		for _, n := range Zones {
			if l, err := Load(n); err == nil {
				locations[n] = l
			}
		}
	})
	if l, ok := locations[name]; ok {
		return l, nil
	}
	return Load(name)
}

// ZoneOptions - Zones (plus current when it is outside them), grouped by
// region, labelled by place and current offset.
func ZoneOptions(current string) []ZoneOption {
	names := Zones
	if current != "" && !slices.Contains(names, current) {
		names = append([]string{current}, names...)
	}
	now := time.Now()
	out := make([]ZoneOption, 0, len(names))
	for _, n := range names {
		l, err := location(n)
		if err != nil {
			continue
		}
		group, place := zoneGroupPlace(n)
		out = append(out, ZoneOption{Name: n, Label: place + " (" + offsetLabel(now.In(l)) + ")", Group: group})
	}
	return out
}

// zoneGroupPlace - a zone's region and its place, readable: the parts after
// the region, the most specific first, "_" as spaces
// ("America/Argentina/Buenos_Aires" → "America", "Buenos Aires,
// Argentina"); a name without a region ("UTC") is its own place.
func zoneGroupPlace(name string) (group, place string) {
	parts := strings.Split(name, "/")
	if len(parts) < 2 {
		return "", name
	}
	rest := parts[1:]
	slices.Reverse(rest)
	return parts[0], strings.ReplaceAll(strings.Join(rest, ", "), "_", " ")
}

func offsetLabel(t time.Time) string {
	_, off := t.Zone()
	if off == 0 {
		return "UTC"
	}
	sign := "+"
	if off < 0 {
		sign, off = "-", -off
	}
	s := "UTC" + sign + strconv.Itoa(off/3600)
	if m := off % 3600 / 60; m != 0 {
		s += fmt.Sprintf(":%02d", m)
	}
	return s
}

// User time zones - whether users may pick their own zone (an account
// settings option). Set at startup and whenever the setting is saved.
var (
	userZones   bool
	zoneCacheMu sync.RWMutex
	zoneCache   = map[string]*time.Location{}
)

// SetUserZones turns per-user time zones on or off.
func SetUserZones(on bool) {
	mu.Lock()
	userZones = on
	mu.Unlock()
}

// UserZonesEnabled - whether users may set their own time zone.
func UserZonesEnabled() bool {
	mu.RLock()
	defer mu.RUnlock()
	return userZones
}

// ForUser - the time zone to show a user's pages in: their own (name)
// when user time zones are on and name is a valid zone, else the site's.
func ForUser(name string) *time.Location {
	if name == "" || !UserZonesEnabled() {
		return Location()
	}
	zoneCacheMu.RLock()
	l, ok := zoneCache[name]
	zoneCacheMu.RUnlock()
	if !ok {
		var err error
		if l, err = Load(name); err != nil {
			l = nil
		}
		zoneCacheMu.Lock()
		zoneCache[name] = l
		zoneCacheMu.Unlock()
	}
	if l == nil {
		return Location()
	}
	return l
}

type ctxKey struct{}

// WithLocation - ctx carrying loc as the request's time zone.
func WithLocation(ctx context.Context, loc *time.Location) context.Context {
	return context.WithValue(ctx, ctxKey{}, loc)
}

// FromContext - the request's time zone (WithLocation), else the site's.
func FromContext(ctx context.Context) *time.Location {
	if ctx != nil {
		if l, ok := ctx.Value(ctxKey{}).(*time.Location); ok && l != nil {
			return l
		}
	}
	return Location()
}

// IsSite - loc is the site's time zone (the shared, pre-rendered output
// applies as is).
func IsSite(loc *time.Location) bool {
	return loc == nil || loc.String() == Location().String()
}
