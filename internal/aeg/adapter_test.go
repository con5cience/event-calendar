package aeg

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"os"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T, venue string) (artifact.SourceConfig, []byte) {
	t.Helper()
	b, err := os.ReadFile("testdata/" + venue + ".yaml")
	if err != nil {
		t.Fatal(err)
	}
	c, err := artifact.DecodeConfigYAML(b)
	if err != nil {
		t.Fatal(err)
	}
	b, err = os.ReadFile("testdata/" + venue + ".json")
	if err != nil {
		t.Fatal(err)
	}
	return c, b
}

var clock = time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)

func changed(t *testing.T, b []byte, edit func(map[string]any)) []byte {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	edit(v)
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func first(v map[string]any) map[string]any { return v["events"].([]any)[0].(map[string]any) }
func TestPilotMapping(t *testing.T) {
	for _, venue := range []string{"gothic", "mission", "bluebird", "ogden", "fiddlers-green"} {
		t.Run(venue, func(t *testing.T) {
			c, b := fixture(t, venue)
			r, err := Decode(c, b, clock)
			if err != nil {
				t.Fatal(err)
			}
			if r.Coverage.From != "2026-09-09" || r.Coverage.Through != "2027-09-09" || !r.Coverage.Complete {
				t.Fatal(r.Coverage)
			}
			out, err := (&artifact.Reconciler{}).Reconcile(c, nil, r, r.Coverage.From)
			if err != nil {
				t.Fatal(err)
			}
			if len(out.Rejected) != 0 || len(out.Artifact.Events) != 1 {
				t.Fatal(out)
			}
			e := out.Artifact.Events[0]
			if e.Title != "Fixture Ensemble" || e.DoorsAt != "2026-09-12T19:00:00-06:00" || e.ShowAt != "2026-09-12T20:00:00-06:00" || e.Price != nil || e.AdmissionPolicy.Category != "16+" {
				t.Fatal(e)
			}
			if !strings.HasPrefix(e.EventURL, c.Venue.Website+"/events/detail?event_id=") || e.TicketURL != "https://example.com/tickets/1001" {
				t.Fatal(e)
			}
			var raw map[string]any
			encoded, _ := json.Marshal(e)
			json.Unmarshal(encoded, &raw)
			if raw["status"] != "Cancelled" {
				t.Fatal("status lost", raw)
			}
			wrongVenue := changed(t, b, func(v map[string]any) {
				first(v)["venue"].(map[string]any)["venueId"] = "999999"
			})
			if _, err := Decode(c, wrongVenue, clock); err == nil {
				t.Fatal("accepted another venue's events")
			}
		})
	}
}
func TestUnsafeSnapshotsFail(t *testing.T) {
	c, b := fixture(t, "gothic")
	edits := map[string]func(map[string]any){
		"cap":            func(v map[string]any) { v["meta"].(map[string]any)["rows"] = 1 },
		"total mismatch": func(v map[string]any) { v["meta"].(map[string]any)["total"] = 2 },
		"page two":       func(v map[string]any) { v["meta"].(map[string]any)["page"] = 2 },
		"missing meta":   func(v map[string]any) { delete(v, "meta") },
		"null events":    func(v map[string]any) { v["events"] = nil },
		"wrong venue":    func(v map[string]any) { first(v)["venue"].(map[string]any)["venueId"] = "other" },
		"missing ID":     func(v map[string]any) { delete(first(v), "eventId") },
		"duplicate ID": func(v map[string]any) {
			v["events"] = append(v["events"].([]any), first(v))
			v["meta"].(map[string]any)["total"] = 2
		},
		"unknown publication": func(v map[string]any) { delete(first(v), "active") },
		"additional dates":    func(v map[string]any) { first(v)["additionalDates"] = []any{"2026-10-01"} },
	}
	for name, edit := range edits {
		t.Run(name, func(t *testing.T) {
			if _, err := Decode(c, changed(t, b, edit), clock); err == nil {
				t.Fatal("accepted unsafe snapshot")
			}
		})
	}
	for _, bad := range [][]byte{[]byte(`{}`), []byte(`null`), []byte(`{"meta":{"total":0,"page":1,"rows":100},"events":[],"events":[]}`), append(b, []byte(` {}`)...), []byte(strings.Repeat(" ", artifact.MaxDocumentBytes+1))} {
		if _, err := Decode(c, bad, clock); err == nil {
			t.Fatal("accepted malformed snapshot")
		}
	}
}
func TestReconciliationOfInvalidAndMissing(t *testing.T) {
	c, b := fixture(t, "gothic")
	r, err := Decode(c, b, clock)
	if err != nil {
		t.Fatal(err)
	}
	engine := &artifact.Reconciler{}
	old, err := engine.Reconcile(c, nil, r, r.Coverage.From)
	if err != nil {
		t.Fatal(err)
	}
	c.State = "established"
	for _, edit := range []func(map[string]any){
		func(v map[string]any) {
			first(v)["title"].(map[string]any)["eventTitleText"] = ""
			first(v)["title"].(map[string]any)["headlinersText"] = ""
		},
		func(v map[string]any) { first(v)["doorDateTimeUTC"] = "2026-09-13T03:00:00" },
		func(v map[string]any) { first(v)["eventDateTimeISO"] = "garbage" },
		func(v map[string]any) { first(v)["ticketing"].(map[string]any)["eventUrl"] = "javascript:alert(1)" },
	} {
		r, err = Decode(c, changed(t, b, edit), clock)
		if err != nil {
			t.Fatal(err)
		}
		out, err := engine.Reconcile(c, &old.Artifact, r, r.Coverage.From)
		if err != nil {
			t.Fatal(err)
		}
		if len(out.Rejected) != 1 || out.Artifact.Events[0].PublicPath != old.Artifact.Events[0].PublicPath || out.Artifact.Events[0].Title != "Fixture Ensemble" || !out.Artifact.Events[0].Listed {
			t.Fatal(out)
		}
	}
	empty := changed(t, b, func(v map[string]any) { v["events"] = []any{}; v["meta"].(map[string]any)["total"] = 0 })
	r, err = Decode(c, empty, clock)
	if err != nil {
		t.Fatal(err)
	}
	out, err := engine.Reconcile(c, &old.Artifact, r, r.Coverage.From)
	if err != nil {
		t.Fatal(err)
	}
	if out.Artifact.Events[0].Listed {
		t.Fatal("absence not applied")
	}
}
func TestWindowAndSparseFields(t *testing.T) {
	c, b := fixture(t, "gothic")
	for _, date := range []string{"2026-09-08", "2027-09-10"} {
		v := changed(t, b, func(v map[string]any) {
			e := first(v)
			e["eventDateTimeISO"] = date + "T20:00:00-06:00"
			delete(e, "eventDateTime")
			delete(e, "eventDateTimeUTC")
			delete(e, "doorDateTime")
			delete(e, "doorDateTimeUTC")
		})
		r, err := Decode(c, v, clock)
		if err != nil || len(r.Observations) != 0 {
			t.Fatal(r, err)
		}
	}
	v := changed(t, b, func(v map[string]any) {
		e := first(v)
		delete(e, "doorDateTime")
		delete(e, "doorDateTimeUTC")
		e["age"] = "Special admission policy"
		e["ticketPrice"] = "$25 advance"
	})
	r, err := Decode(c, v, clock)
	if err != nil {
		t.Fatal(err)
	}
	out, err := (&artifact.Reconciler{}).Reconcile(c, nil, r, r.Coverage.From)
	if err != nil {
		t.Fatal(err)
	}
	e := out.Artifact.Events[0]
	if e.DoorsAt != "" || e.Price != nil || e.AdmissionPolicy.Category != "" {
		t.Fatal(e)
	}
}
