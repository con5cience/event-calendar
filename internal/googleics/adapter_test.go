package googleics

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"testing"
	"time"
)

func config(t *testing.T) artifact.SourceConfig {
	t.Helper()
	b := []byte(`{"schema_version":1,"source":{"id":"d3","adapter":"google-calendar-ics"},"state":"new","venue":{"key":"d3","name":"D3 Arts","timezone":"America/Denver","website":"https://www.d3arts.org/calendar/","address":"3632 Morrison Road, Denver, CO","phone":"720-412-2784","admission_policy":{"text":"All Ages Welcome: 90% of our shows are open to all ages","category":"All ages","url":"https://www.d3arts.org/3632-2/"}},"adapter_options":{"recurring":"exclude"}}`)
	cfg, err := artifact.DecodeConfigJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func calendar(body string) string {
	return header("D3 Arts Events Calendar", "America/Denver") + body + "END:VCALENDAR"
}
func header(calname, timezone string) string {
	return "BEGIN:VCALENDAR\r\nPRODID:-//Google Inc//Google Calendar 70.9054//EN\r\nVERSION:2.0\r\nX-WR-CALNAME:" + calname + "\r\nX-WR-TIMEZONE:" + timezone + "\r\n"
}

func vevent(lines ...string) string {
	out := "BEGIN:VEVENT\r\n"
	for _, l := range lines {
		out += l + "\r\n"
	}
	return out + "END:VEVENT\r\n"
}

func snapshot(vevents string) []byte {
	raw, _ := json.Marshal(map[string]any{
		"from": "2026-10-03", "through": "2027-10-03",
		"calendar": calendar(vevents),
	})
	return raw
}

func TestMapping(t *testing.T) {
	now := time.Date(2026, 10, 3, 18, 0, 0, 0, time.UTC)
	body := vevent(
		"UID:moro123abc@google.com",
		"DTSTAMP:20261003T180000Z",
		"DTSTART:20261010T010000Z",
		"DTEND:20261010T040000Z",
		"SUMMARY:Moro\\, Tava\\, and Semifiction",
		"DESCRIPTION:Doors 7pm\\, music 7:30pm\\n$10 suggested donation",
		"STATUS:CONFIRMED",
	) + vevent(
		"UID:ballerine99@google.com",
		"DTSTART;VALUE=DATE:20261114",
		"SUMMARY:Ballerine",
		"STATUS:CONFIRMED",
	) + vevent(
		"UID:show18plus99@google.com",
		"DTSTART:20261031T010000Z",
		"SUMMARY:WolfGirl",
		"DESCRIPTION:18+ night",
		"STATUS:CONFIRMED",
	) + vevent(
		"UID:meeting12step@google.com",
		"DTSTART:20241205T030000Z",
		"SUMMARY:12 STEP MEETING",
		"RRULE:FREQ=WEEKLY;BYDAY=TH",
		"STATUS:CONFIRMED",
	) + vevent(
		"UID:heroinesseries@google.com",
		"DTSTART:20231126T020000Z",
		"SUMMARY:HEROINES",
		"RRULE:FREQ=WEEKLY;UNTIL=20231218T065959Z;BYDAY=SU",
		"STATUS:CONFIRMED",
	) + vevent(
		"UID:meeting12step@google.com",
		"RECURRENCE-ID;TZID=America/Denver:20261015T183000",
		"DTSTART:20261015T020000Z",
		"SUMMARY:12 STEP MEETING",
		"STATUS:CONFIRMED",
	) + vevent(
		"UID:meeting12step@google.com",
		"RECURRENCE-ID;TZID=America/Denver:20240105T180000",
		"DTSTART:20240105T020000Z",
		"SUMMARY:12 STEP MEETING",
		"STATUS:CONFIRMED",
	)
	run, err := Decode(config(t), snapshot(body), now)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]artifact.Observation{}
	for _, o := range run.Observations {
		byID[o.UpstreamID] = o
	}
	if len(byID) != 5 {
		t.Fatalf("%+v", run.Observations)
	}
	excluded := 0
	for _, o := range run.Observations {
		if o.Failure == "recurring series excluded" {
			excluded++
		}
	}
	// The live master and its in-window override share one UID; the ended
	// series and the past override are dropped entirely.
	if excluded != 2 {
		t.Fatalf("live recurring series not excluded: %+v", run.Observations)
	}
	if _, present := byID["heroinesseries@google.com"]; present {
		t.Fatal("ended series kept")
	}
	var moro, ballerine, wolf artifact.EventData
	for _, pair := range []struct {
		id   string
		want *artifact.EventData
	}{
		{"moro123abc@google.com", &moro},
		{"ballerine99@google.com", &ballerine},
		{"show18plus99@google.com", &wolf},
	} {
		o := byID[pair.id]
		if o.Failure != "" {
			t.Fatal(pair.id, o.Failure)
		}
		if json.Unmarshal(o.Data, pair.want) != nil {
			t.Fatal("data")
		}
	}
	if moro.Title != "Moro, Tava, and Semifiction" || moro.Date != "2026-10-09" ||
		moro.DoorsAt != "2026-10-09T19:00:00-06:00" ||
		moro.ShowAt != "2026-10-09T19:30:00-06:00" ||
		moro.AdmissionPolicy != nil {
		t.Fatalf("%+v", moro)
	}
	if ballerine.Date != "2026-11-14" || ballerine.DoorsAt != "" || ballerine.ShowAt != "" {
		t.Fatalf("%+v", ballerine)
	}
	if wolf.Date != "2026-10-30" || wolf.ShowAt != "2026-10-30T19:00:00-06:00" ||
		wolf.AdmissionPolicy == nil || wolf.AdmissionPolicy.Category != "18+" {
		t.Fatalf("%+v", wolf)
	}
	out, err := (&artifact.Reconciler{}).Reconcile(config(t), nil, run, "2026-10-03")
	if err != nil || len(out.Artifact.Events) != 3 || len(out.Rejected) != 2 {
		t.Fatalf("%v %+v", err, out)
	}
}

func TestRecordFailures(t *testing.T) {
	now := time.Date(2026, 10, 3, 18, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, want string
		vevent     string
	}{
		{"labeled mismatch", "conflicting labeled time", vevent(
			"UID:labeled1@google.com",
			"DTSTART:20261010T030000Z",
			"SUMMARY:Labeled",
			"DESCRIPTION:Doors 7pm\\, music 8pm",
			"STATUS:CONFIRMED",
		)},
		{"doors after start", "conflicting doors time", vevent(
			"UID:doorslate1@google.com",
			"DTSTART:20261010T010000Z",
			"SUMMARY:Doors Late",
			"DESCRIPTION:Doors 9pm",
			"STATUS:CONFIRMED",
		)},
		{"show before start", "conflicting show time", vevent(
			"UID:showearly1@google.com",
			"DTSTART:20261010T020000Z",
			"SUMMARY:Show Early",
			"DESCRIPTION:Music 7pm",
			"STATUS:CONFIRMED",
		)},
		{"all-day label conflict", "conflicting doors or show time", vevent(
			"UID:alldayconf@google.com",
			"DTSTART;VALUE=DATE:20261114",
			"SUMMARY:All Day Conf",
			"DESCRIPTION:Doors 7pm",
			"STATUS:CONFIRMED",
		)},
		{"conflicting ages", "conflicting age text", vevent(
			"UID:agesconf1@google.com",
			"DTSTART:20261010T010000Z",
			"SUMMARY:Ages Conf",
			"DESCRIPTION:ALL AGES 18+ night",
			"STATUS:CONFIRMED",
		)},
		{"unreviewed status", "unreviewed event status", vevent(
			"UID:statusbad1@google.com",
			"DTSTART:20261010T010000Z",
			"SUMMARY:Status",
			"STATUS:WOBBLY",
		)},
		{"missing start", "missing start", vevent(
			"UID:nostart1@google.com",
			"SUMMARY:No Start",
			"STATUS:CONFIRMED",
		)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run, err := Decode(config(t), snapshot(tc.vevent), now)
			if err != nil {
				t.Fatal(err)
			}
			if run.Observations[0].Failure != tc.want {
				t.Fatal(run.Observations[0].Failure)
			}
		})
	}
}

func TestRefreshFailures(t *testing.T) {
	now := time.Date(2026, 10, 3, 18, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		raw  []byte
	}{
		{"bad uid shape", snapshot(vevent("UID:not-google", "DTSTART:20261010T010000Z", "SUMMARY:X"))},
		{"duplicate uid", snapshot(vevent("UID:dup1@google.com", "DTSTART:20261010T010000Z", "SUMMARY:X") +
			vevent("UID:dup1@google.com", "DTSTART:20261011T010000Z", "SUMMARY:Y"))},
		{"duplicate property", snapshot(vevent("UID:dupprop@google.com", "SUMMARY:A", "SUMMARY:B", "DTSTART:20261010T010000Z"))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Decode(config(t), tc.raw, now); err == nil {
				t.Fatal("accepted")
			}
		})
	}
	for _, tc := range []struct {
		name, calname, timezone string
	}{
		{"wrong calname", "Some Other Calendar", "America/Denver"},
		{"wrong timezone", "D3 Arts Events Calendar", "America/New_York"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := json.Marshal(map[string]any{
				"from": "2026-10-03", "through": "2027-10-03",
				"calendar": header(tc.calname, tc.timezone) + vevent("UID:hdr1@google.com", "DTSTART:20261010T010000Z", "SUMMARY:X") + "END:VCALENDAR",
			})
			if _, err := Decode(config(t), raw, now); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}
