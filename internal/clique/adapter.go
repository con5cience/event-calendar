// Package clique normalizes Red Rocks public API snapshots, without fetching.
package clique

import (
	"bytes"
	"encoding/json"
	"event-calendar/internal/artifact"
	"fmt"
	dom "golang.org/x/net/html"
	"html"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode/utf8"
)

const origin = "https://www.redrocksonline.com"

type record struct {
	ID      int
	Title   string `json:"post_title"`
	Content string `json:"post_content"`
	Status  string `json:"post_status"`
	Type    string `json:"post_type"`
	URL     string `json:"guid"`
	ACF     map[string]json.RawMessage
}
type calendarEvent struct{ ID, URL, Title, Start string }
type window struct {
	Start, End string
	Events     []calendarEvent
}
type snapshot struct {
	From, Through string
	Upcoming      []record
	Range         []calendarEvent
	Windows       []window
}

var digits = regexp.MustCompile(`^[1-9][0-9]*$`)

func field(e record, key string) (string, error) {
	b, ok := e.ACF[key]
	if !ok {
		return "", nil
	}
	var values []*string
	if json.Unmarshal(b, &values) != nil || len(values) != 1 {
		return "", fmt.Errorf("invalid %s", key)
	}
	if values[0] == nil {
		return "", nil
	}
	return strings.TrimSpace(*values[0]), nil
}
func ownURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.User == nil && u.Scheme+"://"+u.Host == origin && strings.HasPrefix(u.Path, "/events/") && len(u.Path) > 8 && u.RawQuery == "" && u.Fragment == ""
}
func Decode(cfg artifact.SourceConfig, b []byte, now time.Time) (artifact.Refresh, error) {
	fail := func(s string) (artifact.Refresh, error) { return artifact.Refresh{}, fmt.Errorf("clique: %s", s) }
	raw, err := json.Marshal(cfg)
	if err != nil {
		return fail("invalid configuration")
	}
	cfg, err = artifact.DecodeConfigJSON(raw)
	if err != nil {
		return artifact.Refresh{}, err
	}
	if cfg.Source.ID != "red-rocks" || cfg.Source.Adapter != "clique-calendar" || cfg.Venue.Key != "red-rocks" || cfg.Venue.Name != "Red Rocks Amphitheatre" || cfg.Venue.Website != origin || cfg.Venue.Timezone != "America/Denver" || len(cfg.AdapterOptions) != 0 {
		return fail("unsupported configuration")
	}
	if now.IsZero() || len(b) > artifact.MaxDocumentBytes || !utf8.Valid(b) {
		return fail("invalid clock or snapshot size/encoding")
	}
	var s snapshot
	dec := json.NewDecoder(bytes.NewReader(b))
	if dec.Decode(&s) != nil || dec.Decode(new(any)) != io.EOF || s.Upcoming == nil || s.Range == nil || len(s.Windows) == 0 {
		return fail("invalid snapshot envelope")
	}
	loc, _ := time.LoadLocation(cfg.Venue.Timezone)
	today := now.In(loc)
	if s.From != today.Format("2006-01-02") || s.Through != today.AddDate(1, 0, 0).Format("2006-01-02") {
		return fail("snapshot coverage differs from clock")
	}
	from, _ := time.Parse("2006-01-02", s.From)
	through, _ := time.Parse("2006-01-02", s.Through)
	end := through.AddDate(0, 0, 1).Format("2006-01-02")
	if len(s.Upcoming) >= 10000 || len(s.Range) >= 10000 {
		return fail("snapshot reached safety cap")
	}
	full := map[string]calendarEvent{}
	urls := map[string]bool{}
	for _, e := range s.Range {
		if !digits.MatchString(e.ID) || !ownURL(e.URL) || full[e.ID].ID != "" || urls[e.URL] {
			return fail("invalid or duplicate range identity")
		}
		full[e.ID] = e
		urls[e.URL] = true
	}
	union := map[string]calendarEvent{}
	cursor := from.Format("2006-01-02")
	for _, w := range s.Windows {
		a, ea := time.Parse("2006-01-02", w.Start)
		z, ez := time.Parse("2006-01-02", w.End)
		if ea != nil || ez != nil || !z.After(a) || w.Start != cursor || w.End > end || w.Events == nil || len(w.Events) >= 10000 {
			return fail("invalid or incomplete range windows")
		}
		seen := map[string]bool{}
		for _, e := range w.Events {
			if seen[e.ID] || full[e.ID] != e {
				return fail("range windows disagree")
			}
			seen[e.ID] = true
			if len(e.Start) < 10 || e.Start[:10] < w.Start || e.Start[:10] > w.End {
				return fail("window returned out-of-range event")
			}
			union[e.ID] = e
		}
		cursor = w.End
	}
	if cursor != end || len(union) != len(full) {
		return fail("range coverage incomplete")
	}
	r := artifact.Refresh{GeneratedAt: now.UTC().Format(time.RFC3339Nano), Coverage: artifact.Coverage{From: s.From, Through: s.Through, Complete: true}, Observations: []artifact.Observation{}}
	seen := map[string]bool{}
	matched := map[string]bool{}
	for _, e := range s.Upcoming {
		id, idErr := field(e, "showing_id")
		if idErr != nil || !digits.MatchString(id) || seen[id] || !ownURL(e.URL) {
			return fail("invalid or duplicate upcoming identity")
		}
		seen[id] = true
		re, ok := full[id]
		if !ok {
			start, err := field(e, "event_start")
			if err == nil && len(start) >= 10 && (start[:10] < s.From || start[:10] > s.Through) {
				continue
			}
			return fail("upcoming and range identities differ")
		}
		matched[id] = true
		data, err := normalize(e, re, loc)
		failure := ""
		if err != nil {
			failure = err.Error()
		}
		if failure == "" && (data.Date < s.From || data.Date > s.Through) {
			continue
		}
		raw, _ := json.Marshal(data)
		r.Observations = append(r.Observations, artifact.Observation{UpstreamID: id, Data: raw, Failure: failure})
	}
	for id, e := range full {
		if !matched[id] && (len(e.Start) < 10 || (e.Start[:10] >= s.From && e.Start[:10] <= s.Through)) {
			return fail("range event missing from upcoming")
		}
	}
	return r, nil
}

func plain(s string) string {
	tree, err := dom.Parse(strings.NewReader(s))
	if err != nil {
		return ""
	}
	var b strings.Builder
	var visit func(*dom.Node)
	visit = func(n *dom.Node) {
		if n.Type == dom.ElementNode && (n.Data == "script" || n.Data == "style") {
			return
		}
		if n.Type == dom.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}
	visit(tree)
	return strings.Join(strings.Fields(b.String()), " ")
}

var cancelled = regexp.MustCompile(`(?i)^cancelled\s*:|^canceled\s*:`)
var restriction = regexp.MustCompile(`(?i)\b(13|16|18|21)\s*\+|\bages?\s+(13|16|18|21)\s*(?:and (?:up|over)|\+|or older)`)
var ageCue = regexp.MustCompile(`(?i)\bage(?:s| restriction| limit)?\b|\byears? old\b|\b(?:under|over)\s+\d+|\b\d+\s*\+`)

func normalize(e record, re calendarEvent, loc *time.Location) (artifact.EventData, error) {
	title := strings.TrimSpace(html.UnescapeString(e.Title))
	d := artifact.EventData{Title: title, EventURL: e.URL, Status: "Scheduled"}
	bad := func(s string) (artifact.EventData, error) { return d, fmt.Errorf("clique: %s", s) }
	start, err := field(e, "event_start")
	if len(start) >= 10 {
		d.Date = start[:10]
	}
	t, te := time.Parse(time.RFC3339, re.Start)
	if err != nil || te != nil || t.In(loc).Format("2006-01-02 15:04:05") != start || t.Format("-07:00") != t.In(loc).Format("-07:00") {
		return bad("invalid or conflicting start time")
	}
	d.ShowAt = t.Format(time.RFC3339)
	d.Date = t.In(loc).Format("2006-01-02")
	if title == "" || title != strings.TrimSpace(html.UnescapeString(re.Title)) || e.URL != re.URL || e.ID <= 0 || e.Status != "publish" || e.Type != "event" {
		return bad("invalid or conflicting published event")
	}
	if cancelled.MatchString(title) {
		d.Status = "Cancelled"
		d.Title = strings.TrimSpace(cancelled.ReplaceAllString(title, ""))
	}
	doors, err := field(e, "event_doors_open")
	if err != nil {
		return bad(err.Error())
	}
	if doors != "" {
		clock, err := time.Parse("3:04 PM", strings.ToUpper(doors))
		if err != nil {
			return bad("unsupported doors time")
		}
		stamp := d.Date + " " + clock.Format("15:04:05")
		door, err := time.ParseInLocation("2006-01-02 15:04:05", stamp, loc)
		if err != nil || door.Format("2006-01-02 15:04:05") != stamp || door.After(t) {
			return bad("invalid doors time")
		}
		for _, delta := range []time.Duration{-time.Hour, time.Hour} {
			if door.Add(delta).Format("2006-01-02 15:04:05") == stamp {
				return bad("ambiguous doors time")
			}
		}
		d.DoorsAt = door.Format(time.RFC3339)
	}
	ticket, err := field(e, "event_ticket_link")
	if err != nil {
		return bad(err.Error())
	}
	if ticket != "" {
		u, err := url.Parse(ticket)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
			return bad("unsafe ticket URL")
		}
		d.TicketURL = ticket
	}
	// Subtitle can be promotional copy rather than a performer list. Do not guess.
	content := plain(e.Content)
	matches := restriction.FindAllStringSubmatch(title+" "+content, -1)
	if len(matches) > 0 {
		category := ""
		for _, m := range matches {
			age := m[1]
			if age == "" {
				age = m[2]
			}
			if category != "" && category != age+"+" {
				return bad("conflicting age restrictions")
			}
			category = age + "+"
		}
		d.AdmissionPolicy = &artifact.AdmissionPolicy{Text: category, Category: category, URL: e.URL}
		if category == "13+" || category == "16+" {
			minimum, _ := strconv.Atoi(strings.TrimSuffix(category, "+"))
			d.AdmissionPolicy.WithAdult = &artifact.AdultAdmission{URL: e.URL, ReviewedOn: "2026-09-11", Ranges: []artifact.AdmissionRange{{MinAge: minimum, MaxAge: 17, Condition: "Permitted at this age"}}}
		}
	} else if ageCue.MatchString(content) && !strings.EqualFold(strings.TrimSpace(content), "All ages") {
		// Unknown event-specific age text overrides the venue default without inventing permission.
		d.AdmissionPolicy = &artifact.AdmissionPolicy{Text: "See event age restrictions", URL: e.URL}
	}
	return d, nil
}
