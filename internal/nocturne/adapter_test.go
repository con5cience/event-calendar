package nocturne

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"fmt"
	ics "github.com/arran4/golang-ical"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSets(t *testing.T) {
	for _, tc := range []struct {
		text string
		want int
	}{
		{"Set 1 - 6:30-7:45pm | Set 2 - 8:30-9:45", 2},
		{"Set One: 6:30 - 7:45pm | Set Two: 9:00 - 10:30pm", 2},
		{"First Seating Set: 6:30pm-7:45pm Second Seating Set: 9:00-10:30", 2},
		{"Special Single Set: 8:00pm - 9:30pm", 1},
		{"Sunday Summer Show: 6:30pm-8:00pm", 1},
		{"Set One: 5:30 - 6:45pm | Set Two: 7:30 - 9:00pm", 2},
		{"Set 1 - 6:30-8:00pm (No Second Set on this Evening)", 1},
		{"Doors at 6pm", 0},
		{"Set 1 - 6:30-7:45pm", 0},
		{"Set 1 - 25:30-7:45pm | Set 2 - 8:30-9:45", 0},
	} {
		t.Run(tc.text, func(t *testing.T) {
			s, err := sets(tc.text)
			if tc.want == 0 {
				if err == nil {
					t.Fatal("ambiguous schedule accepted")
				}
				return
			}
			if err != nil || len(s) != tc.want {
				t.Fatalf("%v %v", s, err)
			}
		})
	}
}

func TestDecode(t *testing.T) {
	cfg := artifact.SourceConfig{SchemaVersion: 1, Source: artifact.Source{ID: "nocturne", Adapter: "nocturne-ical"}, State: "new", Venue: artifact.Venue{Key: "nocturne", Name: "Nocturne", Website: "https://nocturnejazz.com", Timezone: "America/Denver"}}
	now := time.Date(2026, 9, 14, 18, 0, 0, 0, time.UTC)
	for _, kind := range []string{"normal", "empty", "missing-month", "changed", "recurrence", "wrong-zone", "unknown-sets", "closure", "cancelled"} {
		t.Run(kind, func(t *testing.T) {
			p := pass{Policy: policyFixture, Months: map[string]string{}}
			event := "BEGIN:VEVENT\r\nUID:series-id\r\nDTSTART;TZID=America/Denver:20260918T180000\r\nSUMMARY:Fixture Quartet\r\nURL:https://nocturnejazz.com/music/16-uncategorised/123-fixture\r\nDESCRIPTION:<p>Set One: 6:30 - 7:45pm | Set Two: 9:00 - 10:30pm</p>\r\nEND:VEVENT\r\n"
			switch kind {
			case "empty":
				event = ""
			case "closure":
				event = strings.ReplaceAll(event, "Fixture Quartet", "Holiday Closure")
			case "recurrence":
				event = strings.ReplaceAll(event, "UID:", "RRULE:FREQ=WEEKLY\r\nUID:")
			case "wrong-zone":
				event = strings.ReplaceAll(event, "TZID=America/Denver", "TZID=UTC")
			case "unknown-sets":
				event = strings.ReplaceAll(event, "Set One:", "Something:")
			case "cancelled":
				event = strings.ReplaceAll(event, "UID:", "STATUS:CANCELLED\r\nUID:")
			}
			for i := 0; i < 13; i++ {
				m := time.Date(2026, time.Month(9+i), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
				e := ""
				if i == 0 {
					e = event
				}
				p.Months[m] = fmt.Sprintf("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nX-WR-TIMEZONE:America/Denver\r\n%sEND:VCALENDAR\r\n", e)
			}
			if kind == "missing-month" {
				delete(p.Months, "2027-01-01")
			}
			check := p
			if kind == "changed" {
				check.Policy = "Unreviewed policy"
			}
			raw, _ := json.Marshal(snapshot{From: "2026-09-14", Through: "2027-09-14", Pages: p, Check: check})
			r, err := Decode(cfg, raw, now)
			if kind == "missing-month" || kind == "changed" || kind == "recurrence" || kind == "wrong-zone" {
				if err == nil {
					t.Fatal("invalid accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if kind == "empty" || kind == "closure" {
				if len(r.Observations) != 0 {
					t.Fatal(r)
				}
				return
			}
			if len(r.Observations) != 2 {
				t.Fatal(r)
			}
			if kind == "normal" {
				checkReconciliation(t, cfg, r)
			}
			for _, o := range r.Observations {
				if kind == "unknown-sets" {
					if o.Failure == "" {
						t.Fatal("missing rejection")
					}
					continue
				}
				var e artifact.EventData
				json.Unmarshal(o.Data, &e)
				if e.DoorsAt != "" || e.ShowAt == "" || e.Price != nil {
					t.Fatal(e)
				}
				if kind == "cancelled" {
					if e.Status != "cancelled" || e.AdmissionPolicy.WithAdult != nil {
						t.Fatal(e)
					}
				} else if e.AdmissionPolicy.WithAdult.Ranges[0].MinAge != 10 {
					t.Fatal(e)
				}
			}
		})
	}
}

const policyFixture = `<p>our space and hospitality is for adults 21 and older. If your child, between the age of 10 and 21, is accompanied by you as parent or guardian and you have table reservation we try to accommodate such a situation. We do not allow access to guests under the age of 10 for any reason.</p>`

func checkReconciliation(t *testing.T, cfg artifact.SourceConfig, r artifact.Refresh) {
	t.Helper()
	first, err := (&artifact.Reconciler{}).Reconcile(cfg, nil, r, "2026-09-14")
	if err != nil || len(first.Artifact.Events) != 2 {
		t.Fatalf("%v %v", first, err)
	}
	cfg.State = "established"
	bad := r
	bad.Observations = append([]artifact.Observation{}, r.Observations...)
	for i := range bad.Observations {
		bad.Observations[i].Failure = "unclear set"
	}
	kept, err := (&artifact.Reconciler{}).Reconcile(cfg, &first.Artifact, bad, "2026-09-14")
	if err != nil || len(kept.Rejected) != 2 || !reflect.DeepEqual(first.Artifact.Events, kept.Artifact.Events) {
		t.Fatalf("retention: %v %v", kept, err)
	}
	cfg.Overrides = []artifact.Override{{Match: artifact.Match{UpstreamID: r.Observations[0].UpstreamID, Date: "2026-09-18", VenueKey: "nocturne"}, Set: map[string]json.RawMessage{"admission_policy": json.RawMessage(`{"text":"No minors","category":"21+"}`)}}}
	overridden, err := (&artifact.Reconciler{}).Reconcile(cfg, &first.Artifact, r, "2026-09-14")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range overridden.Artifact.Events {
		if e.UpstreamID == r.Observations[0].UpstreamID && e.AdmissionPolicy.WithAdult != nil {
			t.Fatal("override lost")
		}
	}
	cfg.Overrides = nil
	r.Observations = nil
	removed, err := (&artifact.Reconciler{}).Reconcile(cfg, &first.Artifact, r, "2026-09-14")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range removed.Artifact.Events {
		if e.Listed {
			t.Fatal("absent set still listed")
		}
	}
}

func TestTicket(t *testing.T) {
	const good = "https://www.exploretock.com/nocturnejazz/experience/123/dinner-and-a-show?date=2026-09-18&size=2"
	for _, tc := range []struct{ html, want string }{
		{`<a href="` + good + `">Dinner and a Show Reservations</a>`, good},
		{`<a href="` + good + `" title="Dinner and a Show Reservation"><img alt="Dinner and a Show Button"></a>`, good},
		{`<a href="javascript:alert(1)">Dinner and a Show Reservations</a>`, ""},
		{`<a href="` + good + `">Reserved Bar Seats</a>`, ""},
		{`<a href="` + strings.ReplaceAll(good, "09-18", "09-19") + `">Dinner and a Show Reservations</a>`, ""},
	} {
		if got := ticket(tc.html, "2026-09-18"); got != tc.want {
			t.Fatalf("%q != %q", got, tc.want)
		}
	}
}

func TestTicketThroughCalendar(t *testing.T) {
	raw := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
DESCRIPTION:<p><a href='https://www.exploretock.com/nocturnejazz/experience/287067/dinner-and-a-show-wed-thurs-sunday?date=2026-09-16&amp\;size=2&amp\;time=20%3A00'><strong>Dinner and a Show Reservations</strong></a></p>
END:VEVENT
END:VCALENDAR`
	cal, err := ics.ParseCalendar(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range cal.Events()[0].Properties {
		if p.IANAToken == "DESCRIPTION" && ticket(unescape(p.Value), "2026-09-16") == "" {
			t.Fatalf("lost ticket: %q", p.Value)
		}
	}
}
