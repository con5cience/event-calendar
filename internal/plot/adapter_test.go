package plot

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"os"
	"testing"
	"time"
)

var now = time.Date(2026, 9, 11, 18, 0, 0, 0, time.UTC)

func fixture(t *testing.T) (artifact.SourceConfig, map[string]any) {
	t.Helper()
	b, err := os.ReadFile("testdata/hi-dive.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := artifact.DecodeConfigYAML(b)
	if err != nil {
		t.Fatal(err)
	}
	row := map[string]any{"id": 9657, "title": "Test &amp; Friends", "day": "20260912", "startTime": "8pm", "doors": "Doors: 8pm", "maxPages": 1, "url": "https://hi-dive.com/listing/test/", "permalink": "https://hi-dive.com/listing/test/", "venue": nil, "description": "<p>Presented by Hi-Dive. This is a 21+ event</p>", "ticket": map[string]any{"link": "https://link.dice.fm/example"}}
	page := []any{row}
	return cfg, map[string]any{"page_size": 5, "pages": []any{page}, "terminal": []any{}, "first_check": page}
}
func decode(t *testing.T, c artifact.SourceConfig, s map[string]any) (artifact.Refresh, error) {
	t.Helper()
	b, _ := json.Marshal(s)
	return Decode(c, b, now)
}
func TestMapping(t *testing.T) {
	c, s := fixture(t)
	r, err := decode(t, c, s)
	if err != nil {
		t.Fatal(err)
	}
	out, err := (&artifact.Reconciler{}).Reconcile(c, nil, r, "2026-09-11")
	if err != nil || len(out.Rejected) != 0 || len(out.Artifact.Events) != 1 {
		t.Fatalf("%+v %v", out, err)
	}
	e := out.Artifact.Events[0]
	if e.Title != "Test & Friends" || e.DoorsAt != "2026-09-12T20:00:00-06:00" || e.ShowAt != "" || e.UpstreamID != "9657" || e.Price != nil || e.AdmissionPolicy.Category != "21+" || e.AdmissionPolicy.WithAdult.Ranges[0].MaxAge != 17 {
		t.Fatalf("%+v", e)
	}
}
func TestEnvelopeFailures(t *testing.T) {
	for _, key := range []string{"pages", "terminal", "first_check"} {
		t.Run(key, func(t *testing.T) {
			c, s := fixture(t)
			s[key] = nil
			if _, err := decode(t, c, s); err == nil {
				t.Fatal("accepted incomplete envelope")
			}
		})
	}
}
func TestCoverageFailures(t *testing.T) {
	for _, kind := range []string{"changed", "duplicate", "missingpage", "terminal"} {
		t.Run(kind, func(t *testing.T) {
			c, s := fixture(t)
			row := s["pages"].([]any)[0].([]any)[0].(map[string]any)
			switch kind {
			case "changed":
				s["first_check"] = []any{}
			case "duplicate":
				s["pages"] = []any{[]any{row, row}}
			case "missingpage":
				row["maxPages"] = 2
			case "terminal":
				s["terminal"] = []any{row}
			}
			if _, err := decode(t, c, s); err == nil {
				t.Fatal("accepted incomplete coverage")
			}
		})
	}
}
func TestRecordFailuresAndUnknown(t *testing.T) {
	for _, kind := range []string{"badtime", "offsite", "badurl", "unknown", "winter"} {
		t.Run(kind, func(t *testing.T) {
			c, s := fixture(t)
			row := s["pages"].([]any)[0].([]any)[0].(map[string]any)
			switch kind {
			case "badtime":
				row["doors"] = "Doors: nonsense"
			case "offsite":
				row["venue"] = "Elsewhere"
			case "badurl":
				row["url"] = "https://elsewhere.example/test"
			case "unknown":
				delete(row, "description")
			case "winter":
				row["day"] = "20261112"
			}
			r, err := decode(t, c, s)
			if err != nil {
				t.Fatal(err)
			}
			out, err := (&artifact.Reconciler{}).Reconcile(c, nil, r, "2026-09-11")
			if err != nil {
				t.Fatal(err)
			}
			if kind == "unknown" || kind == "winter" {
				if len(out.Artifact.Events) != 1 {
					t.Fatal(out)
				}
				e := out.Artifact.Events[0]
				policy := artifact.EffectivePolicy(out.Artifact.Venue, e)
				if policy == nil || policy.WithAdult == nil {
					t.Fatal("missing effective venue policy")
				}
				if kind == "unknown" && policy.Category != "" {
					t.Fatal("guessed restriction")
				}
				if kind == "winter" && e.DoorsAt != "2026-11-12T20:00:00-07:00" {
					t.Fatal(out)
				}
			} else if len(out.Rejected) != 1 {
				t.Fatal(out)
			}
		})
	}
}

func TestAdmissionOverrideAndCategories(t *testing.T) {
	for _, category := range []string{"18+", "21+"} {
		for _, override := range []bool{false, true} {
			c, s := fixture(t)
			row := s["pages"].([]any)[0].([]any)[0].(map[string]any)
			row["description"] = "This is a " + category + " event"
			if override {
				c.Overrides = []artifact.Override{{Match: artifact.Match{UpstreamID: "9657", Date: "2026-09-12", VenueKey: "hi-dive"}, Set: map[string]json.RawMessage{"admission_policy": json.RawMessage(`{"text":"No guardian exception","category":"21+"}`)}}}
			}
			r, err := decode(t, c, s)
			if err != nil {
				t.Fatal(err)
			}
			out, err := (&artifact.Reconciler{}).Reconcile(c, nil, r, "2026-09-11")
			if err != nil || len(out.Rejected) > 0 || len(out.Artifact.Events) != 1 {
				t.Fatal(err, out)
			}
			p := artifact.EffectivePolicy(out.Artifact.Venue, out.Artifact.Events[0])
			if override {
				if p.WithAdult != nil || p.Text != "No guardian exception" {
					t.Fatal("override lost", p)
				}
			} else if p.Category != category || p.WithAdult == nil || p.WithAdult.Ranges[0].MinAge != 0 || p.WithAdult.Ranges[0].MaxAge != 17 {
				t.Fatal(p)
			}
		}
	}
}

func TestMalformedRecordsAndWindow(t *testing.T) {
	for _, kind := range []string{"date", "title", "ticket", "multi", "before", "after", "cancelled", "empty"} {
		t.Run(kind, func(t *testing.T) {
			c, s := fixture(t)
			row := s["pages"].([]any)[0].([]any)[0].(map[string]any)
			switch kind {
			case "date":
				row["day"] = "20260230"
			case "title":
				row["title"] = ""
			case "ticket":
				row["ticket"] = map[string]any{"link": "javascript:alert(1)"}
			case "multi":
				row["isMultiDay"] = true
			case "before":
				row["day"] = "20260910"
			case "after":
				row["day"] = "20270912"
			case "cancelled":
				row["title"] = "CANCELLED: Test"
			case "empty":
				s["pages"] = []any{[]any{}}
				s["first_check"] = []any{}
			}
			r, err := decode(t, c, s)
			if err != nil {
				t.Fatal(err)
			}
			out, err := (&artifact.Reconciler{}).Reconcile(c, nil, r, "2026-09-11")
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "before", "after", "empty":
				if len(out.Artifact.Events) != 0 || len(out.Rejected) != 0 {
					t.Fatal(out)
				}
			case "cancelled":
				if len(out.Artifact.Events) != 1 || out.Artifact.Events[0].Status != "Cancelled" {
					t.Fatal(out)
				}
			default:
				if len(out.Rejected) != 1 {
					t.Fatal(out)
				}
			}
		})
	}
}
