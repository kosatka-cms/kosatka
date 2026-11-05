package database

import "testing"

func TestNodeDeleteSteps(t *testing.T) {
	if last := nodeDeleteSteps[len(nodeDeleteSteps)-1].table; last != "nodes" {
		t.Fatalf("the nodes row must be deleted last, got %q", last)
	}
	if first := nodeDeleteSteps[0].table; first != "field_value_reference" {
		t.Fatalf("references to the node (NO ACTION FK) must be cleared first, got %q", first)
	}
}
