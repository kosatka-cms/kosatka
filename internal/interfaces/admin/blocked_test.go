package admin

import (
	"testing"
)

func TestParseBlockedBotBulk(t *testing.T) {
	input := "GoodBot\n# comment line\n\n  BadBot  \n#another comment\nThirdBot"
	got := parseBlockedBotBulk(input)
	want := []string{"GoodBot", "BadBot", "ThirdBot"}

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseBlockedBotBulk_Empty(t *testing.T) {
	got := parseBlockedBotBulk("\n\n   \n")
	if len(got) != 0 {
		t.Fatalf("expected no patterns, got %v", got)
	}
}
