package ratings

import (
	"math"
	"testing"
)

// TestWeighted - two fives don't outrank forty votes of 4.8 (the site's
// average 3.5 of 5, 5 votes for full weight); with enough votes a node's
// own average counts nearly fully; no votes and no threshold - 0.
func TestWeighted(t *testing.T) {
	site := 0.7
	twoFives := Weighted(Stats{Count: 2, Share: 1}, site, 5) * 5
	fortyGood := Weighted(Stats{Count: 40, Share: 0.96}, site, 5) * 5
	if twoFives >= fortyGood {
		t.Errorf("two fives %.2f outrank forty votes of 4.8 %.2f", twoFives, fortyGood)
	}
	if math.Abs(twoFives-3.93) > 0.01 || math.Abs(fortyGood-4.65) > 0.01 {
		t.Errorf("weighted: %.3f and %.3f, want 3.93 and 4.65", twoFives, fortyGood)
	}
	if got := Weighted(Stats{}, site, 0); got != 0 {
		t.Errorf("no votes, no threshold: %v", got)
	}
}

// TestVoteOn - a vote keeps its share when the scale changes, rounded to
// a whole star, at least one.
func TestVoteOn(t *testing.T) {
	for _, c := range []struct {
		vote  Vote
		scale int
		want  int
	}{
		{Vote{Score: 4, Scale: 5}, 10, 8},
		{Vote{Score: 4, Scale: 5}, 5, 4},
		{Vote{Score: 3, Scale: 3}, 5, 5},
		{Vote{Score: 1, Scale: 10}, 3, 1},
		{Vote{}, 5, 0},
	} {
		if got := c.vote.On(c.scale); got != c.want {
			t.Errorf("%+v on %d = %d, want %d", c.vote, c.scale, got, c.want)
		}
	}
}

func TestFormatAverage(t *testing.T) {
	if got := formatAverage(4.56, ","); got != "4,6" {
		t.Errorf("4.56 with a comma = %q", got)
	}
	if got := formatAverage(3, "."); got != "3.0" {
		t.Errorf("3 = %q", got)
	}
	if ValidScale(2) || ValidScale(11) || !ValidScale(3) || !ValidScale(10) {
		t.Error("ValidScale: 3 to 10")
	}
}
