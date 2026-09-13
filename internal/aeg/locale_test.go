package aeg

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"os"
	"strings"
	"testing"
	"time"
)

func TestConfiguredNonDenverVenue(t *testing.T) {
	c := artifact.SourceConfig{SchemaVersion: 1, Source: artifact.Source{ID: "harbor", Adapter: "aeg-json"}, State: "new", Venue: artifact.Venue{Key: "harbor", Name: "Harbor", Website: "https://harbor.example", Timezone: "Pacific/Auckland"}, AdapterOptions: map[string]string{"feed_id": "987", "venue_id": "654"}}
	if _, err := Decode(c, []byte(`{"Meta":{"Total":0,"Page":1,"Rows":100},"Events":[]}`), time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestNonDenverEventReconciliation(t *testing.T) {
	c := artifact.SourceConfig{SchemaVersion: 1, Source: artifact.Source{ID: "harbor", Adapter: "aeg-json"}, State: "new", Venue: artifact.Venue{Key: "harbor", Name: "Harbor", Website: "https://harbor.example", Timezone: "Pacific/Auckland"}, AdapterOptions: map[string]string{"feed_id": "987", "venue_id": "654"}}
	b, err := os.ReadFile("testdata/gothic.json")
	if err != nil {
		t.Fatal(err)
	}
	b = []byte(strings.NewReplacer("2274", "654", "Gothic Theatre", "Harbor", "America/Denver", "Pacific/Auckland", "-06:00", "+12:00", "2026-09-13T02:00:00", "2026-09-12T08:00:00", "2026-09-13T01:00:00", "2026-09-12T07:00:00").Replace(string(b)))
	r, err := Decode(c, b, time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	out, err := (&artifact.Reconciler{}).Reconcile(c, nil, r, r.Coverage.From)
	if err != nil || len(out.Rejected) != 0 || len(out.Artifact.Events) != 1 {
		t.Fatal(err, out)
	}
	e := out.Artifact.Events[0]
	if e.DoorsAt != "2026-09-12T19:00:00+12:00" || e.ShowAt != "2026-09-12T20:00:00+12:00" || !strings.HasPrefix(e.EventURL, c.Venue.Website) || !strings.HasPrefix(e.PublicPath, "/events/harbor/") {
		t.Fatal(e)
	}
	encoded, err := artifact.EncodeArtifact(out.Artifact)
	if err != nil {
		t.Fatal(err)
	}
	var decoded artifact.Artifact
	if err := json.Unmarshal(encoded, &decoded); err != nil || decoded.Venue.Timezone != c.Venue.Timezone {
		t.Fatal(err, decoded)
	}
}
