package polls

import (
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// TestParsePollDatetimeLocalSiteZone - a datetime-local value is read in
// the request's time zone (as the edit form shows it).
func TestParsePollDatetimeLocalSiteZone(t *testing.T) {
	moscow, err := sitetime.Load("Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParsePollDatetimeLocal("2027-01-02T10:00", moscow)
	if err != nil || got == nil || !got.Equal(time.Date(2027, 1, 2, 7, 0, 0, 0, time.UTC)) {
		t.Fatalf("got %v, %v", got, err)
	}
	if got, err := ParsePollDatetimeLocal("", moscow); got != nil || err != nil {
		t.Fatalf("empty: %v, %v", got, err)
	}
}
