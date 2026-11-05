package service

import (
	"context"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestCopyrightYears - the current year, or a range from an earlier first
// year; a first year of 0, the current one, a later one or garbage is the
// current year alone.
func TestCopyrightYears(t *testing.T) {
	now := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	for since, want := range map[string]string{"": "2026", "0": "2026", "2008": "2008-2026", "2026": "2026", "2030": "2026", "x": "2026"} {
		if got := CopyrightYears(since, now); got != want {
			t.Errorf("CopyrightYears(%q) = %q, want %q", since, got, want)
		}
	}
}

// TestCopyrightBlockSettings - the holder (text) and the first year (up to
// the current one).
func TestCopyrightBlockSettings(t *testing.T) {
	RegisterCopyrightBlock()
	settings, err := PluginBlockSettings(context.Background(), entity.PluginBlockCopyright)
	if err != nil || len(settings) != 2 {
		t.Fatalf("settings %v, %v", settings, err)
	}
	holder, since := settings[0], settings[1]
	if holder.Key != entity.CopyrightHolderKey || holder.Type != entity.PluginBlockSettingText || !holder.Check("") {
		t.Errorf("the holder: %+v", holder)
	}
	year := time.Now().Year()
	if since.Key != entity.CopyrightSinceKey || !since.Check("0") || !since.Check("2008") || since.Check(time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC).Format("2006")) {
		t.Errorf("the first year: %+v", since)
	}
}
