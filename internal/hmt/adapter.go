// Package hmt normalizes saved HoldMyTicket calendar and JSON-LD detail data.
// It never fetches URLs or executes HTML/scripts.
package hmt

import (
	"bytes"
	"encoding/json"
	"event-calendar/internal/artifact"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode/utf8"

	ics "github.com/arran4/golang-ical"
)

type snapshot struct {
	Calendar string                     `json:"calendar"`
	Details  map[string]json.RawMessage `json:"details"`
}
type detail struct {
	Type                                                    string `json:"@type"`
	Name, StartDate, DoorTime, TypicalAgeRange, EventStatus string
	Location                                                struct{ Name string }
	Performer                                               []struct{ Name string }
	Offers                                                  []struct{ URL string }
}

var eventPath = regexp.MustCompile(`^/event/([1-9][0-9]*)$`)

func Decode(cfg artifact.SourceConfig, b []byte, now time.Time) (artifact.Refresh, error) {
	fail := func(s string) (artifact.Refresh, error) { return artifact.Refresh{}, fmt.Errorf("hmt: %s", s) }
	encoded, err := json.Marshal(cfg)
	if err != nil {
		return fail("invalid configuration")
	}
	cfg, err = artifact.DecodeConfigJSON(encoded)
	if err != nil {
		return artifact.Refresh{}, err
	}
	profiles := map[string]struct{ key, name, website string }{"6457": {"hq", "HQ", "https://www.hqdenver.com"}, "801": {"oriental", "The Oriental Theater", "https://www.theorientaltheater.com"}, "8693": {"federal", "The Federal Theatre", "https://thefederaltheatre.com"}}
	p, ok := profiles[cfg.AdapterOptions["feed_id"]]
	if !ok || cfg.Source.Adapter != "holdmyticket-ical" || cfg.Source.ID != p.key || cfg.Venue.Key != p.key || cfg.Venue.Name != p.name || cfg.Venue.Website != p.website || cfg.Venue.Timezone != "America/Denver" || len(cfg.AdapterOptions) != 1 {
		return fail("unsupported source configuration")
	}
	if now.IsZero() || len(b) > artifact.MaxDocumentBytes || !utf8.Valid(b) {
		return fail("invalid clock or snapshot size/encoding")
	}
	var s snapshot
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return fail("invalid snapshot JSON")
	}
	if dec.Decode(new(any)) != io.EOF || s.Details == nil {
		return fail("missing details or trailing JSON")
	}
	trimmed := strings.TrimSpace(s.Calendar)
	if !strings.HasPrefix(trimmed, "BEGIN:VCALENDAR") || !strings.HasSuffix(trimmed, "END:VCALENDAR") || strings.Count(trimmed, "BEGIN:VCALENDAR") != 1 || strings.Count(trimmed, "BEGIN:VEVENT") != strings.Count(trimmed, "END:VEVENT") {
		return fail("incomplete calendar envelope")
	}
	calendar := s.Calendar
	if p.key == "federal" {
		calendar, err = stripFederalDescriptions(calendar)
		if err != nil {
			return fail(err.Error())
		}
	}
	cal, err := ics.ParseCalendar(strings.NewReader(calendar))
	if err != nil {
		return fail("invalid iCalendar")
	}
	props := map[string]string{}
	for _, prop := range cal.CalendarProperties {
		props[prop.IANAToken] = strings.TrimSpace(prop.Value)
	}
	if props["X-WR-TIMEZONE"] != "America/Denver" || props["X-WR-CALNAME"] != p.name {
		return fail("calendar timezone or venue mismatch")
	}
	loc, _ := time.LoadLocation(cfg.Venue.Timezone)
	today := now.In(loc)
	r := artifact.Refresh{GeneratedAt: now.UTC().Format(time.RFC3339Nano), Coverage: artifact.Coverage{From: today.Format("2006-01-02"), Through: today.AddDate(1, 0, 0).Format("2006-01-02"), Complete: true}, Observations: []artifact.Observation{}}
	ids, uids := map[string]bool{}, map[string]bool{}
	for _, e := range cal.Events() {
		fields := map[string]string{}
		for _, prop := range e.Properties {
			if _, exists := fields[prop.IANAToken]; exists {
				return fail("duplicate event property")
			}
			fields[prop.IANAToken] = strings.TrimSpace(prop.Value)
			if prop.IANAToken == "RRULE" || prop.IANAToken == "RDATE" || prop.IANAToken == "EXDATE" || prop.IANAToken == "RECURRENCE-ID" {
				return fail("recurrence requires explicit support")
			}
		}
		u, err := url.Parse(fields["URL"])
		if err != nil || u.User != nil || u.Host != "holdmyticket.com" || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" {
			return fail("invalid provider event URL")
		}
		match := eventPath.FindStringSubmatch(u.Path)
		if match == nil || fields["UID"] == "" {
			return fail("missing event identity")
		}
		id := match[1]
		if ids[id] || uids[fields["UID"]] {
			return fail("duplicate event identity")
		}
		ids[id] = true
		uids[fields["UID"]] = true
		data, err := normalize(e, fields, s.Details[id], p.name, id, loc)
		// Federal review (2026-09-10): explicit All Ages event evidence supports
		// child admission. No accompaniment exception for restricted shows is
		// inferred. Reconciliation still applies configured event overrides.
		if err == nil && p.key == "federal" && data.AdmissionPolicy != nil && data.AdmissionPolicy.Category == "All ages" {
			data.AdmissionPolicy.WithAdult = &artifact.AdultAdmission{
				URL: data.EventURL, ReviewedOn: "2026-09-10",
				Ranges: []artifact.AdmissionRange{{MinAge: 0, MaxAge: 17, Condition: "All ages event; attend with adult and follow event conditions"}},
			}
		}
		failure := ""
		if err != nil {
			failure = err.Error()
		}
		if failure == "" && (data.Date < r.Coverage.From || data.Date > r.Coverage.Through) {
			continue
		}
		raw, _ := json.Marshal(data)
		r.Observations = append(r.Observations, artifact.Observation{UpstreamID: id, Data: raw, Failure: failure})
	}
	if len(cal.Events()) != strings.Count(trimmed, "BEGIN:VEVENT") {
		return fail("calendar parser omitted events")
	}
	for id := range s.Details {
		if !ids[id] {
			return fail("detail does not belong to calendar")
		}
	}
	return r, nil
}

func localTime(value, layout string, loc *time.Location) (time.Time, error) {
	t, err := time.ParseInLocation(layout, value, loc)
	if err != nil || t.Format(layout) != value {
		return time.Time{}, fmt.Errorf("invalid local time")
	}
	// Reject gaps and overlaps instead of choosing a DST occurrence silently.
	for _, offset := range []time.Duration{-time.Hour, time.Hour} {
		if t.Add(offset).Format(layout) == value {
			return time.Time{}, fmt.Errorf("ambiguous local time")
		}
	}
	return t, nil
}

func normalize(e *ics.VEvent, fields map[string]string, raw json.RawMessage, venue, id string, loc *time.Location) (artifact.EventData, error) {
	data := artifact.EventData{EventURL: "https://holdmyticket.com/event/" + id}
	bad := func(s string) (artifact.EventData, error) { return data, fmt.Errorf("hmt: %s", s) }
	var d detail
	if len(raw) == 0 || json.Unmarshal(raw, &d) != nil || d.Type != "Event" {
		return bad("missing or invalid event detail")
	}
	if d.Location.Name != venue {
		return bad("unexpected detail venue")
	}
	data.Title = strings.TrimSpace(d.Name)
	if data.Title == "" || data.Title != strings.TrimSpace(fields["SUMMARY"]) {
		return bad("conflicting or missing event title")
	}
	start := e.GetProperty(ics.ComponentPropertyDtStart)
	if start == nil {
		return bad("missing start")
	}
	if zones := start.ICalParameters["TZID"]; len(zones) > 0 && (len(zones) != 1 || zones[0] != loc.String()) {
		return bad("unsupported event timezone")
	}
	value := fields["DTSTART"]
	var t time.Time
	var err error
	allDay := len(value) == 8
	if allDay {
		if values := start.ICalParameters["VALUE"]; len(values) != 1 || values[0] != "DATE" {
			return bad("date requires VALUE=DATE")
		}
		t, err = time.ParseInLocation("20060102", value, loc)
	} else if strings.HasSuffix(value, "Z") {
		t, err = time.Parse("20060102T150405Z", value)
	} else {
		t, err = localTime(value, "20060102T150405", loc)
	}
	if err != nil {
		return bad("invalid or ambiguous calendar time")
	}
	data.Date = t.In(loc).Format("2006-01-02")
	if allDay {
		if d.StartDate != data.Date || d.DoorTime != "" {
			return bad("date-only detail conflicts")
		}
	} else {
		detailTime, err := time.Parse(time.RFC3339, d.StartDate)
		if err != nil || !detailTime.Equal(t) || detailTime.Format("-07:00") != detailTime.In(loc).Format("-07:00") {
			return bad("calendar and detail show time conflict")
		}
		data.ShowAt = t.In(loc).Format(time.RFC3339)
		if d.DoorTime != "" {
			doors, err := localTime(data.Date+"T"+d.DoorTime, "2006-01-02T15:04", loc)
			if err != nil || doors.After(t) {
				return bad("invalid or ambiguous doors time")
			}
			data.DoorsAt = doors.Format(time.RFC3339)
		}
	}
	for _, p := range d.Performer {
		if name := strings.TrimSpace(p.Name); name != "" {
			data.Performers = append(data.Performers, name)
		}
	}
	if age := strings.TrimSpace(d.TypicalAgeRange); age != "" {
		categories := map[string]string{"All Ages": "All ages", "All Ages / Bar with ID": "All ages"}
		for _, category := range []string{"13+", "16+", "18+", "21+"} {
			categories[category+" Ages"] = category
			categories[category+" / Bar with ID"] = category
		}
		data.AdmissionPolicy = &artifact.AdmissionPolicy{Text: age, Category: categories[age]}
	}
	switch strings.ToLower(strings.TrimSuffix(d.EventStatus, "/")) {
	case "https://schema.org/eventcancelled", "http://schema.org/eventcancelled":
		data.Status = "Cancelled"
	case "https://schema.org/eventscheduled", "http://schema.org/eventscheduled", "":
		data.Status = "Scheduled"
	default:
		return bad("unsupported event status")
	}
	if strings.EqualFold(fields["STATUS"], "CANCELLED") {
		data.Status = "Cancelled"
	} else if fields["STATUS"] != "" && fields["STATUS"] != "CONFIRMED" && fields["STATUS"] != "TENTATIVE" {
		return bad("unsupported calendar status")
	}
	for _, offer := range d.Offers {
		if offer.URL != "" {
			u, err := url.Parse(offer.URL)
			if err != nil || u.Scheme != "https" || u.Host != "tickets.holdmyticket.com" || u.User != nil || u.Path != "/tickets/"+id || u.RawQuery != "" || u.Fragment != "" {
				return bad("unexpected ticket URL")
			}
			data.TicketURL = offer.URL
		}
	}
	return data, nil
}
