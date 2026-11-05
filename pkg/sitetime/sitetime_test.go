package sitetime

import (
	"testing"
	"time"
)

func TestSetAndIn(t *testing.T) {
	t.Cleanup(func() { _ = Set("UTC") })
	if Location() != time.UTC {
		t.Fatalf("default: %v", Location())
	}
	if err := Set(""); err == nil {
		t.Fatal("empty name must be an error")
	}
	if err := Set("Nowhere/City"); err == nil {
		t.Fatal("unknown name must be an error")
	}
	if Location() != time.UTC {
		t.Fatal("a failed Set must not change the zone")
	}
	if err := Set("Asia/Vladivostok"); err != nil {
		t.Fatal(err)
	}
	got := In(time.Date(2027, 1, 1, 20, 0, 0, 0, time.UTC))
	if got.Day() != 2 || got.Hour() != 6 {
		t.Fatalf("In: %v", got)
	}
}

// TestZoneOptions - UTC, then every region's canonical zones grouped by
// region and labelled by place and offset; a stored zone outside them (an
// alias) first; a listed zone isn't added twice.
func TestZoneOptions(t *testing.T) {
	opts := ZoneOptions("Asia/Calcutta")
	if opts[0] != (ZoneOption{Name: "Asia/Calcutta", Label: "Calcutta (UTC+5:30)", Group: "Asia"}) {
		t.Fatalf("current zone outside the list first: %+v", opts[0])
	}
	if opts[1] != (ZoneOption{Name: "UTC", Label: "UTC (UTC)"}) {
		t.Fatalf("UTC next, ungrouped: %+v", opts[1])
	}
	byName := map[string]ZoneOption{}
	for _, o := range opts {
		byName[o.Name] = o
	}
	for name, want := range map[string]ZoneOption{
		"Europe/Moscow":                  {Label: "Moscow (UTC+3)", Group: "Europe"},
		"Asia/Kolkata":                   {Label: "Kolkata (UTC+5:30)", Group: "Asia"},
		"America/Argentina/Buenos_Aires": {Label: "Buenos Aires, Argentina (UTC-3)", Group: "America"},
	} {
		got := byName[name]
		if got.Label != want.Label || got.Group != want.Group {
			t.Errorf("%s: %+v, want %+v", name, got, want)
		}
	}
	for _, name := range []string{"Africa/Lagos", "Pacific/Auckland", "Europe/Kyiv", "America/Sao_Paulo"} {
		if _, ok := byName[name]; !ok {
			t.Errorf("%s not offered", name)
		}
	}
	if _, ok := byName["Europe/Kiev"]; ok {
		t.Error("an alias offered besides its zone")
	}
	if len(Zones) < 300 {
		t.Errorf("only %d zones", len(Zones))
	}
	if n := len(ZoneOptions("Europe/Moscow")); n != len(Zones) {
		t.Fatalf("a listed zone isn't added twice: %d", n)
	}
}

func TestForUserAndContext(t *testing.T) {
	t.Cleanup(func() { _ = Set("UTC"); SetUserZones(false) })
	if err := Set("Europe/Moscow"); err != nil {
		t.Fatal(err)
	}
	if ForUser("Asia/Vladivostok").String() != "Europe/Moscow" {
		t.Fatal("user zones off - the site's zone")
	}
	SetUserZones(true)
	if ForUser("Asia/Vladivostok").String() != "Asia/Vladivostok" || ForUser("").String() != "Europe/Moscow" || ForUser("Nowhere/X").String() != "Europe/Moscow" {
		t.Fatal("ForUser")
	}
	ctx := WithLocation(t.Context(), ForUser("Asia/Vladivostok"))
	if FromContext(ctx).String() != "Asia/Vladivostok" || FromContext(t.Context()).String() != "Europe/Moscow" {
		t.Fatal("FromContext")
	}
	if IsSite(FromContext(ctx)) || !IsSite(FromContext(t.Context())) {
		t.Fatal("IsSite")
	}
}
