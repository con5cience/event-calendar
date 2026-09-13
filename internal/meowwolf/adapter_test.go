package meowwolf

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"os"
	"testing"
	"time"
)

func TestMapping(t *testing.T) {
	for _, kind := range []string{"all", "restricted", "unknown", "cancelled", "sold-out", "identity", "venue", "clock", "date", "changed", "count", "override", "redirect", "unsafe-ticket", "exhibit", "bad-label"} {
		t.Run(kind, func(t *testing.T) {
			b, err := os.ReadFile("testdata/meow-wolf-denver.yaml")
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := artifact.DecodeConfigYAML(b)
			if err != nil {
				t.Fatal(err)
			}
			b, err = os.ReadFile("testdata/meow-wolf-denver.json")
			if err != nil {
				t.Fatal(err)
			}
			var s map[string]any
			if err = json.Unmarshal(b, &s); err != nil {
				t.Fatal(err)
			}
			row := s["events"].([]any)[0].(map[string]any)
			d := row["detail"].(map[string]any)
			meta := d["meta"].([]any)
			switch kind {
			case "exhibit", "bad-label":
				d["venueId"] = "017a7f54-ebc3-c5e1-1499-0887afa464fc"
				meta[1].(map[string]any)["value"] = ""
				meta[3].(map[string]any)["value"] = ""
				if kind == "bad-label" {
					row["text"].(map[string]any)["date"] = "Sep 12th Doors @ 7:00 PM"
				}
			case "restricted":
				meta[0].(map[string]any)["value"] = "18+"
				row["tags"] = []any{}
			case "unknown":
				meta[0].(map[string]any)["value"] = ""
				row["tags"] = []any{"all ages"}
			case "cancelled":
				row["text"].(map[string]any)["banner"] = "CANCELED"
				meta = append(meta, map[string]any{"metakey": "externalTicketUrl", "value": "javascript:;"})
			case "sold-out":
				row["text"].(map[string]any)["banner"] = "Sold Out"
			case "identity":
				d["id"] = "wrong"
			case "venue":
				d["venueId"] = "elsewhere"
			case "clock":
				meta[1].(map[string]any)["value"] = "7:00 PM"
			case "date":
				row["startDateTime"] = "bad"
			case "redirect":
				row["detail"] = map[string]any{"error": "unrelated redirect"}
			case "unsafe-ticket":
				meta = append(meta, map[string]any{"metakey": "externalTicketUrl", "value": "javascript:alert(1)"})
			case "override":
				cfg.Overrides = []artifact.Override{{Match: artifact.Match{UpstreamID: row["id"].(string), Date: "2026-09-12", VenueKey: "meow-wolf-denver"}, Set: map[string]json.RawMessage{"admission_policy": json.RawMessage(`{"text":"Override","category":"21+"}`)}}}
			}
			d["meta"] = meta
			s["check"] = s["events"]
			if kind == "changed" {
				s["check"] = []any{}
			}
			if kind == "count" {
				s["total"] = 2
			}
			b, _ = json.Marshal(s)
			r, err := Decode(cfg, b, time.Date(2026, 9, 11, 18, 0, 0, 0, time.UTC))
			if kind == "changed" || kind == "count" {
				if err == nil {
					t.Fatal("bad snapshot accepted")
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
			case "identity", "venue", "clock", "date", "redirect", "unsafe-ticket", "bad-label":
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
			if kind == "restricted" {
				category = "18+"
			}
			if kind == "unknown" {
				category = ""
			}
			if kind == "override" {
				category = "21+"
			}
			if p.Category != category || (p.WithAdult != nil) != (category == "All ages") {
				t.Fatal(p)
			}
			if e.Title != "Fixture & Friends" || e.Date != "2026-09-12" || e.DoorsAt != "2026-09-12T18:00:00-06:00" || e.ShowAt != "2026-09-12T19:00:00-06:00" || e.Price != nil {
				t.Fatal(e)
			}
			if kind == "cancelled" {
				if e.Status != "Cancelled" || e.TicketURL != "" {
					t.Fatal(e)
				}
			} else if e.Status != "Scheduled" || e.TicketURL != e.EventURL {
				t.Fatal(e)
			}
		})
	}
}
