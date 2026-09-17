// Package anteup reads Ante Up's public Wix events payload and venue page.
package anteup

import (
	"bytes"
	"encoding/json"
	"event-calendar/internal/artifact"
	"fmt"
	dom "golang.org/x/net/html"
	"io"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode/utf8"
)

const (
	calendarURL = "https://www.anteupdenver.com/events-1"
	policyURL   = "https://www.anteupdenver.com/venue"
	origin      = "https://www.anteupdenver.com"
	eventsApp   = "140603ad-af8d-84a5-2c80-a0f60cb47351"
	policyText  = "All ages; sober community space"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var doorsPattern = regexp.MustCompile(`(?i)(?:^|\n)\s*DOORS at\s+(\d{1,2}:\d{2}\s*[AP]M)\b`)
var showAtPattern = regexp.MustCompile(`(?i)(?:^|\n)\s*SHOW at\s+(\d{1,2}:\d{2}\s*[AP]M)\b`)
var showPattern = regexp.MustCompile(`(?i)(?:^|\n)\s*SHOW\s+(\d{1,2}:\d{2}\s*[AP]M)\b`)
var restriction = regexp.MustCompile(`(?i)\b\d{1,2}\s*\+|\badults?\s+only\b|\bage\s+restricted\b`)
var cancelled = regexp.MustCompile(`(?i)\b(?:cancelled|canceled)\b`)
var spaces = regexp.MustCompile(`\s+`)

type pages struct{ Warmup, Policy string }
type record struct {
	ID      string
	Data    artifact.EventData
	Failure string
}
type wixEvent struct {
	ID, Title, Slug, Description string
	Status                       float64
	Location                     struct{ Name, Address string }
	Scheduling                   struct {
		Config struct {
			ScheduleTbd bool
			StartDate   string
			TimeZoneID  string `json:"timeZoneId"`
		}
		StartDateFormatted string
		StartTimeFormatted string
	}
	Registration struct {
		Ticketing json.RawMessage
	}
}
type wixDate struct {
	StartISO  string `json:"startDateISOFormatNotUTC"`
	StartTime string `json:"startTime"`
	UTCOffset int    `json:"utcOffset"`
}
type widget struct {
	Events struct {
		Events      []wixEvent
		HasMore     *bool
		MoreLoading *bool
	}
	Dates struct {
		Events map[string]wixDate
	}
}

func Decode(cfg artifact.SourceConfig, b []byte, now time.Time) (artifact.Refresh, error) {
	fail := func(s string) (artifact.Refresh, error) {
		return artifact.Refresh{}, fmt.Errorf("anteup: %s", s)
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
	rule, reviewed := cfg.AdmissionRules["All ages"]
	if cfg.Source.ID != "ante-up" || cfg.Source.Adapter != "html-anteup" || cfg.Venue.Key != "ante-up" || cfg.Venue.Name != "Ante Up!" || cfg.Venue.Timezone != "America/Denver" || cfg.Venue.Website != calendarURL || cfg.Venue.Address != "2130 S Platte River Dr, Denver, CO 80223" || len(cfg.AdapterOptions) != 0 || len(cfg.AdmissionRules) != 1 || p == nil || p.Text != policyText || p.Category != "All ages" || p.URL != policyURL || p.WithAdult != nil || !reviewed || rule.URL != policyURL || rule.ReviewedOn != "2026-09-17" || len(rule.Ranges) != 1 || rule.Ranges[0].MinAge != 0 || rule.Ranges[0].MaxAge != 17 || rule.Ranges[0].Condition != "Permitted at this age; sober space" {
		return fail("unsupported configuration")
	}
	if now.IsZero() || len(b) > artifact.MaxDocumentBytes || !utf8.Valid(b) {
		return fail("invalid snapshot")
	}
	var s struct {
		From, Through string
		Pages, Check  pages
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
	rows, err := parse(s.Pages, loc)
	if err != nil {
		return fail(err.Error())
	}
	check, err := parse(s.Check, loc)
	if err != nil || !reflect.DeepEqual(rows, check) {
		return fail("incomplete or changed capture")
	}
	out := artifact.Refresh{GeneratedAt: now.UTC().Format(time.RFC3339Nano), Coverage: artifact.Coverage{From: s.From, Through: s.Through, Complete: true}, Observations: []artifact.Observation{}}
	for _, r := range rows {
		if r.Failure == "" && (r.Data.Date < s.From || r.Data.Date > s.Through) {
			continue
		}
		raw, _ := json.Marshal(r.Data)
		out.Observations = append(out.Observations, artifact.Observation{UpstreamID: r.ID, Data: raw, Failure: r.Failure})
	}
	return out, nil
}

func text(n *dom.Node) string {
	if n.Type == dom.TextNode {
		return n.Data
	}
	if n.Data == "script" || n.Data == "style" {
		return ""
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(text(c))
		b.WriteByte(' ')
	}
	return b.String()
}

func parse(p pages, loc *time.Location) ([]record, error) {
	root, err := dom.Parse(strings.NewReader(p.Policy))
	if err != nil {
		return nil, err
	}
	policy := strings.ToLower(spaces.ReplaceAllString(text(root), " "))
	if !strings.Contains(policy, "people of all ages") || !strings.Contains(policy, "sober") || !strings.Contains(policy, "2130") || !strings.Contains(policy, "platte river") {
		return nil, fmt.Errorf("unreviewed venue policy")
	}
	var warm struct {
		Apps map[string]map[string]json.RawMessage `json:"appsWarmupData"`
	}
	dec := json.NewDecoder(strings.NewReader(p.Warmup))
	if dec.Decode(&warm) != nil || dec.Decode(new(any)) != io.EOF || len(warm.Apps[eventsApp]) != 1 {
		return nil, fmt.Errorf("unknown events payload")
	}
	var raw json.RawMessage
	for _, v := range warm.Apps[eventsApp] {
		raw = v
	}
	var w widget
	if json.Unmarshal(raw, &w) != nil || w.Events.HasMore == nil || *w.Events.HasMore || w.Events.MoreLoading == nil || *w.Events.MoreLoading {
		return nil, fmt.Errorf("incomplete events listing")
	}
	if len(w.Events.Events) == 0 || len(w.Events.Events) > 500 {
		return nil, fmt.Errorf("unknown empty calendar or capture cap exceeded")
	}
	seen := map[string]bool{}
	rows := []record{}
	for _, ev := range w.Events.Events {
		if ev.ID == "" || seen[ev.ID] {
			return nil, fmt.Errorf("invalid or duplicate identity")
		}
		seen[ev.ID] = true
		r := record{ID: ev.ID}
		data, ferr := normalize(ev, w.Dates.Events[ev.ID], loc)
		r.Data = data
		if ferr != nil {
			r.Failure = ferr.Error()
		}
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows, nil
}

func normalize(ev wixEvent, when wixDate, loc *time.Location) (artifact.EventData, error) {
	d := artifact.EventData{Status: "Scheduled", Title: strings.TrimSpace(ev.Title)}
	if d.Title == "" || !slugPattern.MatchString(ev.Slug) {
		return d, fmt.Errorf("invalid title or event link")
	}
	d.EventURL = origin + "/events/" + ev.Slug
	if !strings.EqualFold(ev.Location.Name, "ANTE UP") || !strings.Contains(ev.Location.Address, "2130") || !strings.Contains(strings.ToLower(ev.Location.Address), "platte river") || !strings.Contains(ev.Location.Address, "Denver") {
		return d, fmt.Errorf("unreviewed venue")
	}
	if ev.Scheduling.Config.TimeZoneID != "America/Denver" || ev.Scheduling.Config.ScheduleTbd {
		return d, fmt.Errorf("unreviewed schedule")
	}
	start, err := time.Parse(time.RFC3339Nano, ev.Scheduling.Config.StartDate)
	if err != nil {
		return d, fmt.Errorf("invalid start")
	}
	local := start.In(loc).Truncate(time.Minute)
	printed, err := time.Parse(time.RFC3339, when.StartISO)
	if err != nil || !printed.In(loc).Truncate(time.Minute).Equal(local) {
		return d, fmt.Errorf("conflicting start")
	}
	_, offset := local.Zone()
	clock := local.Format("3:04 PM")
	if offset/60 != when.UTCOffset || when.StartTime != clock || spaces.ReplaceAllString(ev.Scheduling.StartTimeFormatted, " ") != clock {
		return d, fmt.Errorf("conflicting clock")
	}
	day, err := time.Parse("January 2, 2006", spaces.ReplaceAllString(ev.Scheduling.StartDateFormatted, " "))
	if err != nil || day.Format("2006-01-02") != local.Format("2006-01-02") {
		return d, fmt.Errorf("conflicting date")
	}
	wall := local.Format("2006-01-02T15:04")
	if local.Format("2006-01-02T15:04") != wall || local.Add(time.Hour).Format("2006-01-02T15:04") == wall || local.Add(-time.Hour).Format("2006-01-02T15:04") == wall {
		return d, fmt.Errorf("invalid or ambiguous Denver clock")
	}
	d.Date = local.Format("2006-01-02")
	if doors := doorsPattern.FindAllStringSubmatch(ev.Description, -1); len(doors) > 1 {
		return d, fmt.Errorf("conflicting doors")
	} else if len(doors) == 1 {
		at, e := clockOn(d.Date, doors[0][1], loc)
		if e != nil || at.Format("15:04") != local.Format("15:04") {
			return d, fmt.Errorf("conflicting doors")
		}
		d.DoorsAt = at.Format(time.RFC3339)
	}
	shows := showAtPattern.FindAllStringSubmatch(ev.Description, -1)
	if len(shows) == 0 {
		shows = showPattern.FindAllStringSubmatch(ev.Description, -1)
	}
	if len(shows) > 1 {
		return d, fmt.Errorf("conflicting show time")
	}
	if len(shows) == 1 {
		at, e := clockOn(d.Date, shows[0][1], loc)
		if e != nil || (d.DoorsAt != "" && at.Format(time.RFC3339) < d.DoorsAt) {
			return d, fmt.Errorf("conflicting show time")
		}
		d.ShowAt = at.Format(time.RFC3339)
	}
	if ev.Status != 0 {
		return d, fmt.Errorf("unreviewed event status")
	}
	if cancelled.MatchString(d.Title) {
		d.Status = "Cancelled"
		d.AdmissionPolicy = &artifact.AdmissionPolicy{Text: "Cancelled; check venue", URL: d.EventURL}
	} else if m := restriction.FindString(ev.Description); m != "" {
		if regexp.MustCompile(`(?i)\ball ages\b`).MatchString(ev.Description) {
			return d, fmt.Errorf("conflicting age text")
		}
		d.AdmissionPolicy = &artifact.AdmissionPolicy{Text: strings.TrimSpace(m), URL: d.EventURL}
	} else if len(ev.Registration.Ticketing) > 0 && string(ev.Registration.Ticketing) != "null" {
		d.TicketURL = d.EventURL
	}
	return d, nil
}

func clockOn(date, raw string, loc *time.Location) (time.Time, error) {
	clock, err := time.Parse("3:04 PM", spaces.ReplaceAllString(strings.ToUpper(raw), " "))
	if err != nil {
		return time.Time{}, err
	}
	wall := date + "T" + clock.Format("15:04")
	at, err := time.ParseInLocation("2006-01-02T15:04", wall, loc)
	if err != nil || at.Format("2006-01-02T15:04") != wall || at.Add(time.Hour).Format("2006-01-02T15:04") == wall || at.Add(-time.Hour).Format("2006-01-02T15:04") == wall {
		return time.Time{}, fmt.Errorf("invalid or ambiguous Denver clock")
	}
	return at, nil
}
