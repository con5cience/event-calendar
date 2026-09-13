package livenation

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"os"
	"testing"
)

func TestSummitRoomsAndAdmission(t *testing.T) {
	for _, venue := range []string{"KovZpZAFFt1A", "KovZ917AQXY", "KovZpZAJeFkA"} {
		for _, policy := range []string{"ALL AGES", "18+", "16+", "unknown", ""} {
			t.Run(venue+"/"+policy, func(t *testing.T) {
				_, s, row := fixture(t)
				b, err := os.ReadFile("testdata/summit.yaml")
				if err != nil {
					t.Fatal(err)
				}
				c, err := artifact.DecodeConfigYAML(b)
				if err != nil {
					t.Fatal(err)
				}
				row["venue"] = map[string]any{"discovery_id": venue}
				row["important_info"] = "DOORS: 7PM SHOW: 8PM"
				if policy != "" {
					row["important_info"] = row["important_info"].(string) + " THIS SHOW IS: " + policy
				}
				r, err := decode(t, c, s)
				if err != nil {
					t.Fatal(err)
				}
				out, err := (&artifact.Reconciler{}).Reconcile(c, nil, r, "2026-09-11")
				if err != nil {
					t.Fatal(err)
				}
				if venue == "KovZpZAJeFkA" {
					if len(out.Rejected) != 1 {
						t.Fatal("accepted unrelated venue")
					}
					return
				}
				if len(out.Rejected) != 0 || len(out.Artifact.Events) != 1 {
					t.Fatal(out)
				}
				e := out.Artifact.Events[0]
				p := artifact.EffectivePolicy(out.Artifact.Venue, e)
				if e.Price != nil || e.DoorsAt != "2027-01-12T19:00:00-07:00" || e.ShowAt != "2027-01-12T20:00:00-07:00" {
					t.Fatal(e)
				}
				allowed := policy == "ALL AGES" || policy == ""
				if (p.WithAdult != nil) != allowed {
					t.Fatal("unexpected admission", p)
				}
				if allowed && (p.Category != "All ages" || p.WithAdult.URL != "https://www.summitdenver.com/visit") {
					t.Fatal(p)
				}
				c.Overrides = []artifact.Override{{Match: artifact.Match{UpstreamID: "1E00646DB543AD47", Date: "2027-01-12", VenueKey: "summit"}, Set: map[string]json.RawMessage{"admission_policy": json.RawMessage(`{"text":"Override","category":"21+"}`)}}}
				out, err = (&artifact.Reconciler{}).Reconcile(c, nil, r, "2026-09-11")
				if err != nil {
					t.Fatal(err)
				}
				p = artifact.EffectivePolicy(out.Artifact.Venue, out.Artifact.Events[0])
				if p.Category != "21+" || p.WithAdult != nil {
					t.Fatal("override lost", p)
				}
			})
		}
	}
}
