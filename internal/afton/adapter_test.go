package afton

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMapping(t *testing.T) {
	for _, kind := range []string{"normal", "external", "unknown-age", "restricted", "cancelled", "soldout", "missing-time", "winter", "gap", "fold", "clock-conflict", "title", "venue", "url", "id", "duplicate", "short", "changed", "detail-changed", "missing-detail", "empty", "override", "past"} {
		t.Run(kind, func(t *testing.T) {
			b, err := os.ReadFile("testdata/roxy.yaml")
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := artifact.DecodeConfigYAML(b)
			if err != nil {
				t.Fatal(err)
			}
			b, err = os.ReadFile("testdata/detail.html")
			if err != nil {
				t.Fatal(err)
			}
			h := string(b)
			e := map[string]any{"event_type": "real_world", "event_id": "3px8g401j1", "event_name": "Fixture & Friends", "start_time": "2026-09-26 19:00:00", "door_time": "2026-09-26 18:00:00", "venue_name": "The Roxy Theatre", "city": "Denver", "state_abbreviation": "CO", "buy_ticket_url": "https://aftontickets.com/event/buyticket/3px8g401j1", "hide_start_time": "no", "display_door_start_time": "yes", "entry_type": "door", "sold_out": false}
			switch kind {
			case "external":
				e["event_type"] = "external_event"
				e["event_id"] = 103
				e["buy_ticket_url"] = "https://tickets.strongsurvivepresents.com/tickets/464607"
			case "unknown-age":
				h = strings.ReplaceAll(h, "All Ages &amp; Bar w/ID", "Check admission")
			case "restricted":
				h = strings.ReplaceAll(h, "All Ages &amp; Bar w/ID", "21+")
			case "cancelled":
				h = strings.ReplaceAll(h, "EventScheduled", "EventCancelled")
			case "soldout":
				e["sold_out"] = true
			case "missing-time":
				e["hide_start_time"] = "yes"
				e["display_door_start_time"] = "no"
			case "winter":
				e["start_time"] = "2026-12-26 19:00:00"
				e["door_time"] = "2026-12-26 18:00:00"
				h = strings.ReplaceAll(h, "2026-09-27T01:00:00+00:00", "2026-12-27T02:00:00+00:00")
			case "gap":
				e["door_time"] = "2027-03-14 02:30:00"
			case "fold":
				e["door_time"] = "2026-11-01 01:30:00"
			case "clock-conflict":
				e["start_time"] = "2026-09-26 20:00:00"
			case "title":
				e["event_name"] = ""
				h = strings.ReplaceAll(h, "Fixture & Friends", "")
			case "venue":
				h = strings.ReplaceAll(h, "2549 Welton St", "Other address")
			case "url":
				e["buy_ticket_url"] = "https://evil.example/event"
			case "id":
				e["event_id"] = ""
			case "override":
				cfg.Overrides = []artifact.Override{{Match: artifact.Match{UpstreamID: "3px8g401j1", Date: "2026-09-26", VenueKey: "roxy"}, Set: map[string]json.RawMessage{"admission_policy": json.RawMessage(`{"text":"21+","category":"21+"}`)}}}
			case "past":
				e["start_time"] = "2026-09-10 19:00:00"
				e["door_time"] = "2026-09-10 18:00:00"
				h = strings.ReplaceAll(h, "2026-09-27T01:00:00+00:00", "2026-09-11T01:00:00+00:00")
			}
			events := []any{e}
			total := 1
			if kind == "duplicate" {
				events = append(events, e)
				total = 2
			}
			if kind == "short" {
				total = 2
			}
			if kind == "empty" {
				events = []any{}
				total = 0
			}
			pages := []any{map[string]any{"page": 1, "total": total, "per_page": 12, "next": false, "events": events}}
			details := map[string]string{"3px8g401j1": h}
			if kind == "external" || kind == "missing-detail" {
				details = map[string]string{}
			}
			s := map[string]any{"from": "2026-09-12", "through": "2027-09-12", "pages": pages, "check_pages": pages, "details": details, "check_details": details}
			if kind == "changed" {
				s["check_pages"] = []any{}
			}
			if kind == "detail-changed" {
				s["check_details"] = map[string]string{"3px8g401j1": strings.ReplaceAll(h, "All Ages &amp; Bar w/ID", "21+")}
			}
			b, _ = json.Marshal(s)
			r, err := Decode(cfg, b, time.Date(2026, 9, 12, 18, 0, 0, 0, time.UTC))
			switch kind {
			case "id", "url", "duplicate", "short", "changed", "detail-changed", "missing-detail", "empty":
				if err == nil {
					t.Fatal("unsafe snapshot accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			out, err := (&artifact.Reconciler{}).Reconcile(cfg, nil, r, "2026-09-12")
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "gap", "fold", "clock-conflict", "title", "venue":
				if len(out.Rejected) != 1 {
					t.Fatal(out)
				}
				return
			case "past":
				if len(out.Artifact.Events) != 0 {
					t.Fatal(out)
				}
				return
			}
			if len(out.Artifact.Events) != 1 || len(out.Rejected) != 0 {
				t.Fatal(out)
			}
			v := out.Artifact.Events[0]
			if v.Price != nil {
				t.Fatal(v)
			}
			if kind == "external" {
				if v.ShowAt != "" || v.DoorsAt != "" || v.AdmissionPolicy != nil {
					t.Fatal(v)
				}
				return
			}
			p := artifact.EffectivePolicy(out.Artifact.Venue, v)
			allowed := kind != "unknown-age" && kind != "restricted" && kind != "cancelled" && kind != "override"
			if (p.WithAdult != nil) != allowed {
				t.Fatal(p)
			}
			wantShow, wantDoors := "2026-09-26T19:00:00-06:00", "2026-09-26T18:00:00-06:00"
			if kind == "winter" {
				wantShow, wantDoors = "2026-12-26T19:00:00-07:00", "2026-12-26T18:00:00-07:00"
			}
			if kind == "missing-time" {
				wantShow, wantDoors = "", ""
			}
			if v.ShowAt != wantShow || v.DoorsAt != wantDoors {
				t.Fatal(v)
			}
			if kind == "cancelled" && v.Status != "Cancelled" {
				t.Fatal(v)
			}
		})
	}
}
