package rhp

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"strings"
	"testing"
	"time"
)

var clock = time.Date(2026, 9, 10, 18, 0, 0, 0, time.UTC)

func fixture(t *testing.T, key string) (artifact.SourceConfig, map[string]any) {
	t.Helper()
	names := map[string]string{"lost-lake": "Lost Lake", "larimer": "Larimer Lounge", "globe-hall": "Globe Hall"}
	origins := map[string]string{"lost-lake": "https://lost-lake.com", "larimer": "https://larimerlounge.com", "globe-hall": "https://globehall.com"}
	origin := origins[key]
	cfg := artifact.SourceConfig{SchemaVersion: 1, Source: artifact.Source{ID: key, Adapter: "rhp-calendar"}, State: "new", Venue: artifact.Venue{Key: key, Name: names[key], Website: origin, Timezone: "America/Denver"}}
	u := origin + "/event/fixture/room/denver-colorado/"
	page := `<html><head><script type="application/ld+json">{"@type":"Event","name":"Fixture & Friends","url":"` + u + `","startDate":"2026-09-12T20:00:00-0600","location":{"name":"` + names[key] + `","address":"123 Test Street"},"offers":{"price":20,"url":"https://www.etix.com/ticket/p/123/fixture"}}</script></head><body><h1>Fixture &amp; Friends</h1><div class="eventAgeRestriction">Ages 16 and up</div><div class="eventDoorStartDate">Doors: 7 pm Show: 8 pm</div><div class="eventDescription"><ul><li>All ages, ticketed guests under 16 ONLY ADMITTED WITH TICKETED GUARDIAN 21+</li></ul></div></body></html>`
	row := map[string]any{"plaintitle": "Fixture &#038; Friends", "url": u, "start": "2026-09-12T20:00:00", "end": "2026-09-12T23:59:59", "venuename": "", "strctaHtml": `<a href="https://www.etix.com/ticket/p/123/fixture">Buy Tickets</a>`}
	return cfg, map[string]any{"calendar": map[string]any{"success": true, "data": map[string]any{"events": []any{row}}}, "details": map[string]any{u: page}}
}

func decodeFixture(t *testing.T, c artifact.SourceConfig, s map[string]any) artifact.Refresh {
	t.Helper()
	b, _ := json.Marshal(s)
	r, err := Decode(c, b, clock)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func TestMapping(t *testing.T) {
	for _, key := range []string{"lost-lake", "larimer", "globe-hall"} {
		t.Run(key, func(t *testing.T) {
			c, s := fixture(t, key)
			r := decodeFixture(t, c, s)
			result, err := (&artifact.Reconciler{}).Reconcile(c, nil, r, r.Coverage.From)
			if err != nil || len(result.Rejected) != 0 || len(result.Artifact.Events) != 1 {
				t.Fatalf("%v %+v", err, result)
			}
			e := result.Artifact.Events[0]
			if e.Title != "Fixture & Friends" || e.DoorsAt != "2026-09-12T19:00:00-06:00" || e.ShowAt != "2026-09-12T20:00:00-06:00" || e.Price != nil || e.TicketURL != "https://www.etix.com/ticket/p/123/fixture" || e.OffSite {
				t.Fatalf("%+v", e)
			}
			p := e.AdmissionPolicy
			if p == nil || p.Category != "16+" || p.WithAdult == nil || p.WithAdult.URL != e.EventURL || p.WithAdult.Ranges[0].MaxAge != 15 || !strings.Contains(p.WithAdult.Ranges[0].Condition, "21+") {
				t.Fatalf("%+v", p)
			}
		})
	}
}

func TestInvalidInputs(t *testing.T) {
	for _, variant := range []string{"failed envelope", "missing events", "null events", "duplicate", "unsafe URL", "missing detail", "mismatched title", "mismatched start", "unsafe ticket", "DST gap", "DST overlap"} {
		t.Run(variant, func(t *testing.T) {
			c, s := fixture(t, "lost-lake")
			cal := s["calendar"].(map[string]any)
			data := cal["data"].(map[string]any)
			rows := data["events"].([]any)
			row := rows[0].(map[string]any)
			u := row["url"].(string)
			details := s["details"].(map[string]any)
			h := details[u].(string)
			switch variant {
			case "failed envelope":
				cal["success"] = false
			case "missing events":
				delete(data, "events")
			case "null events":
				data["events"] = nil
			case "duplicate":
				data["events"] = append(rows, row)
			case "unsafe URL":
				row["url"] = "https://evil.example/event/fixture/"
			case "missing detail":
				delete(details, u)
			case "mismatched title":
				row["plaintitle"] = "Different show"
			case "mismatched start":
				row["start"] = "2026-09-13T20:00:00"
			case "unsafe ticket":
				details[u] = strings.ReplaceAll(h, "https://www.etix.com/ticket/p/123/fixture", "javascript:alert(1)")
			case "DST gap":
				row["start"] = "2027-03-14T02:30:00"
			case "DST overlap":
				row["start"] = "2026-11-01T01:30:00"
			}
			b, _ := json.Marshal(s)
			r, err := Decode(c, b, clock)
			if err == nil {
				result, e := (&artifact.Reconciler{}).Reconcile(c, nil, r, r.Coverage.From)
				if e == nil && len(result.Rejected) == 0 {
					t.Fatal("unsafe input accepted")
				}
			}
		})
	}
}

func TestAdmissionAndOffsite(t *testing.T) {
	for _, v := range []string{"18+", "no exception", "offsite", "missing label", "override"} {
		t.Run(v, func(t *testing.T) {
			c, s := fixture(t, "larimer")
			ds := s["details"].(map[string]any)
			var u, h string
			for k, v := range ds {
				u = k
				h = v.(string)
			}
			switch v {
			case "18+":
				h = strings.ReplaceAll(h, "Ages 16 and up", "Ages 18 and up")
			case "no exception":
				h = strings.ReplaceAll(h, "All ages, ticketed guests under 16 ONLY ADMITTED WITH TICKETED GUARDIAN 21+", "No exceptions")
			case "offsite":
				h = strings.ReplaceAll(h, "Larimer Lounge", "Campground")
			case "missing label":
				h = strings.ReplaceAll(h, "Ages 16 and up", "")
			case "override":
				c.Overrides = []artifact.Override{{Match: artifact.Match{UpstreamID: u, Date: "2026-09-12", VenueKey: "larimer"}, Set: map[string]json.RawMessage{"admission_policy": json.RawMessage(`{"text":"21+ only","category":"21+"}`)}}}
			}
			ds[u] = h
			r := decodeFixture(t, c, s)
			result, err := (&artifact.Reconciler{}).Reconcile(c, nil, r, r.Coverage.From)
			if err != nil || len(result.Rejected) != 0 {
				t.Fatalf("%v %+v", err, result)
			}
			e := result.Artifact.Events[0]
			if v == "offsite" {
				if !e.OffSite || e.Venue.Name != "Campground" {
					t.Fatalf("%+v", e)
				}
			}
			if v == "missing label" {
				if e.AdmissionPolicy == nil || e.AdmissionPolicy.Category != "16+" || e.AdmissionPolicy.WithAdult == nil {
					t.Fatalf("%+v", e)
				}
			} else if e.AdmissionPolicy != nil && e.AdmissionPolicy.WithAdult != nil {
				t.Fatalf("unexpected clearance: %+v", e)
			}
		})
	}
}

func TestAllAgesAndCancellation(t *testing.T) {
	c, s := fixture(t, "globe-hall")
	ds := s["details"].(map[string]any)
	for u, v := range ds {
		ds[u] = strings.ReplaceAll(strings.ReplaceAll(v.(string), "Ages 16 and up", "All Ages"), guardianText, "")
	}
	row := s["calendar"].(map[string]any)["data"].(map[string]any)["events"].([]any)[0].(map[string]any)
	row["strctaHtml"] = `<span class="rhp-event-cta Canceled">Cancelled</span>`
	r := decodeFixture(t, c, s)
	var e artifact.EventData
	json.Unmarshal(r.Observations[0].Data, &e)
	if e.Status != "Cancelled" || e.AdmissionPolicy.WithAdult == nil || e.AdmissionPolicy.WithAdult.Ranges[0].MaxAge != 17 {
		t.Fatalf("%+v", e)
	}
}

func TestGuardianTextOutsideDescriptionDoesNotClear(t *testing.T) {
	c, s := fixture(t, "lost-lake")
	ds := s["details"].(map[string]any)
	for u, v := range ds {
		ds[u] = strings.ReplaceAll(v.(string), `class="eventDescription"`, `class="unrelated-footer"`)
	}
	r := decodeFixture(t, c, s)
	var e artifact.EventData
	json.Unmarshal(r.Observations[0].Data, &e)
	if e.AdmissionPolicy.WithAdult != nil {
		t.Fatal("unrelated page text cleared admission")
	}
}

func TestCalendarCapAndUnknownProfile(t *testing.T) {
	c, s := fixture(t, "lost-lake")
	data := s["calendar"].(map[string]any)["data"].(map[string]any)
	row := data["events"].([]any)[0]
	rows := make([]any, 10000)
	for i := range rows {
		rows[i] = row
	}
	data["events"] = rows
	b, _ := json.Marshal(s)
	if _, err := Decode(c, b, clock); err == nil {
		t.Fatal("accepted capped response")
	}
	c, s = fixture(t, "lost-lake")
	c.Source.ID = "unreviewed"
	b, _ = json.Marshal(s)
	if _, err := Decode(c, b, clock); err == nil {
		t.Fatal("accepted unreviewed source")
	}
}

func TestSixteenYearOldDoesNotLoseClearance(t *testing.T) {
	c, s := fixture(t, "lost-lake")
	r := decodeFixture(t, c, s)
	var e artifact.EventData
	json.Unmarshal(r.Observations[0].Data, &e)
	ranges := e.AdmissionPolicy.WithAdult.Ranges
	if len(ranges) != 2 || ranges[1].MinAge != 16 || ranges[1].MaxAge != 17 || ranges[1].Condition != "Permitted at this age" {
		t.Fatalf("missing ordinary admission at age 16: %+v", ranges)
	}
}
