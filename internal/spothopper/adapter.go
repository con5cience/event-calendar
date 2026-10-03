// Package spothopper reads a venue's public SpotHopper events page. It never
// fetches URLs; the operator capture supplies paired identical page reads.
package spothopper

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

const eventsURL = "https://blackskydenver.com/denver-santa-fe-arts-district-black-sky-brewery-events"

var (
	sectionID  = regexp.MustCompile(`^[1-9][0-9]{0,9}$`)
	dayPattern = regexp.MustCompile(`(?i)^(monday|tuesday|wednesday|thursday|friday|saturday|sunday)\s+(january|february|march|april|may|june|july|august|september|october|november|december)\s+([1-9]|[12][0-9]|3[01])(?:st|nd|rd|th)?$`)
	windowPart = regexp.MustCompile(`^\s*(\d{1,2}:\d{2}\s*[AP]M)\s*-\s*(\d{1,2}:\d{2}\s*[AP]M)\s*$`)
	doorsAt    = regexp.MustCompile(`(?i)\bdoors?\s+open\s+at\s+(\d{1,2})(?::(\d{2}))?\s*(a|p)\.?m\.?`)
	showsAt    = regexp.MustCompile(`(?i)\b(?:show|bands?)\s+starts?\s+at\s+(\d{1,2})(?::(\d{2}))?\s*(a|p)\.?m\.?`)
	allAges    = regexp.MustCompile(`(?i)\ball ages\b`)
	agePlus    = regexp.MustCompile(`(?i)\bages?\s+(\d{1,2})\s*\+`)
	ticketLink = regexp.MustCompile(`https://(?:tickets\.)?holdmyticket\.com/tickets/([1-9][0-9]*)\b`)
	spaces     = regexp.MustCompile(`\s+`)
)

type record struct {
	ID      string
	Data    artifact.EventData
	Failure string
}
type section struct {
	ID, Title, Day, Window, Description string
	Recurring                           bool
}

func Decode(cfg artifact.SourceConfig, b []byte, now time.Time) (artifact.Refresh, error) {
	fail := func(s string) (artifact.Refresh, error) {
		return artifact.Refresh{}, fmt.Errorf("spothopper: %s", s)
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return fail("configuration")
	}
	cfg, err = artifact.DecodeConfigJSON(raw)
	if err != nil {
		return fail("configuration")
	}
	if cfg.Source.ID != "black-sky" || cfg.Source.Adapter != "spothopper-events" ||
		cfg.Venue.Key != "black-sky" || cfg.Venue.Name != "Black Sky Brewery" ||
		cfg.Venue.Timezone != "America/Denver" || cfg.Venue.Website != eventsURL ||
		cfg.Venue.Address != "490 Santa Fe Drive, Denver, CO 80204" ||
		cfg.Venue.Phone != "(720) 708-5816" || cfg.Venue.AdmissionPolicy != nil ||
		len(cfg.AdapterOptions) != 1 || cfg.AdapterOptions["recurring"] != "exclude" ||
		len(cfg.AdmissionRules) != 0 || len(cfg.Overrides) != 0 {
		return fail("unsupported configuration")
	}
	if now.IsZero() || len(b) > artifact.MaxDocumentBytes || !utf8.Valid(b) {
		return fail("invalid snapshot")
	}
	var s struct {
		From, Through string
		Page, Check   string
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
	rows, err := parse(s.Page, loc, today)
	if err != nil {
		return fail(err.Error())
	}
	check, err := parse(s.Check, loc, today)
	if err != nil || !reflect.DeepEqual(rows, check) {
		return fail("incomplete or changed capture")
	}
	out := artifact.Refresh{
		GeneratedAt:  now.UTC().Format(time.RFC3339Nano),
		Coverage:     artifact.Coverage{From: s.From, Through: s.Through, Complete: true},
		Observations: []artifact.Observation{},
	}
	for _, r := range rows {
		if r.Failure == "" && (r.Data.Date < s.From || r.Data.Date > s.Through) {
			continue
		}
		raw, _ := json.Marshal(r.Data)
		out.Observations = append(out.Observations, artifact.Observation{
			UpstreamID: r.ID, Data: raw, Failure: r.Failure,
		})
	}
	return out, nil
}

func attribute(n *dom.Node, key string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func hasClass(n *dom.Node, class string) bool {
	for _, name := range strings.Fields(attribute(n, "class")) {
		if name == class {
			return true
		}
	}
	return false
}
func walk(n *dom.Node, visit func(*dom.Node)) {
	if n == nil {
		return
	}
	visit(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, visit)
	}
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

// The page is the venue's whole public interface: every announced event is a
// section carrying the platform's own event id and recurring-series marker.
func parseSections(page string) ([]section, error) {
	root, err := dom.Parse(strings.NewReader(page))
	if err != nil {
		return nil, err
	}
	found := []section{}
	failure := ""
	var current *section
	var info *dom.Node
	identity := ""
	var titles, days, windows int
	seen := map[string]bool{}
	reset := func() {
		current, info, identity = nil, nil, ""
		titles, days, windows = 0, 0, 0
	}
	finish := func() {
		if failure != "" || current == nil {
			return
		}
		if titles != 1 || days != 1 || windows != 1 || info == nil {
			failure = "unreviewed event section"
			return
		}
		if identity != current.ID {
			failure = "conflicting event identity"
			return
		}
		if !sectionID.MatchString(current.ID) || seen[current.ID] {
			failure = "invalid or duplicate event identity"
			return
		}
		seen[current.ID] = true
		current.Description = strings.TrimSpace(
			spaces.ReplaceAllString(strings.TrimSpace(text(info)), " "))
		found = append(found, *current)
	}
	walk(root, func(n *dom.Node) {
		if failure != "" || n.Type != dom.ElementNode {
			return
		}
		if n.Data == "section" {
			finish()
			reset()
			id := attribute(n, "id")
			if id == "" || !sectionID.MatchString(id) {
				return
			}
			current = &section{ID: id}
			return
		}
		if current == nil {
			return
		}
		for _, a := range n.Attr {
			switch a.Key {
			case "data-event-id":
				if identity != "" && identity != a.Val {
					failure = "conflicting event identity"
					return
				}
				identity = a.Val
			case "data-is-recurring":
				if a.Val == "true" {
					current.Recurring = true
				}
			}
		}
		switch {
		case n.Data == "h2":
			titles++
			current.Title = strings.TrimSpace(text(n))
		case n.Data == "p" && hasClass(n, "event-day"):
			days++
			current.Day = strings.TrimSpace(text(n))
		case n.Data == "p" && hasClass(n, "event-time"):
			windows++
			current.Window = strings.TrimSpace(text(n))
		case n.Data == "div" && hasClass(n, "event-info-text"):
			info = n
		}
	})
	finish()
	if failure != "" {
		return nil, fmt.Errorf("%s", failure)
	}
	if len(found) == 0 || len(found) > 200 {
		return nil, fmt.Errorf("missing or capped event listing")
	}
	return found, nil
}

var months = map[string]time.Month{
	"january": time.January, "february": time.February, "march": time.March,
	"april": time.April, "may": time.May, "june": time.June, "july": time.July,
	"august": time.August, "september": time.September, "october": time.October,
	"november": time.November, "december": time.December,
}
var weekdays = map[string]time.Weekday{
	"sunday": time.Sunday, "monday": time.Monday, "tuesday": time.Tuesday,
	"wednesday": time.Wednesday, "thursday": time.Thursday,
	"friday": time.Friday, "saturday": time.Saturday,
}

// The page prints weekday and month without a year. Dates are read in order,
// the year starts at the capture year and rolls over when the month goes
// backward, and the printed weekday cross-checks the inferred year.
func dayDate(day string, year *int, previous time.Month) (time.Time, error) {
	m := dayPattern.FindStringSubmatch(strings.TrimSpace(day))
	if m == nil {
		return time.Time{}, fmt.Errorf("unreviewed event date")
	}
	month := months[strings.ToLower(m[2])]
	if month < previous {
		*year++
	}
	var dayNumber int
	fmt.Sscanf(m[3], "%d", &dayNumber)
	date := time.Date(*year, month, dayNumber, 0, 0, 0, 0, time.UTC)
	if date.Month() != month || int(date.Day()) != dayNumber {
		return time.Time{}, fmt.Errorf("unreviewed event date")
	}
	if weekdays[strings.ToLower(m[1])] != date.Weekday() {
		return time.Time{}, fmt.Errorf("conflicting event date")
	}
	return date, nil
}

func clockOn(date time.Time, raw string, loc *time.Location) (time.Time, error) {
	clock, err := time.Parse("3:04 PM", strings.ToUpper(spaces.ReplaceAllString(raw, " ")))
	if err != nil {
		return time.Time{}, err
	}
	wall := date.Format("2006-01-02") + "T" + clock.Format("15:04")
	at, err := time.ParseInLocation("2006-01-02T15:04", wall, loc)
	if err != nil || at.Format("2006-01-02T15:04") != wall ||
		at.Add(time.Hour).Format("2006-01-02T15:04") == wall ||
		at.Add(-time.Hour).Format("2006-01-02T15:04") == wall {
		return time.Time{}, fmt.Errorf("invalid or ambiguous Denver clock")
	}
	return at, nil
}

func clockText(m []string) string {
	minutes := m[2]
	if minutes == "" {
		minutes = "00"
	}
	meridiem := "AM"
	if strings.EqualFold(m[3], "p") {
		meridiem = "PM"
	}
	return m[1] + ":" + minutes + " " + meridiem
}

// The platform's window is the venue's own published event window: a listed
// show starts with it unless the description states its own show or doors
// time. Door charges and band set lists are deliberately left unparsed.
func normalize(s section, date time.Time, loc *time.Location) (artifact.EventData, error) {
	d := artifact.EventData{Status: "Scheduled", EventURL: eventsURL}
	d.Title = s.Title
	if d.Title == "" {
		return d, fmt.Errorf("missing title")
	}
	w := windowPart.FindStringSubmatch(s.Window)
	if w == nil {
		return d, fmt.Errorf("unreviewed event window")
	}
	start, err := clockOn(date, w[1], loc)
	if err != nil {
		return d, fmt.Errorf("unreviewed event window")
	}
	endClock, err := time.Parse("3:04 PM", strings.ToUpper(spaces.ReplaceAllString(w[2], " ")))
	if err != nil {
		return d, fmt.Errorf("unreviewed event window")
	}
	endWall := date.Format("2006-01-02") + "T" + endClock.Format("15:04")
	if endWall <= start.Format("2006-01-02T15:04") {
		endWall = date.AddDate(0, 0, 1).Format("2006-01-02") + "T" + endClock.Format("15:04")
	}
	end, err := time.ParseInLocation("2006-01-02T15:04", endWall, loc)
	if err != nil || !end.After(start) ||
		end.Add(time.Hour).Format("2006-01-02T15:04") == endWall ||
		end.Add(-time.Hour).Format("2006-01-02T15:04") == endWall {
		return d, fmt.Errorf("invalid or ambiguous Denver clock")
	}
	d.Date = start.Format("2006-01-02")
	d.ShowAt = start.Format(time.RFC3339)
	if shows := showsAt.FindAllStringSubmatch(s.Description, -1); len(shows) > 1 {
		return d, fmt.Errorf("conflicting show time")
	} else if len(shows) == 1 {
		at, err := clockOn(date, clockText(shows[0]), loc)
		if err != nil || at.Before(start) || at.After(end) {
			return d, fmt.Errorf("conflicting show time")
		}
		d.ShowAt = at.Format(time.RFC3339)
	}
	if doors := doorsAt.FindAllStringSubmatch(s.Description, -1); len(doors) > 1 {
		return d, fmt.Errorf("conflicting doors time")
	} else if len(doors) == 1 {
		at, err := clockOn(date, clockText(doors[0]), loc)
		if err != nil || at.Format(time.RFC3339) > d.ShowAt {
			return d, fmt.Errorf("conflicting doors time")
		}
		d.DoorsAt = at.Format(time.RFC3339)
	}
	all := allAges.MatchString(s.Description)
	ages := agePlus.FindAllStringSubmatch(s.Description, -1)
	if all && len(ages) > 0 {
		return d, fmt.Errorf("conflicting age text")
	}
	if all {
		d.AdmissionPolicy = &artifact.AdmissionPolicy{
			Text: "All Ages", Category: "All ages", URL: eventsURL,
		}
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
	if category != "" {
		d.AdmissionPolicy = &artifact.AdmissionPolicy{
			Text: "Ages " + category + "+", Category: category + "+", URL: eventsURL,
		}
	}
	links := map[string]bool{}
	for _, m := range ticketLink.FindAllStringSubmatch(s.Description, -1) {
		links["https://tickets.holdmyticket.com/tickets/"+m[1]] = true
	}
	if len(links) > 1 {
		return d, fmt.Errorf("conflicting ticket link")
	}
	for link := range links {
		d.TicketURL = link
	}
	return d, nil
}

func parse(page string, loc *time.Location, today time.Time) ([]record, error) {
	found, err := parseSections(page)
	if err != nil {
		return nil, err
	}
	year := today.Year()
	previous := time.Month(0)
	var last time.Time
	rows := []record{}
	for _, s := range found {
		date, err := dayDate(s.Day, &year, previous)
		if err != nil {
			return nil, err
		}
		previous = date.Month()
		if !last.IsZero() && date.Before(last) {
			return nil, fmt.Errorf("unsorted event listing")
		}
		last = date
		r := record{ID: s.ID}
		data, ferr := normalize(s, date, loc)
		r.Data = data
		if s.Recurring {
			r.Failure = "recurring series excluded"
		} else if ferr != nil {
			r.Failure = ferr.Error()
		}
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows, nil
}
