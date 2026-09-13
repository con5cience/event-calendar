package livenation

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"os"
	"testing"
	"time"
)

func fixture(t *testing.T) (artifact.SourceConfig, map[string]any, map[string]any) {
	t.Helper()
	b, err := os.ReadFile("testdata/marquis.yaml")
	if err != nil {
		t.Fatal(err)
	}
	c, err := artifact.DecodeConfigYAML(b)
	if err != nil {
		t.Fatal(err)
	}
	row := map[string]any{"tm_id": "1E00646DB543AD47", "name": "Test &amp; Friends", "type": "REGULAR", "event_data_type": "discovery", "url": "https://www.ticketmaster.com/test/event/1E00646DB543AD47", "start_date_local": "2027-01-12", "start_time_local": "19:00:00", "start_datetime_utc": "2027-01-13T02:00:00Z", "timezone": "America/Denver", "status_code": "onsale", "important_info": "Doors: 7PM Show: 8PM This show is ALL AGES", "venue": map[string]any{"discovery_id": "KovZpZAJeFkA"}}
	pages := []any{[]any{row}, []any{}}
	return c, map[string]any{"pages": pages, "check": pages}, row
}
func decode(t *testing.T, c artifact.SourceConfig, s map[string]any) (artifact.Refresh, error) {
	t.Helper()
	b, _ := json.Marshal(s)
	return Decode(c, b, time.Date(2026, 9, 11, 20, 0, 0, 0, time.UTC))
}
func TestMappingAndPolicy(t *testing.T) {
	for _, restriction := range []string{"ALL AGES", "18+", "16+", "unknown"} {
		for _, override := range []bool{false, true} {
			c, s, row := fixture(t)
			row["important_info"] = "Doors: 7PM Show: 8PM This show is " + restriction
			if override {
				c.Overrides = []artifact.Override{{Match: artifact.Match{UpstreamID: "1E00646DB543AD47", Date: "2027-01-12", VenueKey: "marquis"}, Set: map[string]json.RawMessage{"admission_policy": json.RawMessage(`{"text":"No exception","category":"21+"}`)}}}
			}
			r, err := decode(t, c, s)
			if err != nil {
				t.Fatal(err)
			}
			out, err := (&artifact.Reconciler{}).Reconcile(c, nil, r, "2026-09-11")
			if err != nil || len(out.Rejected) > 0 || len(out.Artifact.Events) != 1 {
				t.Fatal(err, out)
			}
			e := out.Artifact.Events[0]
			p := artifact.EffectivePolicy(out.Artifact.Venue, e)
			if e.Title != "Test & Friends" || e.DoorsAt != "2027-01-12T19:00:00-07:00" || e.ShowAt != "2027-01-12T20:00:00-07:00" || e.Price != nil {
				t.Fatal(e)
			}
			if override {
				if p.WithAdult != nil || p.Category != "21+" {
					t.Fatal(p)
				}
				continue
			}
			if restriction == "ALL AGES" {
				if p.WithAdult == nil || p.Category != "All ages" || p.WithAdult.Ranges[1].MinAge != 3 || p.WithAdult.Ranges[1].MaxAge != 17 {
					t.Fatal(p)
				}
			} else if p.WithAdult != nil {
				t.Fatal("invented permission", p)
			}
		}
	}
}
func TestCoverage(t *testing.T) {
	for _, kind := range []string{"terminal", "changed", "duplicate", "identity"} {
		t.Run(kind, func(t *testing.T) {
			c, s, row := fixture(t)
			switch kind {
			case "terminal":
				s["pages"] = []any{[]any{row}}
				s["check"] = s["pages"]
			case "changed":
				s["check"] = []any{[]any{}}
			case "duplicate":
				s["pages"] = []any{[]any{row, row}, []any{}}
				s["check"] = s["pages"]
			case "identity":
				row["tm_id"] = ""
			}
			if _, err := decode(t, c, s); err == nil {
				t.Fatal("accepted incomplete snapshot")
			}
		})
	}
}
func TestInvalidRecords(t *testing.T) {
	for _, kind := range []string{"venue", "date", "utc", "doors", "url", "conflict", "title", "upsell"} {
		t.Run(kind, func(t *testing.T) {
			c, s, row := fixture(t)
			switch kind {
			case "venue":
				row["venue"] = map[string]any{"discovery_id": "elsewhere"}
			case "date":
				row["start_date_local"] = "2027-01-13"
			case "utc":
				row["start_datetime_utc"] = "bad"
			case "doors":
				row["important_info"] = "Doors 9PM Show 8PM ALL AGES"
			case "url":
				row["url"] = "javascript:bad"
			case "conflict":
				row["name"] = "Test - 18+"
			case "title":
				row["name"] = ""
			case "upsell":
				row["type"] = "FastLane Access"
			}
			r, err := decode(t, c, s)
			if err != nil {
				t.Fatal(err)
			}
			out, err := (&artifact.Reconciler{}).Reconcile(c, nil, r, "2026-09-11")
			if err != nil || len(out.Rejected) != 1 {
				t.Fatal(err, out)
			}
		})
	}
}
func TestAbsentPolicyUsesReviewedDefault(t *testing.T) {
	c, s, row := fixture(t)
	row["important_info"] = "Doors 7PM Show 8PM"
	r, err := decode(t, c, s)
	if err != nil {
		t.Fatal(err)
	}
	out, err := (&artifact.Reconciler{}).Reconcile(c, nil, r, "2026-09-11")
	if err != nil {
		t.Fatal(err)
	}
	p := artifact.EffectivePolicy(out.Artifact.Venue, out.Artifact.Events[0])
	if p.Category != "All ages" || p.WithAdult == nil {
		t.Fatal(p)
	}
}
