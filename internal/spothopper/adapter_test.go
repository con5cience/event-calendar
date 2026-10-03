package spothopper

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"strings"
	"testing"
	"time"
)

func config(t *testing.T) artifact.SourceConfig {
	t.Helper()
	b := []byte(`{"schema_version":1,"source":{"id":"black-sky","adapter":"spothopper-events"},"state":"new","venue":{"key":"black-sky","name":"Black Sky Brewery","timezone":"America/Denver","website":"https://blackskydenver.com/denver-santa-fe-arts-district-black-sky-brewery-events","address":"490 Santa Fe Drive, Denver, CO 80204","phone":"(720) 708-5816"},"adapter_options":{"recurring":"exclude"}}`)
	cfg, err := artifact.DecodeConfigJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

type fixture struct {
	id, title, day, window, description string
	recurring                           bool
}

func kalEl() fixture {
	return fixture{
		id: "3040484", title: "Kal-El Astral Voyaging 2026 North American Tour",
		day: "Friday October 9th", window: "07:00 PM - 10:00 PM",
		description: "W/ Guests GOYA &amp; Black Moon Cult $25.00 at the Door All Ages - Doors open at 6 p.m. Tickets Available at: https://tickets.holdmyticket.com/tickets/461713",
	}
}
func halloween() fixture {
	return fixture{
		id: "3315530", title: "Halloween Party",
		day: "Saturday October 31st", window: "06:00 PM - 12:00 AM",
		description: "Grunja - Prophets Tomb - Die Like Bothans Costume Contest / Prizes $5.00 at the door Ages 18 +",
	}
}
func birthday() fixture {
	return fixture{
		id: "3429873", title: "Rhiannon&#39;s 50th Birthday Bash",
		day: "Saturday January 16th", window: "08:00 PM - 12:00 AM",
		description: "KaRhiaoke - Dance Party - Music - Beer- Friends- Hugs",
	}
}
func magic() fixture {
	return fixture{
		id: "3475887", title: "Monday Night Magic",
		day: "Monday October 5th", window: "04:00 PM - 09:00 PM",
		description: "Every Monday", recurring: true,
	}
}

func sectionHTML(f fixture) string {
	recurring := "false"
	if f.recurring {
		recurring = "true"
	}
	return `<section id="` + f.id + `"><div class="row event-content"><div class="col-md-6 col-sm-6 col-xs-12 event-content-item event-text-holder"><h2>` + f.title + `</h2><p class="event-main-text event-day">` + f.day + `</p><div class="event-info-text"><div data-event-id="` + f.id + `" data-is-recurring="` + recurring + `" data-origin-event-id="` + f.id + `" data-spot-promotion-id="" data-tags="" style="display: none"></div><p>` + f.description + `</p></div><p class="event-main-text event-time">` + f.window + `</p></div></div></section>`
}

func listing(fs []fixture) string {
	var b []byte
	for _, f := range fs {
		b = append(b, sectionHTML(f)...)
	}
	return `<html><body><div class="events-holder">` + string(b) + `</div></body></html>`
}

func snapshot(fs, check []fixture) []byte {
	if check == nil {
		check = fs
	}
	raw, _ := json.Marshal(map[string]any{
		"from": "2026-10-03", "through": "2027-10-03",
		"page":  listing(fs),
		"check": listing(check),
	})
	return raw
}

func paired(page string) []byte {
	raw, _ := json.Marshal(map[string]string{
		"from": "2026-10-03", "through": "2027-10-03",
		"page": page, "check": page,
	})
	return raw
}

func TestMapping(t *testing.T) {
	now := time.Date(2026, 10, 3, 18, 0, 0, 0, time.UTC)
	cfg := config(t)
	run, err := Decode(cfg, snapshot([]fixture{magic(), kalEl(), halloween(), birthday()}, nil), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Observations) != 4 {
		t.Fatalf("%+v", run.Observations)
	}
	byID := map[string]artifact.Observation{}
	for _, o := range run.Observations {
		byID[o.UpstreamID] = o
	}
	if byID["3475887"].Failure != "recurring series excluded" {
		t.Fatalf("recurring series not excluded: %+v", byID["3475887"])
	}
	var kal, hall, birth artifact.EventData
	for id, want := range map[string]*artifact.EventData{"3040484": &kal, "3315530": &hall, "3429873": &birth} {
		o := byID[id]
		if o.Failure != "" {
			t.Fatal(id, o.Failure)
		}
		if json.Unmarshal(o.Data, want) != nil {
			t.Fatal("data")
		}
	}
	if kal.Title != "Kal-El Astral Voyaging 2026 North American Tour" ||
		kal.Date != "2026-10-09" ||
		kal.DoorsAt != "2026-10-09T18:00:00-06:00" ||
		kal.ShowAt != "2026-10-09T19:00:00-06:00" ||
		kal.AdmissionPolicy.Category != "All ages" ||
		kal.TicketURL != "https://tickets.holdmyticket.com/tickets/461713" {
		t.Fatalf("%+v", kal)
	}
	if hall.Date != "2026-10-31" || hall.ShowAt != "2026-10-31T18:00:00-06:00" ||
		hall.DoorsAt != "" || hall.AdmissionPolicy.Category != "18+" ||
		hall.AdmissionPolicy.Text != "Ages 18+" {
		t.Fatalf("%+v", hall)
	}
	// January rolls the inferred year over and the printed weekday confirms it.
	if birth.Date != "2027-01-16" || birth.ShowAt != "2027-01-16T20:00:00-07:00" ||
		birth.AdmissionPolicy != nil || birth.TicketURL != "" {
		t.Fatalf("%+v", birth)
	}
	out, err := (&artifact.Reconciler{}).Reconcile(cfg, nil, run, "2026-10-03")
	if err != nil || len(out.Artifact.Events) != 3 || len(out.Rejected) != 1 ||
		out.Rejected[0].Reason != "invalid normalized record: adapter: recurring series excluded" {
		t.Fatalf("%v %+v", err, out)
	}
}

func TestRecordFailures(t *testing.T) {
	now := time.Date(2026, 10, 3, 18, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, want string
		mutate     func(*fixture)
	}{
		{"doors after show", "conflicting doors time", func(f *fixture) {
			f.description = "Doors open at 9 p.m. All Ages"
		}},
		{"show outside window", "conflicting show time", func(f *fixture) {
			f.description = "Show starts at 11 p.m."
		}},
		{"conflicting ages", "conflicting age text", func(f *fixture) {
			f.description = "All Ages Ages 18+ night"
		}},
		{"unreviewed age", "unreviewed age text", func(f *fixture) {
			f.description = "Ages 19+ only"
		}},
		{"conflicting tickets", "conflicting ticket link", func(f *fixture) {
			f.description = "https://holdmyticket.com/tickets/462407 and https://tickets.holdmyticket.com/tickets/461713"
		}},
		{"bad window", "unreviewed event window", func(f *fixture) {
			f.window = "Doors at seven"
		}},
		{"door charge stays unparsed", "", func(f *fixture) {
			f.window = "12:00 PM - 10:00 PM"
			f.description = "Door Charge of $10.00 starts at 5 p.m. Bands start at 6 p.m. Ages 18+"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := kalEl()
			tc.mutate(&f)
			run, err := Decode(config(t), snapshot([]fixture{f}, nil), now)
			if err != nil {
				t.Fatal(err)
			}
			if run.Observations[0].Failure != tc.want {
				t.Fatal(run.Observations[0].Failure)
			}
			var d artifact.EventData
			json.Unmarshal(run.Observations[0].Data, &d)
			if tc.name == "door charge stays unparsed" {
				if d.DoorsAt != "" || d.ShowAt != "2026-10-09T18:00:00-06:00" ||
					d.AdmissionPolicy.Category != "18+" {
					t.Fatalf("%+v", d)
				}
			}
		})
	}
}

func TestListingFailures(t *testing.T) {
	now := time.Date(2026, 10, 3, 18, 0, 0, 0, time.UTC)
	changed := kalEl()
	changed.description = "A different description"
	later := fixture{id: "2", title: "B", day: "Saturday October 31st", window: "06:00 PM - 10:00 PM", description: "All Ages"}
	earlier := fixture{id: "3", title: "C", day: "Friday October 9th", window: "07:00 PM - 10:00 PM", description: "All Ages"}
	for _, tc := range []struct {
		name string
		raw  func() []byte
	}{
		{"changed check", func() []byte {
			return snapshot([]fixture{kalEl()}, []fixture{changed})
		}},
		{"wrong weekday", func() []byte {
			f := kalEl()
			f.day = "Tuesday October 9th"
			return snapshot([]fixture{f}, nil)
		}},
		{"unsorted dates", func() []byte {
			return snapshot([]fixture{later, earlier}, nil)
		}},
		{"duplicate identity", func() []byte {
			return snapshot([]fixture{kalEl(), kalEl()}, nil)
		}},
		{"identity mismatch", func() []byte {
			return paired(strings.ReplaceAll(listing([]fixture{kalEl()}), `data-event-id="3040484"`, `data-event-id="9999999"`))
		}},
		{"missing heading", func() []byte {
			return paired(strings.ReplaceAll(listing([]fixture{kalEl()}), `<h2>Kal-El Astral Voyaging 2026 North American Tour</h2>`, ``))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Decode(config(t), tc.raw(), now); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}
