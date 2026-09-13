package clique

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"os"
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 9, 10, 23, 0, 0, 0, time.UTC)

func fixture(t *testing.T) (artifact.SourceConfig, map[string]any) {
	t.Helper()
	b, err := os.ReadFile("testdata/red-rocks.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := artifact.DecodeConfigYAML(b)
	if err != nil {
		t.Fatal(err)
	}
	event := map[string]any{"id": "101", "url": "https://www.redrocksonline.com/events/test-show/", "title": "Test &amp; Friends", "start": "2026-09-12T19:30:00-06:00"}
	s := map[string]any{"from": "2026-09-10", "through": "2027-09-10", "upcoming": []any{map[string]any{"ID": 123, "post_status": "publish", "post_type": "event", "post_title": "Test &amp; Friends", "post_content": "<p>A concert.</p>", "guid": event["url"], "acf": map[string]any{"showing_id": []string{"101"}, "event_start": []string{"2026-09-12 19:30:00"}, "event_doors_open": []string{"6:30 PM"}, "event_ticket_link": []string{"https://www.axs.com/events/123/test-tickets"}, "event_subtitle": []string{"Support"}}}}, "range": []any{event}, "windows": []any{map[string]any{"start": "2026-09-10", "end": "2027-09-11", "events": []any{event}}}}
	return cfg, s
}
func decode(t *testing.T, cfg artifact.SourceConfig, s map[string]any) (artifact.Refresh, error) {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return Decode(cfg, b, testNow)
}
func row(s map[string]any) map[string]any { return s["upcoming"].([]any)[0].(map[string]any) }
func TestMappingAndAdmission(t *testing.T) {
	cfg, s := fixture(t)
	r, err := decode(t, cfg, s)
	if err != nil {
		t.Fatal(err)
	}
	result, err := (&artifact.Reconciler{}).Reconcile(cfg, nil, r, r.Coverage.From)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rejected) != 0 || len(result.Artifact.Events) != 1 {
		t.Fatalf("%+v", result)
	}
	e := result.Artifact.Events[0]
	if e.Title != "Test & Friends" || e.DoorsAt != "2026-09-12T18:30:00-06:00" || e.ShowAt != "2026-09-12T19:30:00-06:00" || e.UpstreamID != "101" || e.Price != nil || e.OffSite {
		t.Fatalf("%+v", e)
	}
	if e.AdmissionPolicy.WithAdult == nil || e.AdmissionPolicy.WithAdult.Ranges[1].MinAge != 2 || e.AdmissionPolicy.WithAdult.Ranges[1].MaxAge != 17 {
		t.Fatalf("%+v", e.AdmissionPolicy)
	}
}
func TestRestrictionAndStatus(t *testing.T) {
	for _, tc := range []struct{ text, title, category, status string }{{"Ages 16 and up", "Test &amp; Friends", "16+", "Scheduled"}, {"18+ only", "Test &amp; Friends", "18+", "Scheduled"}, {"21+ only", "Test &amp; Friends", "21+", "Scheduled"}, {"Minimum age varies; contact venue", "Test &amp; Friends", "", "Scheduled"}, {"All ages", "Canceled: Test &amp; Friends", "All ages", "Cancelled"}, {"Previously postponed; tickets valid for this new date", "Test &amp; Friends", "All ages", "Scheduled"}} {
		t.Run(tc.text, func(t *testing.T) {
			cfg, s := fixture(t)
			row(s)["post_content"] = tc.text
			row(s)["post_title"] = tc.title
			s["range"].([]any)[0].(map[string]any)["title"] = tc.title
			r, err := decode(t, cfg, s)
			if err != nil {
				t.Fatal(err)
			}
			result, err := (&artifact.Reconciler{}).Reconcile(cfg, nil, r, r.Coverage.From)
			if err != nil || len(result.Artifact.Events) != 1 {
				t.Fatalf("%v %+v", err, result)
			}
			e := result.Artifact.Events[0]
			p := artifact.EffectivePolicy(cfg.Venue, e)
			if p.Category != tc.category || e.Status != tc.status {
				t.Fatalf("%+v %+v", e, p)
			}
			if tc.category != "All ages" && tc.category != "16+" && p.WithAdult != nil {
				t.Fatal("invented permission")
			}
		})
	}
}
func TestExplicitAgeFloorAndOverride(t *testing.T) {
	cfg, s := fixture(t)
	row(s)["post_content"] = "Age Limit: 13+"
	r, err := decode(t, cfg, s)
	if err != nil {
		t.Fatal(err)
	}
	result, err := (&artifact.Reconciler{}).Reconcile(cfg, nil, r, r.Coverage.From)
	if err != nil {
		t.Fatal(err)
	}
	p := result.Artifact.Events[0].AdmissionPolicy
	if p == nil || p.WithAdult == nil || p.WithAdult.Ranges[0].MinAge != 13 || p.WithAdult.Ranges[0].MaxAge != 17 || p.WithAdult.URL != row(s)["guid"] {
		t.Fatalf("missing ordinary 13+ permission: %+v", p)
	}
	raw, _ := json.Marshal(cfg)
	var config map[string]any
	json.Unmarshal(raw, &config)
	config["overrides"] = []any{map[string]any{"match": map[string]any{"upstream_id": "101", "date": "2026-09-12", "venue_key": "red-rocks"}, "set": map[string]any{"admission_policy": map[string]any{"text": "21+", "category": "21+"}}}}
	raw, _ = json.Marshal(config)
	cfg, err = artifact.DecodeConfigJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	result, err = (&artifact.Reconciler{}).Reconcile(cfg, nil, r, r.Coverage.From)
	if err != nil {
		t.Fatal(err)
	}
	p = result.Artifact.Events[0].AdmissionPolicy
	if p.Category != "21+" || p.WithAdult != nil {
		t.Fatalf("override lost: %+v", p)
	}
}
func TestEnvelopeFailures(t *testing.T) {
	for _, kind := range []string{"missing-range", "null-upcoming", "mismatch", "duplicate", "gap", "bad-clock", "bad-config", "trailing"} {
		t.Run(kind, func(t *testing.T) {
			cfg, s := fixture(t)
			switch kind {
			case "missing-range":
				delete(s, "range")
			case "null-upcoming":
				s["upcoming"] = nil
			case "mismatch":
				s["range"] = []any{}
			case "duplicate":
				s["upcoming"] = append(s["upcoming"].([]any), row(s))
			case "gap":
				s["windows"].([]any)[0].(map[string]any)["start"] = "2026-09-11"
			case "bad-clock":
				s["from"] = "2026-09-09"
			case "bad-config":
				cfg.Source.ID = "other"
			}
			b, _ := json.Marshal(s)
			if kind == "trailing" {
				b = append(b, []byte("{}")...)
			}
			if _, err := Decode(cfg, b, testNow); err == nil {
				t.Fatal("accepted invalid envelope")
			}
		})
	}
}
func TestRecordRejections(t *testing.T) {
	for _, kind := range []string{"doors", "offset", "title", "ticket", "multiple-starts"} {
		t.Run(kind, func(t *testing.T) {
			cfg, s := fixture(t)
			acf := row(s)["acf"].(map[string]any)
			switch kind {
			case "doors":
				acf["event_doors_open"] = []string{"10:00 PM"}
			case "offset":
				s["range"].([]any)[0].(map[string]any)["start"] = "2026-09-12T19:30:00-07:00"
			case "title":
				row(s)["post_title"] = ""
			case "ticket":
				acf["event_ticket_link"] = []string{"javascript:alert(1)"}
			case "multiple-starts":
				acf["event_start"] = []string{"a", "b"}
			}
			r, err := decode(t, cfg, s)
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Observations) != 1 || r.Observations[0].Failure == "" {
				t.Fatal("invalid record not rejected")
			}
		})
	}
}
func TestEmptyAndOverlappingWindows(t *testing.T) {
	cfg, s := fixture(t)
	e := s["range"].([]any)[0]
	s["windows"] = []any{map[string]any{"start": "2026-09-10", "end": "2026-09-12", "events": []any{e}}, map[string]any{"start": "2026-09-12", "end": "2027-09-11", "events": []any{e}}}
	r, err := decode(t, cfg, s)
	if err != nil || len(r.Observations) != 1 {
		t.Fatalf("%v %+v", err, r)
	}
	s["upcoming"] = []any{}
	s["range"] = []any{}
	for _, w := range s["windows"].([]any) {
		w.(map[string]any)["events"] = []any{}
	}
	r, err = decode(t, cfg, s)
	if err != nil || len(r.Observations) != 0 || !r.Coverage.Complete {
		t.Fatalf("%v %+v", err, r)
	}
}
func TestWinterAndOverride(t *testing.T) {
	cfg, s := fixture(t)
	e := s["range"].([]any)[0].(map[string]any)
	e["start"] = "2026-11-15T19:30:00-07:00"
	row(s)["acf"].(map[string]any)["event_start"] = []string{"2026-11-15 19:30:00"}
	r, err := decode(t, cfg, s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(r.Observations[0].Data), "-07:00") {
		t.Fatal("wrong winter offset")
	}
}
