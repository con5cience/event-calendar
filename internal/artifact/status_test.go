package artifact

import (
	"encoding/json"
	"testing"
)

func TestLiteralStatusAndAdapterRejection(t *testing.T) {
	a := baseline(t)
	c := config(t)
	c.Overrides = nil
	obs := observation(t, a, "Status fixture")
	var data map[string]any
	json.Unmarshal(obs.Data, &data)
	data["status"] = "Cancelled"
	obs.Data, _ = json.Marshal(data)
	out, err := engine().Reconcile(c, nil, refresh(obs), "2026-09-08")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := EncodeArtifact(out.Artifact)
	var raw map[string]any
	json.Unmarshal(b, &raw)
	if raw["events"].([]any)[0].(map[string]any)["status"] != "Cancelled" {
		t.Fatal(string(b))
	}
	c.State = "established"
	obs.Failure = "conflicting source timestamps"
	next, err := engine().Reconcile(c, &out.Artifact, refresh(obs), "2026-09-08")
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Rejected) != 1 || next.Artifact.Events[0].ID != out.Artifact.Events[0].ID {
		t.Fatal(next)
	}
	obs.Failure = ""
	c.Overrides = []Override{{Match: Match{UpstreamID: obs.UpstreamID, Date: out.Artifact.Events[0].Date, VenueKey: c.Venue.Key}, Set: map[string]json.RawMessage{"status": json.RawMessage(`"Rescheduled"`)}}}
	next, err = engine().Reconcile(c, &out.Artifact, refresh(obs), "2026-09-08")
	if err != nil || next.Artifact.Events[0].Status != "Rescheduled" {
		t.Fatal(next, err)
	}
	c.Overrides[0].Set = nil
	c.Overrides[0].Remove = []string{"status"}
	next, err = engine().Reconcile(c, &out.Artifact, refresh(obs), "2026-09-08")
	if err != nil || next.Artifact.Events[0].Status != "" {
		t.Fatal(next, err)
	}
}
