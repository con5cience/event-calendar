// Package googleics reads a venue's public Google Calendar iCal feed. It
// never fetches URLs; the operator capture supplies the raw feed.
package googleics

import (
	"bytes"
	"encoding/json"
	"event-calendar/internal/artifact"
	"fmt"
	ics "github.com/arran4/golang-ical"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode/utf8"
)

const calendarURL = "https://calendar.google.com/calendar/ical/mkjl322tg56ecg0mf7qjvtllcc%40group.calendar.google.com/public/basic.ics"

var (
	uidShape    = regexp.MustCompile(`^([a-z0-9]+(_R[0-9T]+)?|[0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12})@google\.com$|^([a-z0-9]+|[0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12})$`)
	untilPart   = regexp.MustCompile(`(?i)UNTIL=(\d{8})`)
	doorsText   = regexp.MustCompile(`(?i)\bdoors?\s*[@:]?\s*(\d{1,2})(?::(\d{2}))?(?:\s*(a|p)\.?m?\.?)?`)
	showText    = regexp.MustCompile(`(?i)\b(?:music|show)\s*[@:]?\s*(\d{1,2})(?::(\d{2}))?(?:\s*(a|p)\.?m?\.?)?`)
	allAgesText = regexp.MustCompile(`(?i)\ball ages\b`)
	agePlusText = regexp.MustCompile(`(?i)(?:\bages?\s+)?\b(\d{1,2})\s*\+`)
	spaces      = regexp.MustCompile(`\s+`)
)

type record struct {
	ID      string
	Data    artifact.EventData
	Failure string
}

func Decode(cfg artifact.SourceConfig, b []byte, now time.Time) (artifact.Refresh, error) {
	fail := func(s string) (artifact.Refresh, error) {
		return artifact.Refresh{}, fmt.Errorf("googleics: %s", s)
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return fail("configuration")
	}
	cfg, err = artifact.DecodeConfigJSON(raw)
	if err != nil {
		return fail("configuration")
	}
	p := cfg.Venue.AdmissionPolicy
	if cfg.Source.ID != "d3" || cfg.Source.Adapter != "google-calendar-ics" ||
		cfg.Venue.Key != "d3" || cfg.Venue.Name != "D3 Arts" ||
		cfg.Venue.Timezone != "America/Denver" || cfg.Venue.Website != "https://www.d3arts.org/calendar/" ||
		cfg.Venue.Address != "3632 Morrison Road, Denver, CO" ||
		cfg.Venue.Phone != "720-412-2784" || p == nil ||
		p.Text != "All Ages Welcome: 90% of our shows are open to all ages" ||
		p.Category != "All ages" || p.URL != "https://www.d3arts.org/3632-2/" ||
		len(cfg.AdapterOptions) != 1 || cfg.AdapterOptions["recurring"] != "exclude" ||
		len(cfg.AdmissionRules) != 0 || len(cfg.Overrides) != 0 {
		return fail("unsupported configuration")
	}
	if now.IsZero() || len(b) > artifact.MaxDocumentBytes || !utf8.Valid(b) {
		return fail("invalid snapshot")
	}
	var s struct {
		From, Through string
		Calendar      string
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	if dec.Decode(&s) != nil || dec.Decode(new(any)) != io.EOF {
		return fail("invalid envelope")
	}
	loc, _ := time.LoadLocation(cfg.Venue.Timezone)
	today := now.In(loc)
	if s.From != today.Format("2006-01-02") || s.Through != today.AddDate(1, 0, 0).Format("2006-01-02") {
		return fail("coverage mismatch")
	}
	trimmed := strings.TrimSpace(s.Calendar)
	if !strings.HasPrefix(trimmed, "BEGIN:VCALENDAR") || !strings.HasSuffix(trimmed, "END:VCALENDAR") ||
		strings.Count(trimmed, "BEGIN:VCALENDAR") != 1 ||
		strings.Count(trimmed, "BEGIN:VEVENT") != strings.Count(trimmed, "END:VEVENT") {
		return fail("incomplete calendar envelope")
	}
	cal, err := ics.ParseCalendar(strings.NewReader(s.Calendar))
	if err != nil {
		return fail("invalid iCalendar")
	}
	props := map[string]string{}
	for _, prop := range cal.CalendarProperties {
		props[prop.IANAToken] = strings.TrimSpace(prop.Value)
	}
	if props["X-WR-TIMEZONE"] != cfg.Venue.Timezone || props["X-WR-CALNAME"] != "D3 Arts Events Calendar" {
		return fail("calendar timezone or venue mismatch")
	}
	rows, err := events(cal, loc, s.From, s.Through)
	if err != nil {
		return fail(err.Error())
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	out := artifact.Refresh{
		GeneratedAt:  now.UTC().Format(time.RFC3339Nano),
		Coverage:     artifact.Coverage{From: s.From, Through: s.Through, Complete: true},
		Observations: []artifact.Observation{},
	}
	for _, r := range rows {
		if r.Data.Date != "" && (r.Data.Date < s.From || r.Data.Date > s.Through) {
			continue
		}
		raw, _ := json.Marshal(r.Data)
		out.Observations = append(out.Observations, artifact.Observation{
			UpstreamID: r.ID, Data: raw, Failure: r.Failure,
		})
	}
	return out, nil
}

func localTime(value string, loc *time.Location) (time.Time, error) {
	t, err := time.ParseInLocation("20060102T150405", value, loc)
	if err != nil || t.Format("20060102T150405") != value {
		return time.Time{}, fmt.Errorf("invalid local time")
	}
	for _, offset := range []time.Duration{-time.Hour, time.Hour} {
		if t.Add(offset).Format("20060102T150405") == value {
			return time.Time{}, fmt.Errorf("ambiguous local time")
		}
	}
	return t, nil
}

func clockOn(date string, m []string, loc *time.Location) (time.Time, error) {
	minutes := m[2]
	if minutes == "" {
		minutes = "00"
	}
	meridiem := "PM"
	if strings.EqualFold(m[3], "a") {
		meridiem = "AM"
	}
	clock, err := time.Parse("3:04 PM", m[1]+":"+minutes+" "+meridiem)
	if err != nil {
		return time.Time{}, err
	}
	wall := date + "T" + clock.Format("15:04")
	at, err := time.ParseInLocation("2006-01-02T15:04", wall, loc)
	if err != nil || at.Format("2006-01-02T15:04") != wall ||
		at.Add(time.Hour).Format("2006-01-02T15:04") == wall ||
		at.Add(-time.Hour).Format("2006-01-02T15:04") == wall {
		return time.Time{}, fmt.Errorf("invalid or ambiguous Denver clock")
	}
	return at, nil
}

// markedRecurring reports whether the event carries any recurrence marker,
// live or ended: a master RRULE, an RDATE, a RECURRENCE-ID override, or an
// EXDATE. Google expresses one series as a master plus RECURRENCE-ID
// overrides sharing the master's UID.
func markedRecurring(e *ics.VEvent) bool {
	for _, prop := range e.Properties {
		if prop.IANAToken == "RRULE" || prop.IANAToken == "RDATE" ||
			prop.IANAToken == "EXDATE" || prop.IANAToken == "RECURRENCE-ID" {
			return true
		}
	}
	return false
}

// recurring reports whether a marked series is still live in the window: a
// master is live without UNTIL or with UNTIL at or after the window start,
// and an override is live when its instance date is inside the window.
func recurring(cal *ics.VEvent, from string) (live bool) {
	for _, prop := range cal.Properties {
		switch prop.IANAToken {
		case "RRULE":
			if m := untilPart.FindStringSubmatch(prop.Value); m != nil {
				return m[1] >= from
			}
			return true
		case "RDATE":
			return true
		case "RECURRENCE-ID":
			value := strings.ReplaceAll(strings.ReplaceAll(prop.Value, "-", ""), ":", "")
			return len(value) >= 8 && value[:8] >= from
		}
	}
	return false
}

var digitsOnly = regexp.MustCompile(`[^0-9]+`)

func digitsOf(value string) string {
	return digitsOnly.ReplaceAllString(strings.TrimSpace(value), "")
}

func dateOf(digits string) string {
	if len(digits) < 8 {
		return ""
	}
	day, err := time.Parse("20060102", digits[:8])
	if err != nil {
		return ""
	}
	return day.Format("2006-01-02")
}

// RFC 5545 permits these properties to repeat; every other duplicated
// property fails the capture.
var repeatable = map[string]bool{"EXDATE": true, "RDATE": true}

func events(cal *ics.Calendar, loc *time.Location, from, through string) ([]record, error) {
	seen := map[string]bool{}
	rows := []record{}
	for _, e := range cal.Events() {
		fields := map[string]string{}
		for _, prop := range e.Properties {
			if _, exists := fields[prop.IANAToken]; exists && !repeatable[prop.IANAToken] {
				return nil, fmt.Errorf("duplicate event property")
			}
			fields[prop.IANAToken] = strings.TrimSpace(prop.Value)
		}
		uid := fields["UID"]
		if uid == "" || !uidShape.MatchString(uid) {
			return nil, fmt.Errorf("invalid event identity")
		}
		// Recurring series repeat one UID across the master and every override;
		// uniqueness applies to one-off events only.
		recurring := recurring(e, from)
		if !recurring && !markedRecurring(e) {
			if seen[uid] {
				return nil, fmt.Errorf("duplicate event identity")
			}
			seen[uid] = true
		}
		r := record{ID: uid}
		if recurring {
			// Excluded series stay visible in every report: Google repeats one
			// UID across the master and its overrides, so overrides carry the
			// instance digits on their identity. The records stay undated; the
			// window filter keeps undated records.
			data := artifact.EventData{Status: "Scheduled", EventURL: "https://www.d3arts.org/calendar/"}
			if instance := digitsOf(fields["RECURRENCE-ID"]); instance != "" {
				r.ID = uid + "@" + instance
			}
			r.Data = data
			r.Failure = "recurring series excluded"
			rows = append(rows, r)
			continue
		}
		data, ferr := normalize(e, fields, loc, from, through)
		r.Data = data
		if ferr != nil {
			r.Failure = ferr.Error()
		}
		rows = append(rows, r)
	}
	return rows, nil
}

// A timed event starts at DTSTART in Denver time; Google emits UTC (Z),
// TZID-qualified, or floating local values. The description's labeled doors
// and show times cross-check it: the start must equal one of them when they
// are present, so the venue's own text verifies the stored time.
func normalize(e *ics.VEvent, fields map[string]string, loc *time.Location, from, through string) (artifact.EventData, error) {
	d := artifact.EventData{Status: "Scheduled", EventURL: "https://www.d3arts.org/calendar/"}
	d.Title = strings.TrimSpace(fields["SUMMARY"])
	if d.Title == "" {
		return d, fmt.Errorf("missing title")
	}
	switch fields["STATUS"] {
	case "CANCELLED":
		d.Status = "Cancelled"
	case "TENTATIVE", "CONFIRMED", "":
	default:
		return d, fmt.Errorf("unreviewed event status")
	}
	startProp := e.GetProperty(ics.ComponentPropertyDtStart)
	if startProp == nil {
		return d, fmt.Errorf("missing start")
	}
	description := strings.ReplaceAll(fields["DESCRIPTION"], `\n`, "\n")
	allDay := false
	var start time.Time
	var err error
	value := fields["DTSTART"]
	switch {
	case len(value) == 8:
		if values := startProp.ICalParameters["VALUE"]; len(values) != 1 || values[0] != "DATE" {
			return d, fmt.Errorf("date requires VALUE=DATE")
		}
		var day time.Time
		day, err = time.Parse("20060102", value)
		if err != nil {
			return d, fmt.Errorf("invalid date")
		}
		d.Date = day.Format("2006-01-02")
		allDay = true
	case strings.HasSuffix(value, "Z"):
		start, err = time.Parse("20060102T150405Z", value)
		if err != nil {
			return d, fmt.Errorf("invalid start")
		}
		start = start.In(loc)
		d.Date = start.Format("2006-01-02")
		d.ShowAt = start.Format(time.RFC3339)
	default:
		if zones := startProp.ICalParameters["TZID"]; len(zones) == 1 && zones[0] != loc.String() {
			return d, fmt.Errorf("unsupported event timezone")
		}
		start, err = localTime(value, loc)
		if err != nil {
			return d, fmt.Errorf("invalid or ambiguous start")
		}
		d.Date = start.Format("2006-01-02")
		d.ShowAt = start.Format(time.RFC3339)
	}
	if allDay {
		if doorsText.MatchString(description) || showText.MatchString(description) {
			return d, fmt.Errorf("conflicting doors or show time")
		}
	} else {
		doors := doorsText.FindAllStringSubmatch(description, -1)
		shows := showText.FindAllStringSubmatch(description, -1)
		if len(doors) > 1 {
			return d, fmt.Errorf("conflicting doors time")
		}
		if len(shows) > 1 {
			return d, fmt.Errorf("conflicting show time")
		}
		var doorsAt, showAt time.Time
		if len(doors) == 1 {
			t, err := clockOn(d.Date, doors[0], loc)
			if err != nil {
				return d, fmt.Errorf("conflicting doors time")
			}
			doorsAt = t
			d.DoorsAt = t.Format(time.RFC3339)
		}
		if len(shows) == 1 {
			t, err := clockOn(d.Date, shows[0], loc)
			if err != nil {
				return d, fmt.Errorf("conflicting show time")
			}
			showAt = t
			d.ShowAt = t.Format(time.RFC3339)
		}
		// The venue's own text verifies the stored start: DTSTART equals the
		// labeled doors or show clock, and doors never follow the show.
		switch {
		case len(doors) == 1 && len(shows) == 1:
			if doorsAt.After(showAt) || (!doorsAt.Equal(start) && !showAt.Equal(start)) {
				return d, fmt.Errorf("conflicting labeled time")
			}
		case len(doors) == 1:
			if !doorsAt.Equal(start) {
				return d, fmt.Errorf("conflicting doors time")
			}
		case len(shows) == 1:
			if !showAt.Equal(start) {
				return d, fmt.Errorf("conflicting show time")
			}
		}
	}
	if end := e.GetProperty(ics.ComponentPropertyDtEnd); end != nil && !allDay {
		endValue := fields["DTEND"]
		var end time.Time
		switch {
		case strings.HasSuffix(endValue, "Z"):
			end, _ = time.Parse("20060102T150405Z", endValue)
		case len(endValue) == 8:
			return d, fmt.Errorf("conflicting end")
		default:
			end, _ = localTime(endValue, loc)
		}
		if end.IsZero() || !end.After(start) {
			return d, fmt.Errorf("invalid end")
		}
	}
	all := allAgesText.MatchString(description)
	ages := agePlusText.FindAllStringSubmatch(description, -1)
	if all && len(ages) > 0 {
		return d, fmt.Errorf("conflicting age text")
	}
	category := ""
	for _, a := range ages {
		if a[1] != "16" && a[1] != "18" && a[1] != "21" {
			return d, fmt.Errorf("unreviewed age text")
		}
		if category != "" && category != a[1] {
			return d, fmt.Errorf("conflicting age text")
		}
		category = a[1]
	}
	if all || category != "" {
		text := "All Ages"
		if category != "" {
			text = "Ages " + category + "+"
		}
		d.AdmissionPolicy = &artifact.AdmissionPolicy{
			Text: text, URL: "https://www.d3arts.org/calendar/",
		}
		if category != "" {
			d.AdmissionPolicy.Category = category + "+"
		} else {
			d.AdmissionPolicy.Category = "All ages"
		}
	}
	return d, nil
}
