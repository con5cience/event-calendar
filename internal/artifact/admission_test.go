package artifact

import (
	"encoding/json"
	"testing"
)

func TestReviewedAdmission(t *testing.T) {
	c := config(t)
	b, _ := json.Marshal(c)
	var raw map[string]any
	json.Unmarshal(b, &raw)
	rule := map[string]any{"url": "https://example.com/policy", "reviewed_on": "2026-09-09", "ranges": []any{map[string]any{"min_age": 11, "max_age": 15, "condition": "Ticketed adult required"}}}
	raw["admission_rules"] = map[string]any{"16+": rule}
	b, _ = json.Marshal(raw)
	c, err := DecodeConfigJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	a := baseline(t)
	for _, variant := range []string{"match", "unknown", "offsite", "override"} {
		t.Run(variant, func(t *testing.T) {
			cfg := c
			d := a.Events[0].EventData
			d.AdmissionPolicy = &AdmissionPolicy{Text: "16+", Category: "16+"}
			if variant == "unknown" {
				d.AdmissionPolicy.Category = ""
			}
			if variant == "offsite" {
				d.Venue = &Venue{Key: "other", Name: "Other", Timezone: c.Venue.Timezone}
			}
			if variant == "override" {
				cfg.Overrides = []Override{{Match: Match{a.Events[0].UpstreamID, d.Date, c.Venue.Key}, Set: map[string]json.RawMessage{"admission_policy": json.RawMessage(`{"text":"No exceptions","category":"16+"}`)}}}
			}
			data, _ := json.Marshal(d)
			out, err := engine().Reconcile(cfg, nil, refresh(Observation{UpstreamID: a.Events[0].UpstreamID, Data: data}), "2026-09-08")
			if err != nil || len(out.Rejected) != 0 {
				t.Fatalf("%v %+v", err, out)
			}
			encoded, err := EncodeArtifact(out.Artifact)
			if err != nil {
				t.Fatal(err)
			}
			var result map[string]any
			json.Unmarshal(encoded, &result)
			policy := result["events"].([]any)[0].(map[string]any)["admission_policy"].(map[string]any)
			_, present := policy["with_adult"]
			if present != (variant == "match") {
				t.Fatalf("unexpected admission: %s", encoded)
			}
		})
	}
}
