package hmt

import (
	"bytes"
	"encoding/json"
	"event-calendar/internal/artifact"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) (artifact.SourceConfig, []byte) {
	t.Helper()
	c := artifact.SourceConfig{SchemaVersion: 1, Source: artifact.Source{ID: "hq", Adapter: "holdmyticket-ical"}, State: "new", Venue: artifact.Venue{Key: "hq", Name: "HQ", Website: "https://www.hqdenver.com", Timezone: "America/Denver"}, AdapterOptions: map[string]string{"feed_id": "6457"}}
	calendar := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nX-WR-TIMEZONE:America/Denver\r\nX-WR-CALNAME:HQ\r\nBEGIN:VEVENT\r\nUID:fixture-id\r\nDTSTART:20260912T200000\r\nSUMMARY:Fixture\\, Ensemble\r\nURL;VALUE=URI:http://holdmyticket.com/event/1001 \r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	raw := map[string]any{"calendar": calendar, "details": map[string]any{"1001": map[string]any{"@type": "Event", "name": "Fixture, Ensemble", "startDate": "2026-09-12T20:00:00-06:00", "doorTime": "19:00", "typicalAgeRange": "18+ Ages", "location": map[string]any{"name": "HQ"}, "eventStatus": "https://schema.org/EventScheduled", "offers": []any{map[string]any{"name": "General Admission", "price": "20.55", "priceCurrency": "USD", "url": "https://tickets.holdmyticket.com/tickets/1001"}}}}}
	b, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	return c, b
}

var clock = time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)

func TestAgeLabels(t *testing.T) {
	for _, label := range []string{"13+ / Bar with ID", "16+ / Bar with ID", "18+ / Bar with ID", "21+ / Bar with ID", "16+ (no exceptions)"} {
		t.Run(label, func(t *testing.T) {
			c, b := fixture(t)
			b = bytes.ReplaceAll(b, []byte("18+ Ages"), []byte(label))
			r, err := Decode(c, b, clock)
			if err != nil {
				t.Fatal(err)
			}
			var e artifact.EventData
			json.Unmarshal(r.Observations[0].Data, &e)
			want := strings.Split(label, " ")[0]
			if strings.Contains(label, "exceptions") {
				want = ""
			}
			if e.AdmissionPolicy.Category != want || e.AdmissionPolicy.Text != label || e.AdmissionPolicy.WithAdult != nil {
				t.Fatalf("%+v", e.AdmissionPolicy)
			}
		})
	}
}

func TestFoldedCalendarAndOffers(t *testing.T) {
	c, b := fixture(t)
	var raw map[string]any
	json.Unmarshal(b, &raw)
	cal := raw["calendar"].(string)
	cal = strings.Replace(cal, "Fixture\\, Ensemble", "Fixture\\, En\r\n semble", 1)
	cal = strings.Replace(cal, "DTSTART:", "STATUS:CANCELLED\r\nDTSTART;TZID=America/Denver:", 1)
	raw["calendar"] = cal
	d := raw["details"].(map[string]any)["1001"].(map[string]any)
	d["offers"] = append(d["offers"].([]any), map[string]any{"name": "Table for four", "price": "200.00", "priceCurrency": "USD", "availability": "https://schema.org/SoldOut"})
	b, _ = json.Marshal(raw)
	r, err := Decode(c, b, clock)
	if err != nil || r.Observations[0].Failure != "" {
		t.Fatalf("%v %+v", err, r)
	}
	var e artifact.EventData
	json.Unmarshal(r.Observations[0].Data, &e)
	if e.Title != "Fixture, Ensemble" || e.Status != "Cancelled" || e.Price != nil {
		t.Fatalf("%+v", e)
	}
}

func TestMapping(t *testing.T) {
	c, b := fixture(t)
	r, err := Decode(c, b, clock)
	if err != nil {
		t.Fatal(err)
	}
	out, err := (&artifact.Reconciler{}).Reconcile(c, nil, r, r.Coverage.From)
	if err != nil || len(out.Rejected) != 0 || len(out.Artifact.Events) != 1 {
		t.Fatalf("%v %+v", err, out)
	}
	e := out.Artifact.Events[0]
	if e.UpstreamID != "1001" || e.Title != "Fixture, Ensemble" || e.DoorsAt != "2026-09-12T19:00:00-06:00" || e.ShowAt != "2026-09-12T20:00:00-06:00" || e.AdmissionPolicy.Category != "18+" || e.AdmissionPolicy.WithAdult != nil || e.Price != nil || e.TicketURL != "https://tickets.holdmyticket.com/tickets/1001" {
		t.Fatalf("%+v", e)
	}
}
func TestUnsafeSnapshots(t *testing.T) {
	for _, variant := range []string{"truncated", "duplicate", "recurrence", "wrong venue", "missing detail", "wrong clock", "unsafe ticket", "wrong timezone"} {
		t.Run(variant, func(t *testing.T) {
			c, b := fixture(t)
			var raw map[string]any
			json.Unmarshal(b, &raw)
			cal := raw["calendar"].(string)
			d := raw["details"].(map[string]any)["1001"].(map[string]any)
			switch variant {
			case "truncated":
				raw["calendar"] = strings.ReplaceAll(cal, "END:VCALENDAR", "")
			case "duplicate":
				raw["calendar"] = strings.Replace(cal, "END:VCALENDAR", cal[strings.Index(cal, "BEGIN:VEVENT"):strings.Index(cal, "END:VCALENDAR")]+"END:VCALENDAR", 1)
			case "recurrence":
				raw["calendar"] = strings.Replace(cal, "SUMMARY:", "RRULE:FREQ=DAILY\r\nSUMMARY:", 1)
			case "wrong venue":
				d["location"] = map[string]any{"name": "Other venue"}
			case "missing detail":
				raw["details"] = map[string]any{}
			case "wrong clock":
				d["startDate"] = "2026-09-12T21:00:00-06:00"
			case "unsafe ticket":
				d["offers"].([]any)[0].(map[string]any)["url"] = "https://example.com/tickets/1001"
			case "wrong timezone":
				raw["calendar"] = strings.ReplaceAll(cal, "America/Denver", "America/New_York")
			}
			b, _ = json.Marshal(raw)
			r, err := Decode(c, b, clock)
			if err == nil {
				out, re := (&artifact.Reconciler{}).Reconcile(c, nil, r, r.Coverage.From)
				if re == nil && len(out.Rejected) == 0 {
					t.Fatalf("accepted %s", variant)
				}
			}
		})
	}
}
func TestTimeVariants(t *testing.T) {
	for _, tc := range []struct {
		start, detail, doors string
		valid                bool
	}{
		{"20260913T020000Z", "2026-09-12T20:00:00-06:00", "19:00", true},
		{"20261101T013000", "2026-11-01T01:30:00-06:00", "", false},
		{"20270314T023000", "2027-03-14T03:30:00-06:00", "", false},
		{"20260912", "2026-09-12", "", true},
	} {
		t.Run(tc.start, func(t *testing.T) {
			c, b := fixture(t)
			var raw map[string]any
			json.Unmarshal(b, &raw)
			raw["calendar"] = strings.ReplaceAll(raw["calendar"].(string), "20260912T200000", tc.start)
			if len(tc.start) == 8 {
				raw["calendar"] = strings.ReplaceAll(raw["calendar"].(string), "DTSTART:", "DTSTART;VALUE=DATE:")
			}
			d := raw["details"].(map[string]any)["1001"].(map[string]any)
			d["startDate"] = tc.detail
			d["doorTime"] = tc.doors
			b, _ = json.Marshal(raw)
			r, err := Decode(c, b, clock)
			valid := err == nil && len(r.Observations) == 1 && r.Observations[0].Failure == ""
			if valid != tc.valid {
				t.Fatalf("valid=%v err=%v %+v", valid, err, r)
			}
		})
	}
}
