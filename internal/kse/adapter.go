// Package kse normalizes the public KSE venue-events array used by Paramount.
package kse

import (
	"bytes"
	"encoding/json"
	"event-calendar/internal/artifact"
	"fmt"
	"html"
	"io"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode/utf8"
)

type start struct {
	LocalDate, LocalTime, DateTime            string
	DateTBD, DateTBA, TimeTBA, NoSpecificTime bool
}
type record struct {
	ID, Name, Type, URL, Info, PleaseNote string
	Test                                  bool
	Dates                                 struct {
		Start            start
		Timezone         string
		Status           struct{ Code string }
		SpanMultipleDays bool
	}
	DoorsTimes    *start
	CalendarStart string `json:"calendar_start_datetime"`
	Embedded      struct {
		Venues      []struct{ ID, Name string }
		Attractions []struct{ Name string }
	} `json:"_embedded"`
}

func Decode(cfg artifact.SourceConfig, b []byte, now time.Time) (artifact.Refresh, error) {
	fail := func(s string) (artifact.Refresh, error) { return artifact.Refresh{}, fmt.Errorf("kse: %s", s) }
	raw, err := json.Marshal(cfg)
	if err != nil {
		return fail("invalid config")
	}
	cfg, err = artifact.DecodeConfigJSON(raw)
	if err != nil {
		return artifact.Refresh{}, err
	}
	if cfg.Source.ID != "paramount" || cfg.Source.Adapter != "kse-venue-events" || cfg.Venue.Key != "paramount" || cfg.Venue.Name != "Paramount Theatre" || cfg.Venue.Timezone != "America/Denver" || cfg.Venue.Website != "https://www.paramountdenver.com" || len(cfg.AdapterOptions) != 0 {
		return fail("unsupported configuration")
	}
	if now.IsZero() || len(b) > artifact.MaxDocumentBytes || !utf8.Valid(b) {
		return fail("invalid snapshot or clock")
	}
	var s struct{ Events, Check []json.RawMessage }
	dec := json.NewDecoder(bytes.NewReader(b))
	if dec.Decode(&s) != nil || dec.Decode(new(any)) != io.EOF || s.Events == nil || s.Check == nil || len(s.Events) >= 1000 {
		return fail("invalid snapshot envelope")
	}
	p, _ := json.Marshal(s.Events)
	q, _ := json.Marshal(s.Check)
	var a, z any
	if json.Unmarshal(p, &a) != nil || json.Unmarshal(q, &z) != nil || !reflect.DeepEqual(a, z) {
		return fail("venue array changed")
	}
	loc, _ := time.LoadLocation(cfg.Venue.Timezone)
	today := now.In(loc)
	r := artifact.Refresh{GeneratedAt: now.UTC().Format(time.RFC3339Nano), Coverage: artifact.Coverage{From: today.Format("2006-01-02"), Through: today.AddDate(1, 0, 0).Format("2006-01-02"), Complete: true}, Observations: []artifact.Observation{}}
	seen := map[string]bool{}
	for _, raw := range s.Events {
		var id struct{ ID string }
		if json.Unmarshal(raw, &id) != nil || id.ID == "" || seen[id.ID] {
			return fail("invalid or duplicate identity")
		}
		seen[id.ID] = true
		data, err := normalize(raw, loc, "2026-09-11")
		failure := ""
		if err != nil {
			failure = err.Error()
		}
		if failure == "" && (data.Date < r.Coverage.From || data.Date > r.Coverage.Through) {
			continue
		}
		value, _ := json.Marshal(data)
		r.Observations = append(r.Observations, artifact.Observation{UpstreamID: id.ID, Data: value, Failure: failure})
	}
	return r, nil
}

func normalize(raw []byte, loc *time.Location, reviewed string) (artifact.EventData, error) {
	var e record
	d := artifact.EventData{Status: "Scheduled"}
	if json.Unmarshal(raw, &e) != nil {
		return d, fmt.Errorf("invalid record fields")
	}
	d.Title = strings.TrimSpace(html.UnescapeString(e.Name))
	if e.Test || e.Type != "event" || len(e.Embedded.Venues) != 1 || e.Embedded.Venues[0].ID != "KovZpZAFa1nA" || e.Embedded.Venues[0].Name != "Paramount Theatre" {
		return d, fmt.Errorf("not a Paramount event")
	}
	u, err := url.Parse(e.URL)
	if err != nil || u.User != nil || u.Scheme != "https" || u.Host != "www.ticketmaster.com" || !strings.Contains(u.Path, "/event/") {
		return d, fmt.Errorf("invalid official event link")
	}
	d.EventURL = e.URL
	d.TicketURL = e.URL
	s := e.Dates.Start
	if e.Dates.Timezone != "America/Denver" || s.DateTBD || s.DateTBA || e.Dates.SpanMultipleDays || len(s.LocalDate) < 10 {
		return d, fmt.Errorf("unsupported or unknown date")
	}
	d.Date = s.LocalDate[:10]
	date, err := time.Parse("2006-01-02", d.Date)
	if err != nil || date.Format("2006-01-02") != d.Date {
		return d, fmt.Errorf("invalid date")
	}
	if !s.TimeTBA && !s.NoSpecificTime {
		at, err := time.Parse(time.RFC3339, s.DateTime)
		if err != nil || at.In(loc).Format("2006-01-02") != d.Date {
			return d, fmt.Errorf("invalid or inconsistent start instant")
		}
		local := at.In(loc)
		if e.CalendarStart != "" && e.CalendarStart != local.Format("2006-01-02T15:04:05") {
			return d, fmt.Errorf("calendar/start mismatch")
		}
		d.ShowAt = local.Format(time.RFC3339)
	}
	if e.DoorsTimes != nil && e.DoorsTimes.DateTime != "" {
		at, err := time.Parse(time.RFC3339, e.DoorsTimes.DateTime)
		if err != nil || at.In(loc).Format("2006-01-02") != d.Date {
			return d, fmt.Errorf("invalid doors instant")
		}
		if d.ShowAt != "" {
			show, _ := time.Parse(time.RFC3339, d.ShowAt)
			if at.After(show) {
				return d, fmt.Errorf("doors follow show")
			}
		}
		d.DoorsAt = at.In(loc).Format(time.RFC3339)
	}
	switch e.Dates.Status.Code {
	case "onsale", "offsale", "rescheduled", "postponed":
	case "cancelled", "canceled":
		d.Status = "Cancelled"
	default:
		return d, fmt.Errorf("unknown status")
	}
	for _, p := range e.Embedded.Attractions {
		if name := strings.TrimSpace(html.UnescapeString(p.Name)); name != "" {
			d.Performers = append(d.Performers, name)
		}
	}
	policy, err := admission(e.Info+" "+e.PleaseNote, e.URL, reviewed)
	if err != nil {
		return d, err
	}
	d.AdmissionPolicy = policy
	return d, nil
}

var strictAge = regexp.MustCompile(`(?i)\b(?:ages?|age limit\s*[-:]?|this (?:event|show) is)\s*(\d{1,2})\s*\+`)
var recommended = regexp.MustCompile(`(?i)\brecommended\s+(?:for|age)\s+\d{1,2}\s*\+`)

func admission(text, eventURL, reviewed string) (*artifact.AdmissionPolicy, error) {
	text = html.UnescapeString(text)
	adult := func(ranges []artifact.AdmissionRange) *artifact.AdultAdmission {
		return &artifact.AdultAdmission{URL: eventURL, ReviewedOn: reviewed, Ranges: ranges}
	}
	if strings.Contains(strings.ToLower(text), "under 14 must be accompanied by a person aged 18+") {
		if strictAge.MatchString(text) {
			return nil, fmt.Errorf("conflicting accompaniment and minimum-age statements")
		}
		return &artifact.AdmissionPolicy{Text: "Under 14 must be accompanied by a person aged 18+", URL: eventURL, WithAdult: adult([]artifact.AdmissionRange{{MinAge: 0, MaxAge: 13, Condition: "Accompanying person aged 18+ required"}, {MinAge: 14, MaxAge: 17, Condition: "Ticket required"}})}, nil
	}
	if rec := recommended.FindString(text); rec != "" {
		return &artifact.AdmissionPolicy{Text: rec, URL: eventURL}, nil
	}
	matches := strictAge.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil, nil
	}
	age, _ := strconv.Atoi(matches[0][1])
	for _, m := range matches {
		n, _ := strconv.Atoi(m[1])
		if n != age {
			return nil, fmt.Errorf("conflicting age restrictions")
		}
	}
	p := &artifact.AdmissionPolicy{Text: strconv.Itoa(age) + "+", URL: eventURL}
	if age == 13 || age == 16 || age == 18 || age == 21 {
		p.Category = p.Text
	}
	if age <= 17 {
		p.WithAdult = adult([]artifact.AdmissionRange{{MinAge: age, MaxAge: 17, Condition: "Meets event minimum age; ticket required"}})
	}
	return p, nil
}
