package reelworks

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"strings"
	"testing"
	"time"
)

const faqHTML = `<main>Is the Show 18+? That depends on the promoter putting on the show. That information can be found on the event page for the show.</main>`

type fixture struct {
	slug, title, date, support, start, end, age, offer, buy, status, mode, location string
}

func base() fixture {
	return fixture{
		slug: "deorro-3", title: "DEORRO", date: "Friday, Oct 2, 2026", support: "WITH LA PATRONA",
		start: "2026-10-02T21:00:00-06:00", end: "2026-10-03T02:00:00-06:00",
		age: "18+", offer: "https://www.axs.com/events/1621884/deorro-tickets",
		buy:      "https://www.axs.com/events/1621884/deorro-tickets",
		status:   "https://schema.org/EventScheduled",
		mode:     "https://schema.org/OfflineEventAttendanceMode",
		location: "ReelWorks Denver",
	}
}

func config(t *testing.T) artifact.SourceConfig {
	t.Helper()
	b := []byte(`{"schema_version":1,"source":{"id":"reelworks","adapter":"reelworks-wordpress-events"},"state":"new","venue":{"key":"reelworks","name":"ReelWorks Denver","timezone":"America/Denver","website":"https://reelworksdenver.com/events/","address":"1399 35th St, Denver, CO 80205","phone":"+1-303-468-5443"}}`)
	cfg, err := artifact.DecodeConfigJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func listingHTML(fs []fixture) string {
	var b strings.Builder
	for _, f := range fs {
		b.WriteString(`<div class="event-item"><a href="https://reelworksdenver.com/event/` + f.slug + `/" class="event-image"><img alt=""/></a><div class="event-item-info"><a href="https://reelworksdenver.com/event/` + f.slug + `/"><h3>` + f.date + `</h3><h1>` + f.title + `</h1><h4>` + f.support + `</h4></a><a href="https://reelworksdenver.com/event/` + f.slug + `/" class="event-btn button">More Info</a>`)
		if f.buy != "" {
			b.WriteString(`<a href="` + f.buy + `" class="event-btn button" target="_blank">Buy Tickets</a>`)
		}
		b.WriteString(`</div></div>`)
	}
	return b.String()
}

func clockOf(at string) string {
	t, _ := time.Parse(time.RFC3339, at)
	return t.Format("3:04 pm")
}
func dayOf(date string) time.Time {
	t, _ := time.Parse("Monday, Jan 2, 2006", date)
	return t
}
func dayText(t time.Time) string { return t.Format("Monday, Jan 2, 2006") }

func jsonld(f fixture) string {
	offer := ""
	if f.offer != "" {
		offer = `,"offers":{"@type":"Offer","url":"` + f.offer + `","availability":"https://schema.org/InStock"}`
	}
	return `{"@context":"https://schema.org","@graph":[{"@type":"WebPage","url":"https://reelworksdenver.com/event/` + f.slug + `/"},{"@type":"MusicEvent","url":"https://reelworksdenver.com/event/` + f.slug + `/","name":"` + f.title + `","startDate":"` + f.start + `","endDate":"` + f.end + `","eventStatus":"` + f.status + `","eventAttendanceMode":"` + f.mode + `","location":{"@type":"MusicVenue","name":"` + f.location + `"},"performer":{"@type":"MusicGroup","name":"` + f.title + `"}` + offer + `}]}`
}

func build(fs []fixture, faq string) pages {
	events := map[string]eventPage{}
	for _, f := range fs {
		end, _ := time.Parse(time.RFC3339, f.end)
		events[f.slug] = eventPage{
			JSONLD: jsonld(f),
			Sidebar: sidebar{
				Date: dayText(dayOf(f.date)) + " - " + dayText(end),
				Time: clockOf(f.start) + " - " + clockOf(f.end),
				Age:  f.age,
			},
		}
	}
	return pages{Listing: listingHTML(fs), FAQ: faq, Events: events}
}

func snapshot(fs, check []fixture, faq string) []byte {
	if check == nil {
		check = fs
	}
	if faq == "" {
		faq = faqHTML
	}
	raw, _ := json.Marshal(map[string]any{
		"from": "2026-10-02", "through": "2027-10-02",
		"pages": build(fs, faq), "check": build(check, faq),
	})
	return raw
}

// A listing that links two events while only one event page was captured.
func mismatched() []byte {
	f := base()
	later := base()
	later.slug, later.title, later.date = "blanke", "Blanke", "Saturday, Feb 6, 2027"
	events := map[string]eventPage{}
	for _, x := range []fixture{f} {
		events[x.slug] = eventPage{JSONLD: jsonld(x), Sidebar: sidebar{
			Date: "Friday, Oct 2, 2026 - Saturday, Oct 3, 2026",
			Time: "9:00 pm - 2:00 am", Age: x.age,
		}}
	}
	raw, _ := json.Marshal(map[string]any{
		"from": "2026-10-02", "through": "2027-10-02",
		"pages": pages{Listing: listingHTML([]fixture{f, later}), FAQ: faqHTML, Events: events},
		"check": pages{Listing: listingHTML([]fixture{f, later}), FAQ: faqHTML, Events: events},
	})
	return raw
}

func TestMapping(t *testing.T) {
	now := time.Date(2026, 10, 2, 18, 0, 0, 0, time.UTC)
	cfg := config(t)
	frost := base()
	frost.slug, frost.title, frost.date = "frost-children", "Frost Children", "Tuesday, Dec 1, 2026"
	frost.support = ""
	frost.start, frost.end = "2026-12-01T20:00:00-07:00", "2026-12-01T23:00:00-07:00"
	frost.age = "16+ (Under 16 w/ adult guardian)"
	frost.offer = "https://www.axs.com/events/1559274/frost-children-tickets"
	frost.buy = frost.offer
	run, err := Decode(cfg, snapshot([]fixture{base(), frost}, nil, ""), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Observations) != 2 {
		t.Fatalf("%+v", run.Observations)
	}
	var deorro, cold artifact.EventData
	for _, o := range run.Observations {
		if o.Failure != "" {
			t.Fatal(o.Failure)
		}
		var d artifact.EventData
		if json.Unmarshal(o.Data, &d) != nil {
			t.Fatal("data")
		}
		if o.UpstreamID == "https://reelworksdenver.com/event/deorro-3/" {
			deorro = d
		} else {
			cold = d
		}
	}
	if deorro.Title != "DEORRO" || deorro.Date != "2026-10-02" ||
		deorro.DoorsAt != "2026-10-02T21:00:00-06:00" || deorro.ShowAt != "" ||
		deorro.Performers[0] != "DEORRO" ||
		deorro.TicketURL != "https://www.axs.com/events/1621884/deorro-tickets" ||
		deorro.AdmissionPolicy.Category != "18+" || deorro.AdmissionPolicy.WithAdult != nil {
		t.Fatalf("%+v", deorro)
	}
	if cold.Date != "2026-12-01" || cold.DoorsAt != "2026-12-01T20:00:00-07:00" ||
		cold.AdmissionPolicy.Category != "16+" || cold.AdmissionPolicy.WithAdult == nil ||
		cold.AdmissionPolicy.WithAdult.Ranges[0].MaxAge != 15 ||
		cold.AdmissionPolicy.WithAdult.Ranges[0].Condition != "Under 16 w/ adult guardian" ||
		cold.AdmissionPolicy.WithAdult.URL != cold.EventURL {
		t.Fatalf("%+v", cold)
	}
	out, err := (&artifact.Reconciler{}).Reconcile(cfg, nil, run, "2026-10-02")
	if err != nil || len(out.Artifact.Events) != 2 || len(out.Rejected) != 0 {
		t.Fatalf("%v %+v", err, out)
	}
}

func TestAgeVariants(t *testing.T) {
	now := time.Date(2026, 10, 2, 18, 0, 0, 0, time.UTC)
	for kind, want := range map[string]string{
		"21": "21+", "18+": "18+", "All Ages": "All ages", "16": "16+",
	} {
		f := base()
		f.age = kind
		run, err := Decode(config(t), snapshot([]fixture{f}, nil, ""), now)
		if err != nil {
			t.Fatal(kind, err)
		}
		var d artifact.EventData
		json.Unmarshal(run.Observations[0].Data, &d)
		if run.Observations[0].Failure != "" || d.AdmissionPolicy.Category != want {
			t.Fatalf("%s: %s %+v", kind, run.Observations[0].Failure, d.AdmissionPolicy)
		}
	}
}

func TestRecordFailures(t *testing.T) {
	now := time.Date(2026, 10, 2, 18, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, want string
		mutate     func(*fixture)
	}{
		{"offsite", "unreviewed venue", func(f *fixture) { f.location = "Tracks" }},
		{"missing age", "missing age", func(f *fixture) { f.age = "" }},
		{"bad age", "unreviewed age", func(f *fixture) { f.age = "19+" }},
		{"age condition", "unreviewed age condition", func(f *fixture) { f.age = "16+ (Under 18 w/ adult)" }},
		{"ticket host", "unsupported ticket provider", func(f *fixture) {
			f.offer = "https://tickets.example.com/deorro"
			f.buy = f.offer
		}},
		{"ticket mismatch", "conflicting ticket link", func(f *fixture) {
			f.buy = "https://www.axs.com/events/999999/other-tickets"
		}},
		{"ticket button absent", "", func(f *fixture) { f.buy = "" }},
		{"status", "unsupported event status", func(f *fixture) { f.status = "https://schema.org/EventPostponed" }},
		{"mode", "unsupported attendance mode", func(f *fixture) { f.mode = "https://schema.org/OnlineEventAttendanceMode" }},
		{"cancelled", "", func(f *fixture) { f.status = "https://schema.org/EventCancelled" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := base()
			tc.mutate(&f)
			run, err := Decode(config(t), snapshot([]fixture{f}, nil, ""), now)
			if err != nil {
				t.Fatal(err)
			}
			if run.Observations[0].Failure != tc.want {
				t.Fatal(run.Observations[0].Failure)
			}
			var d artifact.EventData
			json.Unmarshal(run.Observations[0].Data, &d)
			if tc.name == "cancelled" && d.Status != "Cancelled" {
				t.Fatal(d)
			}
		})
	}
}

func editBoth(raw []byte, edit func(pages map[string]any)) []byte {
	var body map[string]any
	if json.Unmarshal(raw, &body) != nil {
		panic("snapshot")
	}
	for _, side := range []string{"pages", "check"} {
		edit(body[side].(map[string]any))
	}
	out, _ := json.Marshal(body)
	return out
}

func TestSidebarConflicts(t *testing.T) {
	now := time.Date(2026, 10, 2, 18, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, want, date, clock string
	}{
		{"doors", "conflicting doors", "Friday, Oct 2, 2026 - Saturday, Oct 3, 2026", "10:00 pm - 2:00 am"},
		{"date", "conflicting date", "Friday, Oct 9, 2026 - Saturday, Oct 10, 2026", "9:00 pm - 2:00 am"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := editBoth(snapshot([]fixture{base()}, nil, ""), func(pages map[string]any) {
				side := pages["events"].(map[string]any)["deorro-3"].(map[string]any)["sidebar"].(map[string]any)
				side["date"] = tc.date
				side["time"] = tc.clock
			})
			run, err := Decode(config(t), out, now)
			if err != nil {
				t.Fatal(err)
			}
			if run.Observations[0].Failure != tc.want {
				t.Fatal(run.Observations[0].Failure)
			}
		})
	}
}

func TestListingTitleConflict(t *testing.T) {
	now := time.Date(2026, 10, 2, 18, 0, 0, 0, time.UTC)
	out := editBoth(snapshot([]fixture{base()}, nil, ""), func(pages map[string]any) {
		pages["listing"] = strings.Replace(
			pages["listing"].(string), "<h1>DEORRO</h1>", "<h1>Deorro</h1>", 2,
		)
	})
	run, err := Decode(config(t), out, now)
	if err != nil {
		t.Fatal(err)
	}
	if run.Observations[0].Failure != "conflicting or missing title" {
		t.Fatal(run.Observations[0].Failure)
	}
}

func TestRefreshFailures(t *testing.T) {
	now := time.Date(2026, 10, 2, 18, 0, 0, 0, time.UTC)
	other := base()
	other.slug, other.title = "hedex", "Hedex"
	for _, tc := range []struct {
		name  string
		fs    []fixture
		check []fixture
		faq   string
	}{
		{"changed", []fixture{base()}, []fixture{other}, ""},
		{"policy", []fixture{base()}, nil, `<main>Everyone is welcome at all times.</main>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decode(config(t), snapshot(tc.fs, tc.check, tc.faq), now)
			if err == nil {
				t.Fatal("accepted")
			}
		})
	}
	if _, err := Decode(config(t), mismatched(), now); err == nil {
		t.Fatal("accepted listing mismatch")
	}
}

func TestUnsortedListing(t *testing.T) {
	now := time.Date(2026, 10, 2, 18, 0, 0, 0, time.UTC)
	later := base()
	later.slug, later.title, later.date = "blanke", "Blanke", "Saturday, Feb 6, 2027"
	raw := snapshot([]fixture{later, base()}, nil, "")
	if _, err := Decode(config(t), raw, now); err == nil || !strings.Contains(err.Error(), "unsorted") {
		t.Fatal("accepted unsorted listing")
	}
}

func TestCoverageMismatch(t *testing.T) {
	raw := snapshot([]fixture{base()}, nil, "")
	var body map[string]any
	json.Unmarshal(raw, &body)
	body["from"] = "2026-10-01"
	out, _ := json.Marshal(body)
	now := time.Date(2026, 10, 2, 18, 0, 0, 0, time.UTC)
	if _, err := Decode(config(t), out, now); err == nil {
		t.Fatal("accepted coverage mismatch")
	}
}
