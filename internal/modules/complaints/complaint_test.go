package complaints

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

// TestReasonList - the settings' reasons, one per line (blank lines,
// spaces, repeats and "Other" itself dropped, a long one cut), the
// defaults when none, "Other" always last.
func TestReasonList(t *testing.T) {
	want := append(slices.Clone(DefaultReasons), OtherReason)
	if got := (Settings{}).ReasonList(); !slices.Equal(got, want) {
		t.Errorf("no reasons: %q, want the defaults %q", got, want)
	}
	if got := (Settings{Reasons: " \n\n "}).ReasonList(); !slices.Equal(got, want) {
		t.Errorf("blank reasons: %q, want the defaults", got)
	}
	got := (Settings{Reasons: " Off topic \nSpoilers\n\nOther\nSpoilers\n"}).ReasonList()
	if !slices.Equal(got, []string{"Off topic", "Spoilers", OtherReason}) {
		t.Errorf("own reasons: %q", got)
	}
	long := (Settings{Reasons: strings.Repeat("я", reasonMax+20)}).ReasonList()
	if len(long) != 2 || utf8.RuneCountInString(long[0]) != reasonMax {
		t.Errorf("a long reason: %d characters, want %d", utf8.RuneCountInString(long[0]), reasonMax)
	}
}

// TestNodeTypeAllowed - only the types chosen take complaints.
func TestNodeTypeAllowed(t *testing.T) {
	s := Settings{NodeTypes: []string{"article", "forum"}}
	if !s.NodeTypeAllowed("forum") || s.NodeTypeAllowed("page") || (Settings{}).NodeTypeAllowed("article") {
		t.Error("NodeTypeAllowed")
	}
}
