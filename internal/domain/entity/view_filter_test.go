package entity

import (
	"testing"
	"time"
)

func TestParseRelativeDate(t *testing.T) {
	// Midday UTC: the same calendar day in any site time zone within
	// ±11h, so the test doesn't hit a day boundary.
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	today := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		expr    string
		want    time.Time
		wantErr bool
	}{
		{name: "now", expr: "now", want: today},
		{name: "empty string treated as now", expr: "", want: today},
		{name: "plus days", expr: "+7 days", want: today.AddDate(0, 0, 7)},
		{name: "minus month", expr: "-1 month", want: today.AddDate(0, -1, 0)},
		{name: "plus weeks", expr: "+2 weeks", want: today.AddDate(0, 0, 14)},
		{name: "plus year", expr: "+1 year", want: today.AddDate(1, 0, 0)},
		{name: "singular day no sign", expr: "1 day", want: today.AddDate(0, 0, 1)},
		{name: "garbage", expr: "banana", wantErr: true},
		{name: "garbage unit", expr: "+3 fortnights", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRelativeDate(tt.expr, now)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseRelativeDate(%q) = %v, want error", tt.expr, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRelativeDate(%q) unexpected error: %v", tt.expr, err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("ParseRelativeDate(%q) = %v, want %v", tt.expr, got, tt.want)
			}
		})
	}
}
