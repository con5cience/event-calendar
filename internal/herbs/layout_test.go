package herbs

import (
	"os"
	"strings"
	"testing"
)

func TestVisibleRestrictions(t *testing.T) {
	b, err := os.ReadFile("testdata/calendar.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"21+", "18+ only", "Ages 18+; no minors"} {
		h := strings.ReplaceAll(string(b), "eventlist-excerpt", "eventlist-description")
		h = strings.ReplaceAll(h, "All genres welcome", s)
		rows, err := parse(h)
		if err != nil || len(rows) != 1 || rows[0].Restriction != s {
			t.Fatalf("%s: %v %#v", s, err, rows)
		}
	}
}
func TestSameDayStartAndNarrowSpaces(t *testing.T) {
	b, err := os.ReadFile("testdata/calendar.html")
	if err != nil {
		t.Fatal(err)
	}
	h := string(b)
	h = strings.ReplaceAll(h, "eventlist-event--multiday", "")
	h = strings.Replace(h, "event-time-12hr", "event-time-12hr-start", 1)
	h = strings.Replace(h, "event-time-24hr", "event-time-24hr-start", 1)
	h = strings.ReplaceAll(h, "9:30 PM", "9:30\u202fPM")
	rows, err := parse(h)
	if err != nil || len(rows) != 1 || rows[0].Clock12 != "9:30 PM" || rows[0].Clock24 != "21:30" {
		t.Fatal(rows, err)
	}
}
