package livenation

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"os"
	"testing"
)

func TestFillmoreAdmissionAndRejection(t *testing.T) {
	for _, kind := range []string{"all", "16", "18", "21", "absent", "unknown", "midnight-pass", "multiple-clocks", "offsite"} {
		t.Run(kind, func(t *testing.T) {
			_, s, row := fixture(t)
			b, err := os.ReadFile("testdata/fillmore.yaml")
			if err != nil {
				t.Fatal(err)
			}
			c, err := artifact.DecodeConfigYAML(b)
			if err != nil {
				t.Fatal(err)
			}
			row["venue"] = map[string]any{"discovery_id": "KovZpZAE6eJA"}
			info := "DOORS: 7PM SHOW: 8PM"
			switch kind {
			case "all":
				info = "ALL AGES WELCOME " + info
			case "16", "18", "21":
				info = "AGES " + kind + "+ WELCOME WITH VALID ID ONLY. NO EXCEPTIONS. " + info
			case "unknown":
				info = "Contact venue about age requirements. " + info
			case "midnight-pass":
				row["start_time_local"] = "00:00:01"
				row["start_datetime_utc"] = "2027-01-12T07:00:01Z"
				info = "2-DAY PASS AGES 16+ " + info
			case "multiple-clocks":
				info = "21+ FRIDAY DOORS: 7PM SHOW: 8PM SATURDAY DOORS: 5PM SHOW: 6PM"
			case "offsite":
				row["venue"] = map[string]any{"discovery_id": "KovZpZAFFt1A"}
			}
			row["important_info"] = info
			r, err := decode(t, c, s)
			if err != nil {
				t.Fatal(err)
			}
			out, err := (&artifact.Reconciler{}).Reconcile(c, nil, r, "2026-09-11")
			if err != nil {
				t.Fatal(err)
			}
			if kind == "midnight-pass" || kind == "multiple-clocks" || kind == "offsite" {
				if len(out.Rejected) != 1 {
					t.Fatal("invalid record accepted", out)
				}
				return
			}
			if len(out.Artifact.Events) != 1 || len(out.Rejected) != 0 {
				t.Fatal(out)
			}
			e := out.Artifact.Events[0]
			p := artifact.EffectivePolicy(out.Artifact.Venue, e)
			if e.Price != nil || e.DoorsAt != "2027-01-12T19:00:00-07:00" {
				t.Fatal(e)
			}
			for _, age := range []int{0, 2, 3, 14, 15, 16, 17} {
				allowed := false
				if p.WithAdult != nil {
					for _, r := range p.WithAdult.Ranges {
						if age >= r.MinAge && age <= r.MaxAge {
							allowed = true
						}
					}
				}
				if allowed != (kind == "all" || kind == "16" && age >= 16) {
					t.Fatalf("%s age %d: %v", kind, age, p)
				}
			}
			if (kind == "absent" || kind == "unknown") && p.Category != "" {
				t.Fatal("invented default", p)
			}
			c.Overrides = []artifact.Override{{Match: artifact.Match{UpstreamID: "1E00646DB543AD47", Date: "2027-01-12", VenueKey: "fillmore"}, Set: map[string]json.RawMessage{"admission_policy": json.RawMessage(`{"text":"Override","category":"21+"}`)}}}
			out, err = (&artifact.Reconciler{}).Reconcile(c, nil, r, "2026-09-11")
			if err != nil {
				t.Fatal(err)
			}
			p = artifact.EffectivePolicy(out.Artifact.Venue, out.Artifact.Events[0])
			if p.WithAdult != nil || p.Category != "21+" {
				t.Fatal("override lost", p)
			}
		})
	}
}
