package artifact

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestLongestSourceKey(t *testing.T) {
	a := baseline(t)
	c := config(t)
	c.Source.ID = strings.Repeat("a", 100)
	if _, err := engine().Reconcile(c, nil, refresh(observation(t, a, "Title")), "2026-09-08"); err != nil {
		t.Fatal(err)
	}
}

func TestCoverageAndReappearance(t *testing.T) {
	a := baseline(t)
	c := config(t)
	c.State = "established"
	r := engine()
	limited := refresh()
	limited.Coverage.Through = "2026-09-09"
	unchanged, err := r.Reconcile(c, &a, limited, "2026-09-08")
	if err != nil {
		t.Fatal(err)
	}
	if !unchanged.Artifact.Events[0].Listed {
		t.Fatal("removed an event outside the completed coverage")
	}
	removed, err := r.Reconcile(c, &a, refresh(), "2026-09-08")
	if err != nil {
		t.Fatal(err)
	}
	back, err := r.Reconcile(c, &removed.Artifact, refresh(observation(t, a, "Back again")), "2026-09-08")
	if err != nil {
		t.Fatal(err)
	}
	if !back.Artifact.Events[0].Listed || back.Artifact.Events[0].PublicPath != a.Events[0].PublicPath {
		t.Fatal("reappearance changed URL or remained hidden")
	}
}
func TestMixedGoodAndBadRecords(t *testing.T) {
	a := baseline(t)
	c := config(t)
	c.State = "established"
	good := observation(t, a, "New valid event")
	good.UpstreamID = "new-valid"
	bad := observation(t, a, "")
	out, err := engine().Reconcile(c, &a, refresh(good, bad, Observation{UpstreamID: "new-bad", Data: json.RawMessage(`{}`)}), "2026-09-08")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Rejected) != 2 || len(out.Artifact.Events) != 2 {
		t.Fatal("failed to publish valid peers and retain known invalid")
	}
	for _, e := range out.Artifact.Events {
		if !e.Listed {
			t.Fatal("invalid update counted as absence")
		}
	}
}
func TestOverridesRepairAndRemoveFields(t *testing.T) {
	a := baseline(t)
	c := config(t)
	c.Overrides = []Override{{Match: Match{UpstreamID: "provider-1", Date: "2026-09-10", VenueKey: "gothic"}, Set: map[string]json.RawMessage{"title": json.RawMessage(`"Repaired"`)}, Remove: []string{"price", "doors_at", "show_at"}}}
	out, err := engine().Reconcile(c, nil, refresh(observation(t, a, "")), "2026-09-08")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Rejected) != 0 || out.Artifact.Events[0].Title != "Repaired" || out.Artifact.Events[0].Price != nil || out.Artifact.Events[0].DoorsAt != "" {
		t.Fatal("override did not precede validation or remove fields")
	}
	c.Overrides[0].Set["date"] = json.RawMessage(`"2026-09-11"`)
	if _, err := engine().Reconcile(c, nil, refresh(observation(t, a, "Title")), "2026-09-08"); err == nil {
		t.Fatal("typed config bypassed forbidden override validation")
	}
}
func TestNoAliasingOrPublicationOnFailure(t *testing.T) {
	a := baseline(t)
	c := config(t)
	c.State = "established"
	r := &Reconciler{NewSuffix: func() (string, error) { return "", errors.New("entropy unavailable") }}
	fresh := observation(t, a, "New")
	fresh.UpstreamID = "new"
	result, err := r.Reconcile(c, &a, refresh(fresh), "2026-09-08")
	if err == nil || result.Artifact.Events != nil {
		t.Fatal("failed assignment produced a publishable candidate")
	}
	out, err := engine().Reconcile(c, &a, refresh(observation(t, a, "Updated")), "2026-09-08")
	if err != nil {
		t.Fatal(err)
	}
	out.Artifact.Venue.AdmissionPolicy.Text = "mutated"
	out.Artifact.Events[0].Performers[0] = "mutated"
	if c.Venue.AdmissionPolicy.Text == "mutated" || a.Events[0].Performers[0] == "mutated" {
		t.Fatal("result aliases inputs")
	}
}
func TestSourceAndPriorGuards(t *testing.T) {
	a := baseline(t)
	c := config(t)
	if _, err := engine().Reconcile(c, &a, refresh(), "2026-09-08"); err == nil {
		t.Fatal("new source overwrote prior")
	}
	c.State = "established"
	a.SchemaVersion = 2
	if _, err := engine().Reconcile(c, &a, refresh(), "2026-09-08"); err == nil {
		t.Fatal("invalid prior silently reset")
	}
	a = baseline(t)
	c.Source.Adapter = "other"
	if _, err := engine().Reconcile(c, &a, refresh(), "2026-09-08"); err == nil {
		t.Fatal("adapter identity changed")
	}
}
func TestBoundedCollisionAndRandomIdentity(t *testing.T) {
	a := baseline(t)
	c := config(t)
	c.State = "established"
	o := observation(t, a, "New")
	o.UpstreamID = "second"
	r := &Reconciler{NewSuffix: func() (string, error) { return "012345abcdef", nil }}
	if _, err := r.Reconcile(c, &a, refresh(o), "2026-09-08"); err == nil {
		t.Fatal("collision did not stop")
	}
	c.State = "new"
	out, err := (&Reconciler{}).Reconcile(c, nil, refresh(o), "2026-09-08")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.Artifact.Events[0].ID, "gothic-") {
		t.Fatal("identity is not source-scoped")
	}
}
func TestPublishedIdentityCannotChangeSource(t *testing.T) {
	a := baseline(t)
	a.Events[0].ID = "mission-012345abcdef"
	if _, err := EncodeArtifact(a); err == nil {
		t.Fatal("accepted another source's event ID")
	}
	a = baseline(t)
	a.Events[0].PublicPath = "/events/mission/2026-09-10-test-show-012345abcdef"
	if _, err := EncodeArtifact(a); err == nil {
		t.Fatal("accepted another venue's path")
	}
}

func TestInvalidVenueMoveDoesNotMatchOldOccurrence(t *testing.T) {
	a := baseline(t)
	c := config(t)
	c.State = "established"
	a.Events[0].Venue = &Venue{Key: "warehouse", Name: "Warehouse", Timezone: "America/Denver"}
	a.Events[0].OffSite = true
	a.Events[0].PublicPath = strings.Replace(a.Events[0].PublicPath, "/gothic/", "/warehouse/", 1)
	d := a.Events[0].EventData
	d.Venue = nil
	d.Title = ""
	b, _ := json.Marshal(d)
	out, err := engine().Reconcile(c, &a, refresh(Observation{UpstreamID: "provider-1", Data: b}), "2026-09-08")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Rejected) != 1 || len(out.Artifact.Events) != 1 || out.Artifact.Events[0].Listed {
		t.Fatal("invalid new venue was mistaken for an invalid update to old venue")
	}
}
