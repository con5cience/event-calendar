package venuepilot

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"os"
	"testing"
	"time"
)

func TestLevittMapping(t *testing.T) {
	for _, kind := range []string{"all", "restricted", "unknown", "numeric", "conflict", "cancelled", "status", "venue", "date", "clock", "winter", "missing-time", "changed", "count", "duplicate", "bounds", "override", "url", "script", "biography", "rescheduled"} {
		t.Run(kind, func(t *testing.T) {
			b, err := os.ReadFile("testdata/levitt.yaml")
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := artifact.DecodeConfigYAML(b)
			if err != nil {
				t.Fatal(err)
			}
			b, err = os.ReadFile("testdata/levitt.json")
			if err != nil {
				t.Fatal(err)
			}
			var s map[string]any
			if err = json.Unmarshal(b, &s); err != nil {
				t.Fatal(err)
			}
			row := s["events"].([]any)[0].(map[string]any)
			switch kind {
			case "biography":
				row["description"] = "<p>The band has played all ages concerts around the world.</p>"
			case "rescheduled":
				row["ticketsUrl"] = "https://tickets.venuepilot.com/e/fixture-2026-09-01"
			case "restricted":
				row["description"] = "<p>Guests must be <strong>21+</strong>.</p>"
				row["footerContent"] = "All ages"
				row["status"] = "TICKETS"
			case "unknown":
				row["description"] = "Music outdoors"
				row["footerContent"] = "All ages"
			case "numeric":
				row["description"] = "Music outdoors"
				row["minimumAge"] = 16
			case "conflict":
				row["minimumAge"] = 21
			case "cancelled":
				row["status"] = "CANCELLED"
			case "status":
				row["status"] = "PRIVATE"
			case "venue":
				row["venue"] = map[string]any{"name": "Elsewhere"}
			case "date":
				row["date"] = "2026-02-30"
			case "clock":
				row["doorTime"] = "27:00:00"
			case "winter":
				row["date"] = "2026-12-12"
			case "missing-time":
				delete(row, "doorTime")
				delete(row, "startTime")
			case "bounds":
				row["date"] = "2027-10-10"
			case "url":
				row["ticketsUrl"] = "javascript:alert(1)"
			case "script":
				row["description"] = "<script>All ages</script><p>Music outdoors</p>"
			case "override":
				cfg.Overrides = []artifact.Override{{Match: artifact.Match{UpstreamID: "123", Date: "2026-09-12", VenueKey: "levitt"}, Set: map[string]json.RawMessage{"admission_policy": json.RawMessage(`{"text":"Override","category":"21+"}`)}}}
			}
			s["check"] = s["events"]
			if kind == "changed" {
				s["check"] = []any{}
			}
			if kind == "count" {
				s["total"] = 2
			}
			if kind == "duplicate" {
				s["events"] = []any{row, row}
				s["check"] = s["events"]
				s["total"] = 2
			}
			b, _ = json.Marshal(s)
			r, err := Decode(cfg, b, time.Date(2026, 9, 11, 18, 0, 0, 0, time.UTC))
			if kind == "changed" || kind == "count" || kind == "duplicate" || kind == "bounds" {
				if err == nil {
					t.Fatal("invalid snapshot accepted")
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
			case "conflict", "status", "venue", "date", "clock", "url":
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
			category := "All ages"
			switch kind {
			case "restricted", "override":
				category = "21+"
			case "numeric":
				category = "16+"
			case "unknown", "script", "biography":
				category = ""
			}
			if p.Category != category || (p.WithAdult != nil) != (category == "All ages") {
				t.Fatal(p)
			}
			if e.Title != "Fixture & Friends" || e.EventURL != "https://www.levittdenver.org/summer-concert-series#/events/123" || e.Price != nil {
				t.Fatal(e)
			}
			if kind == "rescheduled" && (e.Date != "2026-09-12" || e.TicketURL != row["ticketsUrl"]) {
				t.Fatal("ticket URL date changed event date or link", e)
			}
			if kind == "cancelled" && e.Status != "Cancelled" {
				t.Fatal(e)
			}
			expected := "2026-09-12T18:00:00-06:00"
			if kind == "winter" {
				expected = "2026-12-12T18:00:00-07:00"
			}
			if kind == "missing-time" {
				expected = ""
			}
			if e.DoorsAt != expected {
				t.Fatal(e)
			}
		})
	}
}
