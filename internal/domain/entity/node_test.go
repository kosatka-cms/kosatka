package entity

import "testing"

func TestNodeRelPath(t *testing.T) {
	withModules(t, testModules()...)
	tests := []struct {
		name     string
		nodeType ContentType
		slug     string
		want     string
	}{
		{"slug already carrying its own type prefix is used as-is", ContentType("events"), "events/bar-2024", "events/bar-2024"},
		{"new forum topic's bare slug gets forum/ prepended", ContentType("forum"), "24bba67", "forum/24bba67"},
		{"legacy forum topic already carrying forum/ isn't double-prefixed", ContentType("forum"), "forum/staraya-tema", "forum/staraya-tema"},
		{"leading/trailing slashes and whitespace are trimmed first", ContentType("forum"), "  /24bba67/  ", "forum/24bba67"},
		{"empty slug returns empty", ContentType("events"), "", ""},
		{"path segments are percent-escaped", ContentType("forum"), "24bba67 spam?", "forum/24bba67%20spam%3F"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NodeRelPath(tt.nodeType, tt.slug); got != tt.want {
				t.Errorf("NodeRelPath(%q, %q) = %q, want %q", tt.nodeType, tt.slug, got, tt.want)
			}
		})
	}
}

// TestNodeURL - a node's link is "/" + NodeRelPath: a forum topic's bare
// slug gets its prefix.
func TestNodeURL(t *testing.T) {
	withModules(t, testModules()...)
	if got := (Node{Type: "forum", Slug: "24bba67"}).URL(); got != "/forum/24bba67" {
		t.Errorf("forum topic: %q", got)
	}
	if got := (Node{Type: "events", Slug: "events/bar-2024"}).URL(); got != "/events/bar-2024" {
		t.Errorf("event: %q", got)
	}
}

// The built-in modules' node types are reserved, and the templates'
// suffixes for every type ("node", "default"): every other type is a
// content_types row, and a repeated machine name is the database's
// duplicate error.
func TestReservedContentTypeMachineNames(t *testing.T) {
	withModules(t, testModules()...)
	got := ReservedContentTypeMachineNames()
	labels := ModuleNodeTypeLabels()
	if len(got) != len(labels)+2 {
		t.Fatalf("reserved = %v, want the module node types %v and node, default", got, labels)
	}
	for _, name := range got {
		if _, ok := labels[name]; !ok && name != "node" && name != "default" {
			t.Errorf("%q is reserved but neither a module node type nor a template suffix", name)
		}
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] > got[i] {
			t.Fatalf("not sorted: %v", got)
		}
	}
}
