package kse

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"os"
	"testing"
	"time"
)

func fixture(t *testing.T) (artifact.SourceConfig, map[string]any, map[string]any) {
	t.Helper()
	b, err := os.ReadFile("testdata/paramount.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := artifact.DecodeConfigYAML(b)
	if err != nil {
		t.Fatal(err)
	}
	row := map[string]any{"id": "test123", "type": "event", "name": "Test &amp; Friends", "url": "https://www.ticketmaster.com/event/1E00643B96BD5EEF", "dates": map[string]any{"timezone": "America/Denver", "start": map[string]any{"localDate": "2027-01-12T00:00:00+00:00", "localTime": "2026-09-11T20:00:00+00:00", "dateTime": "2027-01-13T03:00:00+00:00"}, "status": map[string]any{"code": "onsale"}}, "doorsTimes": map[string]any{"dateTime": "2027-01-13T02:00:00+00:00"}, "calendar_start_datetime": "2027-01-12T20:00:00", "ageRestrictions": map[string]any{"legalAgeEnforced": false}, "_embedded": map[string]any{"venues": []any{map[string]any{"id": "KovZpZAFa1nA", "name": "Paramount Theatre"}}}}
	rows := []any{row}
	return cfg, map[string]any{"events": rows, "check": rows}, row
}
func decode(t *testing.T, c artifact.SourceConfig, s map[string]any) (artifact.Refresh, error) {
	t.Helper()
	b, _ := json.Marshal(s)
	return Decode(c, b, time.Date(2026, 9, 11, 18, 0, 0, 0, time.UTC))
}
func TestMapping(t *testing.T) {
	c, s, _ := fixture(t)
	r, err := decode(t, c, s)
	if err != nil {
		t.Fatal(err)
	}
	out, err := (&artifact.Reconciler{}).Reconcile(c, nil, r, "2026-09-11")
	if err != nil || len(out.Rejected) > 0 || len(out.Artifact.Events) != 1 {
		t.Fatal(err, out)
	}
	e := out.Artifact.Events[0]
	if e.Title != "Test & Friends" || e.DoorsAt != "2027-01-12T19:00:00-07:00" || e.ShowAt != "2027-01-12T20:00:00-07:00" || e.Date != "2027-01-12" || e.Price != nil {
		t.Fatal(e)
	}
	p := artifact.EffectivePolicy(out.Artifact.Venue, e)
	if p.Category != "" || p.WithAdult != nil {
		t.Fatal("false legalAgeEnforced is not all ages", p)
	}
}

func TestConflictingAccompanimentAndReviewDate(t *testing.T) {
	if _, err := admission("Under 14 must be accompanied by a person aged 18+. Ages 18+ only", "https://www.ticketmaster.com/event/test", "2026-09-11"); err == nil {
		t.Fatal("contradictory restriction accepted")
	}
	c, s, row := fixture(t)
	row["pleaseNote"] = "This event is 12+"
	b, _ := json.Marshal(s)
	r, err := Decode(c, b, time.Date(2026, 10, 1, 18, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var d artifact.EventData
	if err = json.Unmarshal(r.Observations[0].Data, &d); err != nil {
		t.Fatal(err)
	}
	if d.AdmissionPolicy.WithAdult.ReviewedOn != "2026-09-11" {
		t.Fatal("capture is not a new policy review")
	}
}
func TestAdmission(t *testing.T) {
	for _, sample := range []struct {
		text, category string
		min, max       int
	}{
		{"This event is 12+", "", 12, 17}, {"Ages 15+", "", 15, 17}, {"Age Limit - 16+", "16+", 16, 17}, {"Ages 18+ show", "18+", -1, -1}, {"Recommended for 13+", "", -1, -1}, {"RECOMMENDED AGE 18+", "", -1, -1}, {"Under 14 must be accompanied by a person aged 18+", "", 0, 13},
	} {
		t.Run(sample.text, func(t *testing.T) {
			c, s, row := fixture(t)
			row["pleaseNote"] = sample.text
			r, err := decode(t, c, s)
			if err != nil {
				t.Fatal(err)
			}
			out, err := (&artifact.Reconciler{}).Reconcile(c, nil, r, "2026-09-11")
			if err != nil || len(out.Artifact.Events) != 1 {
				t.Fatal(err, out)
			}
			p := artifact.EffectivePolicy(out.Artifact.Venue, out.Artifact.Events[0])
			if p.Category != sample.category {
				t.Fatal(p)
			}
			if sample.min < 0 {
				if p.WithAdult != nil {
					t.Fatal("invented exception", p)
				}
			} else if p.WithAdult == nil || p.WithAdult.Ranges[0].MinAge != sample.min || p.WithAdult.Ranges[0].MaxAge != sample.max {
				t.Fatal(p)
			}
		})
	}
}
func TestCoverage(t *testing.T) {
	for _, kind := range []string{"missing", "changed", "duplicate", "badid"} {
		t.Run(kind, func(t *testing.T) {
			c, s, row := fixture(t)
			switch kind {
			case "missing":
				delete(s, "check")
			case "changed":
				s["check"] = []any{}
			case "duplicate":
				s["events"] = []any{row, row}
				s["check"] = s["events"]
			case "badid":
				row["id"] = ""
			}
			if _, err := decode(t, c, s); err == nil {
				t.Fatal("accepted incomplete capture")
			}
		})
	}
}
func TestRejectedRecords(t *testing.T) {
	for _, kind := range []string{"venue", "utc", "date", "doors", "tbd", "title", "url"} {
		t.Run(kind, func(t *testing.T) {
			c, s, row := fixture(t)
			dates := row["dates"].(map[string]any)
			start := dates["start"].(map[string]any)
			switch kind {
			case "venue":
				row["_embedded"] = map[string]any{"venues": []any{map[string]any{"id": "elsewhere"}}}
			case "utc":
				start["dateTime"] = "bad"
			case "date":
				start["localDate"] = "2027-01-13T00:00:00Z"
			case "doors":
				row["doorsTimes"] = map[string]any{"dateTime": "bad"}
			case "tbd":
				start["dateTBD"] = true
			case "title":
				row["name"] = ""
			case "url":
				row["url"] = "javascript:alert(1)"
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
