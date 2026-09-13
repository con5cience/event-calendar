// Package venuepilot normalizes paired public VenuePilot widget captures.
package venuepilot

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

	dom "golang.org/x/net/html"
)

type record struct {
	ID                                                               int64
	Name, Date, DoorTime, StartTime, Status, Description, TicketsURL string
	MinimumAge                                                       *int
	Venue                                                            struct{ Name string }
}

func Decode(cfg artifact.SourceConfig, b []byte, now time.Time) (artifact.Refresh, error) {
	fail := func(s string) (artifact.Refresh, error) { return artifact.Refresh{}, fmt.Errorf("venuepilot: %s", s) }
	raw, err := json.Marshal(cfg)
	if err != nil {
		return fail("invalid configuration")
	}
	cfg, err = artifact.DecodeConfigJSON(raw)
	if err != nil {
		return artifact.Refresh{}, err
	}
	if cfg.Source.ID != "levitt" || cfg.Source.Adapter != "venuepilot" || cfg.Venue.Key != "levitt" || cfg.Venue.Name != "Levitt Pavilion Denver" || cfg.Venue.Website != "https://www.levittdenver.org" || cfg.Venue.Timezone != "America/Denver" || len(cfg.AdapterOptions) != 0 {
		return fail("unsupported configuration")
	}
	if now.IsZero() || len(b) > artifact.MaxDocumentBytes || !utf8.Valid(b) {
		return fail("invalid snapshot")
	}
	var s struct {
		From, Through string
		Total         *int
		Events, Check []json.RawMessage
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	if dec.Decode(&s) != nil || dec.Decode(new(any)) != io.EOF || s.Total == nil || s.Events == nil || s.Check == nil || len(s.Events) > 500 || *s.Total != len(s.Events) {
		return fail("invalid envelope")
	}
	var left, right any
	p, _ := json.Marshal(s.Events)
	q, _ := json.Marshal(s.Check)
	json.Unmarshal(p, &left)
	json.Unmarshal(q, &right)
	if !reflect.DeepEqual(left, right) {
		return fail("capture changed")
	}
	loc, _ := time.LoadLocation(cfg.Venue.Timezone)
	today := now.In(loc)
	if s.From != today.Format("2006-01-02") || s.Through != today.AddDate(1, 0, 0).Format("2006-01-02") {
		return fail("coverage mismatch")
	}
	out := artifact.Refresh{GeneratedAt: now.UTC().Format(time.RFC3339Nano), Coverage: artifact.Coverage{From: s.From, Through: s.Through, Complete: true}, Observations: []artifact.Observation{}}
	seen := map[int64]bool{}
	for _, raw := range s.Events {
		var id struct{ ID int64 }
		if json.Unmarshal(raw, &id) != nil || id.ID <= 0 || id.ID > 9007199254740991 || seen[id.ID] {
			return fail("invalid identity")
		}
		seen[id.ID] = true
		data, err := normalize(raw, loc)
		if err == nil && (data.Date < s.From || data.Date > s.Through) {
			return fail("record outside query coverage")
		}
		failure := ""
		if err != nil {
			failure = err.Error()
		}
		b, _ := json.Marshal(data)
		out.Observations = append(out.Observations, artifact.Observation{UpstreamID: strconv.FormatInt(id.ID, 10), Data: b, Failure: failure})
	}
	return out, nil
}

func clean(s string) string { return strings.Join(strings.Fields(html.UnescapeString(s)), " ") }

// Same inert DOM text traversal used by the HTML adapters; no markup is published.
func text(n *dom.Node) string {
	if n.Type == dom.TextNode {
		return n.Data
	}
	if n.Type == dom.ElementNode && (n.Data == "script" || n.Data == "style") {
		return ""
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(text(c))
	}
	if n.Data == "p" || n.Data == "div" || n.Data == "br" {
		b.WriteByte('\n')
	}
	return b.String()
}

var allAges = regexp.MustCompile(`(?im)^\s*all[ -]ages\s*(?:[|.!]|$)`)
var restricted = regexp.MustCompile(`(?i)\b(?:guests|patrons|attendees)\s+must\s+be\s+(13|16|18|21)\+`)

func normalize(raw []byte, loc *time.Location) (artifact.EventData, error) {
	var e record
	d := artifact.EventData{Status: "Scheduled"}
	if json.Unmarshal(raw, &e) != nil {
		return d, fmt.Errorf("invalid fields")
	}
	if e.Venue.Name != "Levitt Pavilion Denver" {
		return d, fmt.Errorf("unreviewed venue")
	}
	switch strings.ToUpper(strings.TrimSpace(e.Status)) {
	case "FREE RSVP", "FREE RSVP/UPGRADE", "TICKETS":
	case "CANCELLED", "CANCELED":
		d.Status = "Cancelled"
	default:
		return d, fmt.Errorf("unreviewed event status")
	}
	d.Title = clean(e.Name)
	d.Date = e.Date
	day, err := time.ParseInLocation("2006-01-02", e.Date, loc)
	if err != nil || day.Format("2006-01-02") != e.Date {
		return d, fmt.Errorf("invalid date")
	}
	d.EventURL = "https://www.levittdenver.org/summer-concert-series#/events/" + strconv.FormatInt(e.ID, 10)
	if e.TicketsURL != "" {
		u, err := url.Parse(e.TicketsURL)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
			return d, fmt.Errorf("invalid ticket URL")
		}
		d.TicketURL = e.TicketsURL
	}
	for _, pair := range []struct {
		raw string
		dst *string
	}{{e.DoorTime, &d.DoorsAt}, {e.StartTime, &d.ShowAt}} {
		if pair.raw == "" {
			continue
		}
		value := e.Date + " " + pair.raw
		at, err := time.ParseInLocation("2006-01-02 15:04:05", value, loc)
		if err != nil || at.Format("2006-01-02 15:04:05") != value {
			return d, fmt.Errorf("invalid local time")
		}
		*pair.dst = at.Format(time.RFC3339)
	}
	root, err := dom.Parse(strings.NewReader(e.Description))
	if err != nil {
		return d, err
	}
	description := html.UnescapeString(text(root))
	categories := map[string]bool{}
	if allAges.MatchString(description) {
		categories["All ages"] = true
	}
	for _, m := range restricted.FindAllStringSubmatch(clean(description), -1) {
		categories[m[1]+"+"] = true
	}
	if e.MinimumAge != nil {
		switch *e.MinimumAge {
		case 0:
			categories["All ages"] = true
		case 13, 16, 18, 21:
			categories[strconv.Itoa(*e.MinimumAge)+"+"] = true
		default:
			return d, fmt.Errorf("unreviewed minimum age")
		}
	}
	if len(categories) > 1 {
		return d, fmt.Errorf("conflicting age restrictions")
	}
	policy := &artifact.AdmissionPolicy{Text: "Age restriction unavailable; check event terms", URL: d.EventURL}
	for category := range categories {
		policy.Category = category
		policy.Text = category
	}
	d.AdmissionPolicy = policy
	return d, nil
}
