package blackbox

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"os"
	"testing"
	"time"
)

func TestBlackBoxMappingAndFailures(t *testing.T) {
	for _, kind := range []string{"main", "lounge", "unknown", "all", "vip", "offsite", "clock", "changed", "count", "date", "override", "redirect", "ticket-identity"} {
		t.Run(kind, func(t *testing.T) {
			b, err := os.ReadFile("testdata/black-box.yaml")
			if err != nil {
				t.Fatal(err)
			}
			c, err := artifact.DecodeConfigYAML(b)
			if err != nil {
				t.Fatal(err)
			}
			row := map[string]any{"id": "6a860478-c968-4422-9205-00330a1e615e", "slug": "test", "title": "Test &amp; Friends", "date": "2026-09-12", "time": "09:01 PM", "end_date": "2026-09-13", "end_time": "01:00 AM", "venue": "The Black Box", "location": "Denver", "address": "314 East 13th Avenue, Denver, 80203", "status": "published", "ticket_url": "https://events.blackboxdenver.co/e/test/tickets", "summary_html": "<p>Doors: 9:00PM<br>Music: 9:00PM<br>End: 1:00AM</p>", "age_restriction": "18+"}
			switch kind {
			case "ticket-identity":
				row["ticket_title"] = "Different event"
			case "redirect":
				row["ticket_error"] = "external ticket provider"
			case "lounge":
				row["venue"] = "The Lounge"
			case "unknown":
				row["age_restriction"] = nil
			case "all":
				row["age_restriction"] = "All Ages"
			case "vip":
				row["summary_html"] = "BASSCOUCH 21+ ONLY " + row["summary_html"].(string)
			case "offsite":
				row["address"] = "Somewhere else"
			case "clock":
				row["summary_html"] = "<p>Doors: 9PM Doors: 10PM</p>"
			case "date":
				row["date"] = "2026-02-30"
			case "override":
				c.Overrides = []artifact.Override{{Match: artifact.Match{UpstreamID: row["id"].(string), Date: "2026-09-12", VenueKey: "black-box"}, Set: map[string]json.RawMessage{"admission_policy": json.RawMessage(`{"text":"Override","category":"21+"}`)}}}
			}
			if kind != "ticket-identity" {
				row["ticket_title"] = "Test & Friends"
			}
			events := []any{row}
			s := map[string]any{"from": "2026-09-11", "through": "2027-09-11", "total": 1, "events": events, "check": events}
			if kind == "changed" {
				s["check"] = []any{}
			}
			if kind == "count" {
				s["total"] = 2
			}
			b, _ = json.Marshal(s)
			r, err := Decode(c, b, time.Date(2026, 9, 11, 18, 0, 0, 0, time.UTC))
			if kind == "changed" || kind == "count" {
				if err == nil {
					t.Fatal("invalid snapshot accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			out, err := (&artifact.Reconciler{}).Reconcile(c, nil, r, "2026-09-11")
			if err != nil {
				t.Fatal(err)
			}
			if kind == "offsite" || kind == "clock" || kind == "date" || kind == "redirect" || kind == "ticket-identity" {
				if len(out.Rejected) != 1 {
					t.Fatal(out)
				}
				return
			}
			if len(out.Artifact.Events) != 1 || len(out.Rejected) != 0 {
				t.Fatal(out)
			}
			e := out.Artifact.Events[0]
			p := artifact.EffectivePolicy(out.Artifact.Venue, e)
			if e.Title != "Test & Friends" || e.DoorsAt != "2026-09-12T21:00:00-06:00" || e.ShowAt != e.DoorsAt || e.Price != nil {
				t.Fatal(e)
			}
			category := "18+"
			if kind == "unknown" {
				category = ""
			}
			if kind == "all" {
				category = "All ages"
			}
			if kind == "override" {
				category = "21+"
			}
			if p.Category != category || (p.WithAdult != nil) != (kind == "all") {
				t.Fatal(p)
			}
			if kind == "all" && p.WithAdult.URL != e.TicketURL {
				t.Fatal("clearance must cite the event terms", p)
			}
		})
	}
}
