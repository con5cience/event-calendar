package ophelias

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMapping(t *testing.T) {
	for _, kind := range []string{"normal", "16", "16unknown", "21", "missing-age", "unknown", "conflict", "cancelled", "soldout", "winter", "missing-time", "invalid-time", "date", "duplicate", "changed", "empty", "layout", "pagination", "url", "override", "outside", "script", "missing-title"} {
		t.Run(kind, func(t *testing.T) {
			b, _ := os.ReadFile("testdata/ophelias.yaml")
			cfg, err := artifact.DecodeConfigYAML(b)
			if err != nil {
				t.Fatal(err)
			}
			b, _ = os.ReadFile("testdata/listing.html")
			h := string(b)
			switch kind {
			case "16", "16unknown":
				h = strings.ReplaceAll(h, "18+", "16+")
				if kind == "16" {
					h = strings.Replace(h, "with valid ID.", "Anyone under 16 must be accompanied by a legal guardian.", 1)
				}
			case "21":
				h = strings.ReplaceAll(h, "18+", "21+")
			case "missing-age":
				h = strings.ReplaceAll(h, "Ages 18+", "")
			case "unknown":
				h = strings.ReplaceAll(h, "Ages 18+", "Invitation only")
			case "conflict":
				h = strings.Replace(h, "This event is Ages 18+", "This event is Ages 21+", 1)
			case "cancelled":
				h = strings.Replace(h, "Buy Tickets", "Cancelled", 1)
			case "soldout":
				h = strings.Replace(h, "Buy Tickets", "Sold Out", 1)
			case "winter":
				h = strings.ReplaceAll(h, "September", "December")
			case "missing-time":
				h = strings.ReplaceAll(h, "08:00 PM", "")
				h = strings.ReplaceAll(h, "Doors: 7pm / Show: 8pm.", "")
			case "invalid-time":
				h = strings.ReplaceAll(h, "08:00 PM", "29:00 PM")
			case "date":
				h = strings.ReplaceAll(h, ">12<", ">32<")
			case "duplicate":
				h = strings.Replace(h, "</body>", h+"</body>", 1)
			case "empty":
				h = `<div id="primary" class="eventspage"><h1>Upcoming Events</h1></div>`
			case "layout":
				h = strings.ReplaceAll(h, "events-item", "changed-item")
			case "pagination":
				h = strings.Replace(h, "</body>", `<a rel="next" href="/calendar/page/2">Next</a></body>`, 1)
			case "url":
				h = strings.ReplaceAll(h, "https://www.ticketmaster.com/fixture/event/1E0064D0BB2EA1FC", "javascript:alert(1)")
			case "override":
				cfg.Overrides = []artifact.Override{{Match: artifact.Match{UpstreamID: "1E0064D0BB2EA1FC", Date: "2026-09-12", VenueKey: "ophelias"}, Set: map[string]json.RawMessage{"admission_policy": json.RawMessage(`{"text":"Override","category":"21+"}`)}}}
			case "outside":
				h = strings.ReplaceAll(h, "2026", "2028")
			case "script":
				h = strings.ReplaceAll(h, "with valid ID.", `with valid ID.<script>Anyone under 16 must be accompanied by a guardian.</script>`)
			case "missing-title":
				h = strings.ReplaceAll(h, "Fixture &amp; Friends", "")
			}
			check := h
			if kind == "changed" {
				check = strings.Replace(h, "Fixture &amp; Friends", "Changed", 1)
			}
			b, _ = json.Marshal(map[string]any{"from": "2026-09-11", "through": "2027-09-11", "html": h, "check_html": check})
			r, err := Decode(cfg, b, time.Date(2026, 9, 11, 18, 0, 0, 0, time.UTC))
			switch kind {
			case "duplicate", "changed", "empty", "layout", "pagination", "url":
				if err == nil {
					t.Fatal("unsafe snapshot accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			out, err := (&artifact.Reconciler{}).Reconcile(cfg, nil, r, "2026-09-11")
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "conflict", "invalid-time", "date", "missing-title":
				if len(out.Rejected) != 1 {
					t.Fatal(out)
				}
				return
			case "outside":
				if len(out.Artifact.Events) != 0 {
					t.Fatal(out)
				}
				return
			}
			if len(out.Artifact.Events) != 1 || len(out.Rejected) != 0 {
				t.Fatal(out)
			}
			e := out.Artifact.Events[0]
			p := artifact.EffectivePolicy(out.Artifact.Venue, e)
			want := "18+"
			switch kind {
			case "16", "16unknown":
				want = "16+"
			case "21", "missing-age", "override":
				want = "21+"
			case "unknown":
				want = ""
			}
			if p.Category != want || (p.WithAdult != nil) != (want == "18+" || kind == "16") {
				t.Fatal(p)
			}
			if p.WithAdult != nil {
				for _, age := range []int{12, 13, 14, 15, 16, 17, 18} {
					allowed := false
					for _, r := range p.WithAdult.Ranges {
						allowed = allowed || (age >= r.MinAge && age <= r.MaxAge)
					}
					if allowed != (age >= 13 && age <= 17) {
						t.Fatal(age, p)
					}
				}
			}
			if e.Title != "Fixture & Friends" || e.Price != nil {
				t.Fatal(e)
			}
			doors := "2026-09-12T19:00:00-06:00"
			if kind == "winter" {
				doors = "2026-12-12T19:00:00-07:00"
			}
			if kind == "missing-time" {
				doors = ""
			}
			if e.DoorsAt != doors {
				t.Fatal(e)
			}
			if kind == "cancelled" && (e.Status != "Cancelled" || e.TicketURL != "") {
				t.Fatal(e)
			}
		})
	}
}

func TestMalformedClockDoesNotPanic(t *testing.T) {
	b, _ := os.ReadFile("testdata/listing.html")
	rows, err := parse(string(b))
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("America/Denver")
	for _, bad := range []string{"x", "1", "PM", "TBA", "25:30PM"} {
		t.Run(bad, func(t *testing.T) {
			e := rows[0]
			e.Clock = bad
			if _, err := normalize(e, loc); err == nil {
				t.Fatal("bad clock accepted")
			}
		})
	}
}
