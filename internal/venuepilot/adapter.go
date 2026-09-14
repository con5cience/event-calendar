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
	base, baseErr := url.Parse(cfg.AdapterOptions["event_base"])
	layout := cfg.AdapterOptions["layout"]
	options := len(cfg.AdapterOptions) == 1 && layout == ""
	if layout == "dazzle" {
		p := cfg.Venue.AdmissionPolicy
		options = len(cfg.AdapterOptions) == 2 && len(cfg.AdmissionRules) == 0 && p != nil && p.URL != "" && p.WithAdult == nil
	}
	if cfg.Source.Adapter != "venuepilot" || cfg.Venue.Key != cfg.Source.ID || baseErr != nil || base.Scheme != "https" || base.Host == "" || base.User != nil || !options {
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
		data, err := normalize(raw, loc, cfg)
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

func normalize(raw []byte, loc *time.Location, cfg artifact.SourceConfig) (artifact.EventData, error) {
	var e record
	d := artifact.EventData{Status: "Scheduled"}
	if json.Unmarshal(raw, &e) != nil {
		return d, fmt.Errorf("invalid fields")
	}
	if e.Venue.Name != cfg.Venue.Name {
		return d, fmt.Errorf("unreviewed venue")
	}
	switch strings.ToUpper(strings.TrimSpace(e.Status)) {
	case "FREE RSVP", "FREE RSVP/UPGRADE", "TICKETS":
	case "NO COVER":
		if cfg.AdapterOptions["layout"] != "dazzle" {
			return d, fmt.Errorf("unreviewed event status")
		}
	case "CANCELLED", "CANCELED":
		d.Status = "Cancelled"
	default:
		return d, fmt.Errorf("unreviewed event status")
	}
	d.Title = clean(e.Name)
	if cfg.AdapterOptions["layout"] == "dazzle" && d.Title == "Dazzle Membership" {
		return d, fmt.Errorf("membership product, not a performance")
	}
	d.Date = e.Date
	day, err := time.ParseInLocation("2006-01-02", e.Date, loc)
	if err != nil || day.Format("2006-01-02") != e.Date {
		return d, fmt.Errorf("invalid date")
	}
	d.EventURL = cfg.AdapterOptions["event_base"] + strconv.FormatInt(e.ID, 10)
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
		if err != nil || at.Format("2006-01-02 15:04:05") != value || at.Add(time.Hour).Format("2006-01-02 15:04:05") == value || at.Add(-time.Hour).Format("2006-01-02 15:04:05") == value {
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
	if cfg.AdapterOptions["layout"] == "dazzle" && policy.Category == "All ages" {
		policy.Text = "All ages until 11 PM"
		// Doors alone do not establish that the performance starts before curfew.
		if d.Status == "Scheduled" && e.StartTime != "" && e.StartTime < "23:00:00" {
			policy.WithAdult = &artifact.AdultAdmission{URL: cfg.Venue.AdmissionPolicy.URL, ReviewedOn: "2026-09-14", Ranges: []artifact.AdmissionRange{{MinAge: 0, MaxAge: 17, Condition: "Under 21 must leave by 11 PM"}}}
		}
	}
	return d, nil
}
