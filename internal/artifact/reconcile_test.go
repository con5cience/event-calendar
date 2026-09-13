package artifact

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func engine() *Reconciler {
	n := 0
	return &Reconciler{NewSuffix: func() (string, error) { n++; return fmt.Sprintf("%012x", n), nil }}
}
func observation(t *testing.T, a Artifact, title string) Observation {
	t.Helper()
	data := a.Events[0].EventData
	data.Title = title
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return Observation{UpstreamID: a.Events[0].UpstreamID, Data: b}
}
func refresh(obs ...Observation) Refresh {
	return Refresh{GeneratedAt: "2026-09-08T12:00:00Z", Coverage: Coverage{From: "2026-09-08", Through: "2027-09-08", Complete: true}, Observations: obs}
}
func TestReconcileNewAndStableUpdate(t *testing.T) {
	a := baseline(t)
	c := config(t)
	r := engine()
	first, err := r.Reconcile(c, nil, refresh(observation(t, a, "First title")), "2026-09-08")
	if err != nil {
		t.Fatal(err)
	}
	e := first.Artifact.Events[0]
	if e.PublicPath != "/events/gothic/2026-09-10-first-title-000000000001" || !e.Listed {
		t.Fatalf("new event %#v", e)
	}
	c.State = "established"
	o := observation(t, a, "Corrected title")
	var d EventData
	if err = json.Unmarshal(o.Data, &d); err != nil {
		t.Fatal(err)
	}
	d.DoorsAt = "2026-09-10T18:00:00-06:00"
	o.Data, _ = json.Marshal(d)
	next, err := r.Reconcile(c, &first.Artifact, refresh(o), "2026-09-08")
	if err != nil {
		t.Fatal(err)
	}
	if next.Artifact.Events[0].PublicPath != e.PublicPath || next.Artifact.Events[0].ID != e.ID || next.Artifact.Events[0].Title != "Corrected title" {
		t.Fatal("update lost identity or content")
	}
}
func TestReconcileMovesCreateEvents(t *testing.T) {
	for _, move := range []string{"date", "venue"} {
		t.Run(move, func(t *testing.T) {
			a := baseline(t)
			c := config(t)
			c.State = "established"
			d := a.Events[0].EventData
			if move == "date" {
				d.Date = "2026-09-11"
				d.DoorsAt = "2026-09-11T19:00:00-06:00"
				d.ShowAt = ""
			} else {
				d.Venue = &Venue{Key: "mission", Name: "Mission Ballroom", Timezone: "America/Denver"}
			}
			b, _ := json.Marshal(d)
			result, err := engine().Reconcile(c, &a, refresh(Observation{UpstreamID: a.Events[0].UpstreamID, Data: b}), "2026-09-08")
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Artifact.Events) != 2 {
				t.Fatalf("events %#v", result.Artifact.Events)
			}
			for _, e := range result.Artifact.Events {
				if e.ID == a.Events[0].ID {
					if e.Listed || e.PublicPath != a.Events[0].PublicPath {
						t.Fatal("old occurrence lost retained URL")
					}
				} else if !e.Listed || e.PublicPath == a.Events[0].PublicPath {
					t.Fatal("move reused old occurrence")
				}
			}
		})
	}
}
func TestReconcileAbsenceHistoryAndExpiry(t *testing.T) {
	a := baseline(t)
	c := config(t)
	c.State = "established"
	removed, err := engine().Reconcile(c, &a, refresh(), "2026-09-08")
	if err != nil {
		t.Fatal(err)
	}
	if removed.Artifact.Events[0].Listed {
		t.Fatal("successful absence stayed listed")
	}
	later := refresh()
	later.Coverage.From = "2026-09-11"
	past, err := engine().Reconcile(c, &a, later, "2026-09-11")
	if err != nil {
		t.Fatal(err)
	}
	if !past.Artifact.Events[0].Listed {
		t.Fatal("upcoming refresh erased history")
	}
	expiredRun := refresh()
	expiredRun.Coverage.From = "2026-12-09"
	gone, err := engine().Reconcile(c, &a, expiredRun, "2026-12-09")
	if err != nil {
		t.Fatal(err)
	}
	if len(gone.Artifact.Events) != 0 {
		t.Fatal("expired event retained")
	}
}
func TestReconcileInvalidUpdatesAndPartialRejects(t *testing.T) {
	a := baseline(t)
	c := config(t)
	c.State = "established"
	before, _ := json.Marshal(a)
	bad := Observation{UpstreamID: a.Events[0].UpstreamID, Data: json.RawMessage(`{"title":null}`)}
	result, err := engine().Reconcile(c, &a, refresh(bad, Observation{UpstreamID: "new-invalid", Data: json.RawMessage(`{}`)}), "2026-09-08")
	if err != nil {
		t.Fatal(err)
	}
	expected := a.Events[0]
	expected.Price = nil
	if len(result.Rejected) != 2 || len(result.Artifact.Events) != 1 || !reflect.DeepEqual(result.Artifact.Events[0], expected) {
		t.Fatalf("unexpected result %#v", result)
	}
	after, _ := json.Marshal(a)
	if string(before) != string(after) {
		t.Fatal("mutated prior artifact")
	}
}
func TestReconcileFailsSafely(t *testing.T) {
	a := baseline(t)
	c := config(t)
	c.State = "established"
	before, _ := json.Marshal(a)
	for _, mode := range []string{"failed", "incomplete", "unidentified", "duplicate", "missing-prior", "ambiguous-invalid"} {
		t.Run(mode, func(t *testing.T) {
			prior := a
			run := refresh()
			p := &prior
			switch mode {
			case "failed":
				run.Failure = "network failure"
			case "incomplete":
				run.Coverage.Complete = false
			case "unidentified":
				run.Observations = []Observation{{Data: json.RawMessage(`{}`)}}
			case "duplicate":
				o := observation(t, a, "Duplicate")
				run.Observations = []Observation{o, o}
			case "missing-prior":
				p = nil
			case "ambiguous-invalid":
				e := a.Events[0]
				e.ID = "gothic-123456abcdef"
				e.Date = "2026-09-11"
				e.DoorsAt = ""
				e.ShowAt = ""
				e.ExpiresOn = "2026-12-10"
				e.PublicPath = "/events/gothic/2026-09-11-test-123456abcdef"
				prior.Events = append(append([]Event{}, prior.Events...), e)
				run.Observations = []Observation{{UpstreamID: e.UpstreamID, Data: json.RawMessage(`{}`)}}
			}
			if _, err := engine().Reconcile(c, p, run, "2026-09-08"); err == nil {
				t.Fatal("expected job failure")
			}
		})
	}
	after, _ := json.Marshal(a)
	if string(before) != string(after) {
		t.Fatal("failed job changed prior")
	}
}
func TestConfiguredOverridesAlwaysWin(t *testing.T) {
	c, err := DecodeConfigYAML(readFixture(t, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	a := baseline(t)
	first, err := engine().Reconcile(c, nil, refresh(observation(t, a, "Provider title")), "2026-09-08")
	if err != nil {
		t.Fatal(err)
	}
	c.State = "established"
	second, err := engine().Reconcile(c, &first.Artifact, refresh(observation(t, a, "Changed provider title")), "2026-09-08")
	if err != nil {
		t.Fatal(err)
	}
	e := second.Artifact.Events[0]
	if e.Title != "Configured title" || EffectivePolicy(c.Venue, e).Text != "18+" {
		t.Fatal("provider defeated override")
	}
	c.Overrides = nil
	third, err := engine().Reconcile(c, &second.Artifact, refresh(observation(t, a, "Now provider title")), "2026-09-08")
	if err != nil {
		t.Fatal(err)
	}
	if third.Artifact.Events[0].Title != "Now provider title" || third.Artifact.Events[0].PublicPath != e.PublicPath {
		t.Fatal("removed override did not restore provider content with stable URL")
	}
}
func TestSuffixCollisionAndSeparatePerformances(t *testing.T) {
	a := baseline(t)
	c := config(t)
	c.State = "established"
	o := observation(t, a, "Test show")
	o.UpstreamID = "second-performance"
	n := 0
	r := &Reconciler{NewSuffix: func() (string, error) {
		n++
		if n == 1 {
			return "012345abcdef", nil
		}
		return "000000000002", nil
	}}
	out, err := r.Reconcile(c, &a, refresh(observation(t, a, "Test show"), o), "2026-09-08")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Artifact.Events) != 2 || out.Artifact.Events[0].PublicPath == out.Artifact.Events[1].PublicPath {
		t.Fatal("collision or performance collapsed")
	}
}
func TestConsumerHandoff(t *testing.T) {
	a := baseline(t)
	c, err := DecodeConfigYAML(readFixture(t, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := engine().Reconcile(c, nil, refresh(observation(t, a, "Provider title")), "2026-09-08")
	if err != nil {
		t.Fatal(err)
	}
	b, err := EncodeArtifact(out.Artifact)
	if err != nil {
		t.Fatal(err)
	}
	// Optional cross-language handoff is test output, never a published artifact.
	if dir := os.Getenv("CONTRACT_HANDOFF_DIR"); dir != "" {
		ref := ArtifactRef{SourceID: c.Source.ID, Artifact: "sources/gothic/generation-001.json", SHA256: fmt.Sprintf("%x", sha256.Sum256(b))}
		catalog := Catalog{SchemaVersion: 1, Generation: "generation-001", GeneratedAt: out.Artifact.GeneratedAt, Sources: []ArtifactRef{ref}}
		catalogBytes, err := json.Marshal(catalog)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = DecodeCatalog(catalogBytes); err != nil {
			t.Fatal(err)
		}
		if err = os.MkdirAll(filepath.Dir(filepath.Join(dir, ref.Artifact)), 0755); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, ref.Artifact), b, 0644); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, "catalog.json"), catalogBytes, 0644); err != nil {
			t.Fatal(err)
		}
	}
}
