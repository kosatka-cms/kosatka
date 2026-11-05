package entity

import (
	"maps"
	"testing"
)

func TestViewModuleParams(t *testing.T) {
	params := []ViewModuleParam{
		{Key: "min", Type: PluginBlockSettingNumber, Default: "5", Min: 1, Max: 100},
		{Key: "look", Type: PluginBlockSettingSelect, Default: "stars", Options: []PluginBlockTarget{{Value: "stars"}, {Value: "average"}}},
	}
	cases := []struct {
		name   string
		stored map[string]string
		want   map[string]string
	}{
		{"none stored: the defaults", nil, map[string]string{"min": "5", "look": "stars"}},
		{"accepted values kept", map[string]string{"min": "20", "look": "average"}, map[string]string{"min": "20", "look": "average"}},
		{"out of range or gone: the default", map[string]string{"min": "0", "look": "bars"}, map[string]string{"min": "5", "look": "stars"}},
		{"a value of no param dropped", map[string]string{"min": "7", "old": "x"}, map[string]string{"min": "7", "look": "stars"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ViewModuleParams(params, c.stored); !maps.Equal(got, c.want) {
				t.Errorf("ViewModuleParams = %v, want %v", got, c.want)
			}
		})
	}
}

func TestViewModuleFilterMatch(t *testing.T) {
	cases := []struct {
		op    ViewFilterOperator
		value string
		v     float64
		has   bool
		want  bool
	}{
		{ViewFilterOperatorGte, "4", 4.5, true, true},
		{ViewFilterOperatorGte, "4", 3.9, true, false},
		{ViewFilterOperatorLte, "4", 3.9, true, true},
		{ViewFilterOperatorEq, "3", 3, true, true},
		{ViewFilterOperatorEq, "3", 0, false, false},
		// No value: a comparison doesn't match it, "empty" does.
		{ViewFilterOperatorGte, "0", 0, false, false},
		{ViewFilterOperatorEmpty, "", 0, false, true},
		{ViewFilterOperatorEmpty, "", 2, true, false},
		{ViewFilterOperatorNotEmpty, "", 2, true, true},
		// Not a number: no constraint.
		{ViewFilterOperatorGte, "many", 1, true, true},
	}
	for _, c := range cases {
		if got := ViewModuleFilterMatch(c.op, c.value, c.v, c.has); got != c.want {
			t.Errorf("ViewModuleFilterMatch(%s, %q, %v, %v) = %v, want %v", c.op, c.value, c.v, c.has, got, c.want)
		}
	}
}

// TestViewModuleChoiceMatch - the one value, one of several, empty and
// not; no value asked - no constraint.
func TestViewModuleChoiceMatch(t *testing.T) {
	for _, tc := range []struct {
		op    ViewFilterOperator
		value string
		v     string
		has   bool
		want  bool
	}{
		{ViewFilterOperatorEq, "review", "review", true, true},
		{ViewFilterOperatorEq, "review", "draft", true, false},
		{ViewFilterOperatorIn, "draft, review", "review", true, true},
		{ViewFilterOperatorIn, "draft,review", "live", true, false},
		{ViewFilterOperatorEq, "review", "", false, false},
		{ViewFilterOperatorEq, "", "draft", true, true},
		{ViewFilterOperatorEmpty, "", "", false, true},
		{ViewFilterOperatorNotEmpty, "", "draft", true, true},
	} {
		if got := ViewModuleChoiceMatch(tc.op, tc.value, tc.v, tc.has); got != tc.want {
			t.Errorf("%s %q on %q (%v) = %v", tc.op, tc.value, tc.v, tc.has, got)
		}
	}
}
