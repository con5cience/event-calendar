package venuepilot

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"os"
	"testing"
	"time"
)

func TestDazzleAdmission(t *testing.T) {
	for _, kind := range []string{"all", "no-cover", "restricted", "unknown", "before", "cutoff", "after", "missing", "doors-only", "cancelled", "override", "conflict", "winter", "fold", "separate", "retention", "membership"} {
		t.Run(kind, func(t *testing.T) {
			b, err := os.ReadFile("testdata/levitt.yaml")
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := artifact.DecodeConfigYAML(b)
			if err != nil {
				t.Fatal(err)
			}
			cfg.Source.ID = "dazzle"
			cfg.Venue.Key = "dazzle"
			cfg.Venue.Name = "Dazzle @ The Arts Complex"
			cfg.AdapterOptions = map[string]string{"event_base": "https://www.dazzledenver.com/live-music/#/events/", "layout": "dazzle"}
			cfg.AdmissionRules = nil
			cfg.Venue.AdmissionPolicy = &artifact.AdmissionPolicy{Text: "All ages until 11 PM; event restrictions apply", URL: "https://www.dazzledenver.com/faq/"}
			row := map[string]any{"id": 123, "name": "Fixture Jazz", "date": "2026-09-15", "doorTime": "18:00:00", "startTime": "19:00:00", "minimumAge": 0, "status": "Tickets", "description": "", "ticketsUrl": nil, "venue": map[string]string{"name": cfg.Venue.Name}}
			allowed := true
			switch kind {
			case "membership":
				row["name"] = "Dazzle Membership"
			case "no-cover":
				row["status"] = "NO COVER"
			case "restricted":
				row["minimumAge"] = 21
				allowed = false
			case "unknown":
				row["minimumAge"] = nil
				allowed = false
			case "before":
				row["startTime"] = "22:59:00"
			case "cutoff":
				row["startTime"] = "23:00:00"
				allowed = false
			case "after":
				row["startTime"] = "23:30:00"
				allowed = false
			case "missing":
				row["startTime"] = nil
				row["doorTime"] = nil
				allowed = false
			case "doors-only":
				row["startTime"] = nil
				allowed = false
			case "cancelled":
				row["status"] = "Cancelled"
				allowed = false
			case "winter":
				row["date"] = "2026-12-15"
			case "fold":
				row["date"] = "2026-11-01"
				row["startTime"] = "01:30:00"
			case "conflict":
				row["description"] = "<p>Guests must be 21+</p>"
			case "override":
				cfg.Overrides = []artifact.Override{{Match: artifact.Match{UpstreamID: "123", Date: "2026-09-15", VenueKey: "dazzle"}, Set: map[string]json.RawMessage{"admission_policy": json.RawMessage(`{"text":"Override","category":"21+"}`)}}}
				allowed = false
			}
			rows := []any{row}
			if kind == "separate" {
				other := map[string]any{}
				for k, v := range row {
					other[k] = v
				}
				other["id"] = 124
				other["startTime"] = "21:00:00"
				rows = append(rows, other)
			}
			now := time.Date(2026, 9, 14, 18, 0, 0, 0, time.UTC)
			decode := func(rows []any) artifact.Refresh {
				t.Helper()
				raw, _ := json.Marshal(map[string]any{"from": "2026-09-14", "through": "2027-09-14", "total": len(rows), "events": rows, "check": rows})
				r, e := Decode(cfg, raw, now)
				if e != nil {
					t.Fatal(e)
				}
				return r
			}
			out, err := (&artifact.Reconciler{}).Reconcile(cfg, nil, decode(rows), "2026-09-14")
			if err != nil {
				t.Fatal(err)
			}
			if kind == "conflict" || kind == "fold" || kind == "membership" {
				if len(out.Rejected) != 1 {
					t.Fatal("expected rejection", out)
				}
				return
			}
			if len(out.Rejected) != 0 || len(out.Artifact.Events) != len(rows) {
				t.Fatal(out)
			}
			e := out.Artifact.Events[0]
			for _, event := range out.Artifact.Events {
				if event.UpstreamID == "123" {
					e = event
				}
			}
			p := artifact.EffectivePolicy(out.Artifact.Venue, e)
			if (p.WithAdult != nil) != allowed {
				t.Fatalf("clearance: %+v", p)
			}
			if allowed && (len(p.WithAdult.Ranges) != 1 || p.WithAdult.Ranges[0].Condition != "Under 21 must leave by 11 PM" || p.WithAdult.Ranges[0].MaxAge != 17) {
				t.Fatal(p)
			}
			if e.TicketURL != "" || e.Price != nil || e.EventURL != "https://www.dazzledenver.com/live-music/#/events/123" {
				t.Fatal(e)
			}
			if kind == "retention" {
				cfg.State = "established"
				row["description"] = "<p>Guests must be 21+</p>"
				retained, err := (&artifact.Reconciler{}).Reconcile(cfg, &out.Artifact, decode(rows), "2026-09-14")
				if err != nil || len(retained.Artifact.Events) != 1 {
					t.Fatal(err, retained)
				}
				removed, err := (&artifact.Reconciler{}).Reconcile(cfg, &out.Artifact, decode([]any{}), "2026-09-14")
				if err != nil {
					t.Fatal(err)
				}
				for _, e := range removed.Artifact.Events {
					if e.Listed {
						t.Fatal("absent record still listed")
					}
				}
			}
		})
	}
}
