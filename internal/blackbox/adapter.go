// Package blackbox normalizes scoped public Supabase event captures.
package blackbox

import (
	"bytes"
	"encoding/json"
	"event-calendar/internal/artifact"
	"fmt"
	dom "golang.org/x/net/html"
	"html"
	"io"
	"reflect"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode/utf8"
)

type record struct {
	TicketError                                                   string `json:"ticket_error"`
	ID, Slug, Title, Date, Time, Venue, Location, Address, Status string
	TicketURL                                                     string  `json:"ticket_url"`
	TicketTitle                                                   string  `json:"ticket_title"`
	Summary                                                       string  `json:"summary_html"`
	Age                                                           *string `json:"age_restriction"`
}

func Decode(cfg artifact.SourceConfig, b []byte, now time.Time) (artifact.Refresh, error) {
	fail := func(s string) (artifact.Refresh, error) { return artifact.Refresh{}, fmt.Errorf("blackbox: %s", s) }
	raw, err := json.Marshal(cfg)
	if err != nil {
		return fail("invalid configuration")
	}
	cfg, err = artifact.DecodeConfigJSON(raw)
	if err != nil {
		return artifact.Refresh{}, err
	}
	if cfg.Source.ID != "black-box" || cfg.Source.Adapter != "supabase-events" || cfg.Venue.Key != "black-box" || cfg.Venue.Name != "The Black Box" || cfg.Venue.Website != "https://blackboxdenver.co" || cfg.Venue.Timezone != "America/Denver" || len(cfg.AdapterOptions) != 0 {
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
	seen := map[string]bool{}
	for _, raw := range s.Events {
		var id struct{ ID string }
		if json.Unmarshal(raw, &id) != nil || !uuid.MatchString(id.ID) || seen[id.ID] {
			return fail("invalid identity")
		}
		seen[id.ID] = true
		e, err := normalize(raw, loc)
		if err == nil && e.AdmissionPolicy != nil && e.AdmissionPolicy.Category == "All ages" {
			if rule, ok := cfg.AdmissionRules["All ages"]; ok {
				rule.URL = e.TicketURL
				e.AdmissionPolicy.WithAdult = &rule
			}
		}
		failure := ""
		if err != nil {
			failure = err.Error()
		}
		if failure == "" && (e.Date < s.From || e.Date > s.Through) {
			return fail("record outside query coverage")
		}
		data, _ := json.Marshal(e)
		out.Observations = append(out.Observations, artifact.Observation{UpstreamID: id.ID, Data: data, Failure: failure})
	}
	return out, nil
}

var uuid = regexp.MustCompile(`^[a-f0-9]{8}(-[a-f0-9]{4}){3}-[a-f0-9]{12}$`)
var slug = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var clock = regexp.MustCompile(`(?i)\b(doors|music)\s*:\s*(\d{1,2}(?::\d{2})?\s*[ap]m)\b`)

func clean(s string) string { return strings.Join(strings.Fields(html.UnescapeString(s)), " ") }
func text(n *dom.Node) string {
	if n.Type == dom.TextNode {
		return n.Data
	}
	if n.Type == dom.ElementNode && (n.Data == "script" || n.Data == "style") {
		return ""
	}
	out := ""
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		out += text(c)
	}
	if n.Data == "p" || n.Data == "div" || n.Data == "br" {
		out += " "
	}
	return out
}
func normalize(raw []byte, loc *time.Location) (artifact.EventData, error) {
	var e record
	d := artifact.EventData{Status: "Scheduled"}
	if json.Unmarshal(raw, &e) != nil {
		return d, fmt.Errorf("invalid fields")
	}
	if e.TicketError != "" {
		return d, fmt.Errorf("unsupported ticket provider")
	}
	d.Title = clean(e.Title)
	d.Date = e.Date
	if (e.Venue != "The Black Box" && e.Venue != "The Lounge") || e.Location != "Denver" || e.Address != "314 East 13th Avenue, Denver, 80203" {
		return d, fmt.Errorf("unreviewed venue")
	}
	if e.Status != "published" {
		return d, fmt.Errorf("unpublished record")
	}
	if !slug.MatchString(e.Slug) || e.TicketURL != "https://events.blackboxdenver.co/e/"+e.Slug+"/tickets" {
		return d, fmt.Errorf("invalid ticket URL")
	}
	if e.TicketTitle != "" && clean(e.TicketTitle) != d.Title {
		return d, fmt.Errorf("ticket title mismatch")
	}
	if at, err := time.ParseInLocation("2006-01-02", e.Date, loc); err != nil || at.Format("2006-01-02") != e.Date {
		return d, fmt.Errorf("invalid date")
	}
	d.EventURL = "https://blackboxdenver.co/events/" + e.Slug + "/tickets"
	d.TicketURL = e.TicketURL
	root, err := dom.Parse(strings.NewReader(e.Summary))
	if err != nil {
		return d, err
	}
	clocks := map[string]string{}
	for _, m := range clock.FindAllStringSubmatch(text(root), -1) {
		value := strings.ToUpper(strings.ReplaceAll(m[2], " ", ""))
		layout := "2006-01-02 3:04PM"
		if !strings.Contains(value, ":") {
			layout = "2006-01-02 3PM"
		}
		at, err := time.ParseInLocation(layout, e.Date+" "+value, loc)
		if err != nil {
			return d, fmt.Errorf("invalid labeled clock")
		}
		key := strings.ToLower(m[1])
		v := at.Format(time.RFC3339)
		if prior, ok := clocks[key]; ok && prior != v {
			return d, fmt.Errorf("conflicting labeled clocks")
		}
		clocks[key] = v
	}
	d.DoorsAt = clocks["doors"]
	d.ShowAt = clocks["music"]
	if d.DoorsAt != "" && d.ShowAt != "" {
		a, _ := time.Parse(time.RFC3339, d.DoorsAt)
		b, _ := time.Parse(time.RFC3339, d.ShowAt)
		if b.Before(a) {
			return d, fmt.Errorf("music precedes doors")
		}
	}
	policy := &artifact.AdmissionPolicy{Text: "Age restriction unavailable; check event terms", URL: e.TicketURL}
	if e.Age != nil && clean(*e.Age) != "" {
		policy.Text = clean(*e.Age)
		switch strings.ToLower(policy.Text) {
		case "all ages":
			policy.Category = "All ages"
		case "13+", "16+", "18+", "21+":
			policy.Category = policy.Text
		}
	}
	d.AdmissionPolicy = policy
	return d, nil
}
