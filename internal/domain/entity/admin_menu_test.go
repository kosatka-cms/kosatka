package entity

import "testing"

// TestInsertInGroup - a module's item goes after the last item of its
// group, one without a group (or of a group not there) at the end.
func TestInsertInGroup(t *testing.T) {
	items := []AdminMenuItem{{Label: "a", Group: "site"}, {Label: "b", Group: "texts"}, {Label: "c", Group: "texts"}, {Label: "d", Group: "modules"}}
	labels := func(items []AdminMenuItem) string {
		s := ""
		for _, it := range items {
			s += it.Label
		}
		return s
	}
	if got := labels(insertInGroup(append([]AdminMenuItem(nil), items...), AdminMenuItem{Label: "x", Group: "texts"})); got != "abcxd" {
		t.Errorf("in its group: %s", got)
	}
	if got := labels(insertInGroup(append([]AdminMenuItem(nil), items...), AdminMenuItem{Label: "x"})); got != "abcdx" {
		t.Errorf("no group: %s", got)
	}
	if got := labels(insertInGroup(append([]AdminMenuItem(nil), items...), AdminMenuItem{Label: "x", Group: "other"})); got != "abcdx" {
		t.Errorf("a group not there: %s", got)
	}
}
