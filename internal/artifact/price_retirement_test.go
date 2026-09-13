package artifact

import (
	"encoding/json"
	"testing"
)

func TestPriceRetiredAcrossReconciliation(t *testing.T) {
	for _, mode := range []string{"new", "updated", "invalid", "absent", "past", "override"} {
		t.Run(mode, func(t *testing.T) {
			a, c := baseline(t), config(t)
			if a.Events[0].Price == nil {
				t.Fatal("legacy fixture must contain a price")
			}
			obs := observation(t, a, "Updated title")
			run, today := refresh(obs), "2026-09-08"
			var prior *Artifact
			if mode != "new" {
				c.State = "established"
				prior = &a
			}
			switch mode {
			case "invalid":
				run.Observations[0].Failure = "upstream error"
			case "absent":
				run.Observations = nil
			case "past":
				run.Observations = nil
				today = "2026-09-11"
				run.Coverage.From = today
			case "override":
				c.Overrides = []Override{{Match: Match{a.Events[0].UpstreamID, a.Events[0].Date, c.Venue.Key}, Set: map[string]json.RawMessage{"price": json.RawMessage(`{"text":"$999"}`)}}}
			}
			out, err := engine().Reconcile(c, prior, run, today)
			if err != nil || len(out.Artifact.Events) != 1 {
				t.Fatalf("%v %+v", err, out)
			}
			if out.Artifact.Events[0].Price != nil {
				t.Fatal("price survived reconciliation")
			}
			if a.Events[0].Price == nil {
				t.Fatal("prior artifact mutated")
			}
			if prior != nil && out.Artifact.Events[0].PublicPath != a.Events[0].PublicPath {
				t.Fatal("identity changed")
			}
		})
	}
}
