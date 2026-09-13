// Package herbs reads the printed calendar, not Squarespace's mis-zoned epochs.
package herbs

import (
	"bytes"
	"encoding/json"
	"event-calendar/internal/artifact"
	"fmt"
	dom "golang.org/x/net/html"
	"io"
	"net/url"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode/utf8"
)

const calendarURL = "https://www.herbsbar.com/live-music-calendar-1"
const policyText = "21+; minors may attend until 10:30 PM with a parent"

type row struct{ ID, Title, Date, Clock12, Clock24, Link, Status, Restriction, Failure string }

func Decode(cfg artifact.SourceConfig, b []byte, now time.Time) (artifact.Refresh, error) {
	fail := func(s string) (artifact.Refresh, error) { return artifact.Refresh{}, fmt.Errorf("herbs: %s", s) }
	raw, err := json.Marshal(cfg)
	if err != nil {
		return fail("configuration")
	}
	cfg, err = artifact.DecodeConfigJSON(raw)
	if err != nil {
		return artifact.Refresh{}, err
	}
	p := cfg.Venue.AdmissionPolicy
	if cfg.Source.ID != "herbs" || cfg.Source.Adapter != "html-herbs" || cfg.Venue.Key != "herbs" || cfg.Venue.Name != "Herb's" || cfg.Venue.Timezone != "America/Denver" || cfg.Venue.Website != calendarURL || len(cfg.AdapterOptions) != 0 || len(cfg.AdmissionRules) != 0 || p == nil || p.Text != policyText || p.Category != "21+" || p.URL != "https://www.herbsbar.com/" || p.WithAdult != nil {
		return fail("unsupported configuration")
	}
	if now.IsZero() || len(b) > artifact.MaxDocumentBytes || !utf8.Valid(b) {
		return fail("invalid snapshot")
	}
	var s struct {
		From, Through, HTML string
		CheckHTML           string `json:"check_html"`
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
	rows, err := parse(s.HTML)
	if err != nil {
		return fail(err.Error())
	}
	check, err := parse(s.CheckHTML)
	if err != nil || !reflect.DeepEqual(rows, check) {
		return fail("incomplete or changed capture")
	}
	out := artifact.Refresh{GeneratedAt: now.UTC().Format(time.RFC3339Nano), Coverage: artifact.Coverage{From: s.From, Through: s.Through, Complete: true}, Observations: []artifact.Observation{}}
	for _, r := range rows {
		d := artifact.EventData{Title: r.Title, Date: r.Date, EventURL: r.Link, Status: "Scheduled"}
		failure := r.Failure
		if _, err := time.Parse("2006-01-02", r.Date); err != nil {
			failure = "invalid date"
		}
		switch strings.ToLower(r.Status) {
		case "", "sold out":
		case "cancelled", "canceled":
			d.Status = "Cancelled"
		default:
			failure = "unreviewed status"
		}
		before := false
		if r.Clock12 != "" || r.Clock24 != "" {
			a, err12 := time.Parse("3:04 PM", r.Clock12)
			v, err24 := time.Parse("15:04", r.Clock24)
			if err12 != nil || err24 != nil || a.Format("15:04") != v.Format("15:04") {
				failure = "invalid or conflicting clock"
			} else {
				wall := r.Date + "T" + v.Format("15:04")
				t, err := time.ParseInLocation("2006-01-02T15:04", wall, loc)
				if err != nil || t.Format("2006-01-02T15:04") != wall || t.Add(time.Hour).Format("2006-01-02T15:04") == wall || t.Add(-time.Hour).Format("2006-01-02T15:04") == wall {
					failure = "invalid or ambiguous Denver clock"
				} else {
					d.ShowAt = t.Format(time.RFC3339)
					before = v.Hour()*60+v.Minute() < 22*60+30
				}
			}
		}
		if r.Restriction != "" {
			d.AdmissionPolicy = &artifact.AdmissionPolicy{Text: r.Restriction, URL: r.Link}
		} else if before && d.Status == "Scheduled" && failure == "" {
			copy := *p
			copy.WithAdult = &artifact.AdultAdmission{URL: p.URL, ReviewedOn: "2026-09-12", Ranges: []artifact.AdmissionRange{{MinAge: 0, MaxAge: 17, Condition: "Parent required; minors must leave by 10:30 PM"}}}
			d.AdmissionPolicy = &copy
		}
		if failure == "" && (d.Date < s.From || d.Date > s.Through) {
			continue
		}
		raw, _ := json.Marshal(d)
		out.Observations = append(out.Observations, artifact.Observation{UpstreamID: r.ID, Data: raw, Failure: failure})
	}
	return out, nil
}

// Inert, scoped HTML traversal follows the existing HTML adapters.
func attr(n *dom.Node, k string) string {
	for _, a := range n.Attr {
		if a.Key == k {
			return a.Val
		}
	}
	return ""
}
func class(n *dom.Node, k string) bool {
	for _, v := range strings.Fields(attr(n, "class")) {
		if v == k {
			return true
		}
	}
	return false
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
	return strings.Join(strings.Fields(b.String()), " ")
}
func find(n *dom.Node, p func(*dom.Node) bool) []*dom.Node {
	out := []*dom.Node{}
	var walk func(*dom.Node)
	walk = func(n *dom.Node) {
		if n.Type == dom.ElementNode && p(n) {
			out = append(out, n)
		}
		if n.Data == "script" || n.Data == "style" {
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}
func selected(n *dom.Node, k string) []*dom.Node {
	return find(n, func(n *dom.Node) bool { return class(n, k) })
}

var idPattern = regexp.MustCompile(`^[a-f0-9]{24}$`)
var restrictionPattern = regexp.MustCompile(`(?i)\b(?:\d{1,2}\s*\+|ages?\s+\d|all\s+ages\b|no\s+minors\b|adults?\s+only\b|under\s+\d|over\s+\d)`)

func parse(h string) ([]row, error) {
	if h == "" || len(h) > 1024*1024 {
		return nil, fmt.Errorf("missing or oversized page")
	}
	root, err := dom.Parse(strings.NewReader(h))
	if err != nil {
		return nil, err
	}
	if len(find(root, func(n *dom.Node) bool { return attr(n, "rel") == "next" })) > 0 {
		return nil, fmt.Errorf("unreviewed pagination")
	}
	footers := find(root, func(n *dom.Node) bool { return n.Data == "footer" })
	verified := false
	for _, f := range footers {
		s := text(f)
		if strings.Contains(s, "2057 Larimer Street") && strings.Contains(s, "Herb’s is a 21+ Establishment") && strings.Contains(s, "Minors are allowed until 10:30pm if accompanied by a Parent") {
			verified = true
		}
	}
	if !verified {
		return nil, fmt.Errorf("admission policy or venue changed")
	}
	lists := selected(root, "eventlist--upcoming")
	if len(lists) != 1 {
		return nil, fmt.Errorf("calendar layout changed")
	}
	cards := find(lists[0], func(n *dom.Node) bool { return n.Data == "article" && class(n, "eventlist-event") })
	if len(cards) == 0 || len(cards) >= 500 {
		return nil, fmt.Errorf("unverified empty or potentially capped calendar")
	}
	rows := []row{}
	seen := map[string]bool{}
	for _, c := range cards {
		r := row{}
		ids := find(c, func(n *dom.Node) bool { return attr(n, "data-item-id") != "" })
		if len(ids) != 1 || !idPattern.MatchString(attr(ids[0], "data-item-id")) {
			return nil, fmt.Errorf("invalid event identity")
		}
		r.ID = attr(ids[0], "data-item-id")
		if seen[r.ID] {
			return nil, fmt.Errorf("duplicate identity")
		}
		seen[r.ID] = true
		links := selected(c, "eventlist-title-link")
		if len(links) != 1 {
			return nil, fmt.Errorf("missing title link")
		}
		r.Title = text(links[0])
		u, err := url.Parse(attr(links[0], "href"))
		if err != nil {
			return nil, err
		}
		base, _ := url.Parse(calendarURL)
		u = base.ResolveReference(u)
		if u.Scheme != "https" || u.Host != "www.herbsbar.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || !strings.HasPrefix(u.Path, "/live-music-calendar-1/") {
			return nil, fmt.Errorf("unreviewed event link")
		}
		r.Link = u.String()
		dates := selected(c, "event-date")
		if len(dates) < 1 {
			r.Failure = "missing date"
		} else {
			r.Date = attr(dates[0], "datetime")
		}
		k12, k24 := "event-time-12hr-start", "event-time-24hr-start"
		if class(c, "eventlist-event--multiday") {
			k12, k24 = "event-time-12hr", "event-time-24hr"
		}
		a, b := selected(c, k12), selected(c, k24)
		if len(a) > 0 && len(b) > 0 {
			r.Clock12 = text(a[0])
			r.Clock24 = text(b[0])
			if attr(a[0], "datetime") != r.Date || attr(b[0], "datetime") != r.Date {
				r.Failure = "clock date mismatch"
			}
		} else if len(a) != len(b) {
			r.Failure = "incomplete clock"
		}
		statuses := selected(c, "eventlist-datetag-status")
		if len(statuses) > 1 {
			return nil, fmt.Errorf("duplicate status")
		}
		if len(statuses) == 1 {
			r.Status = text(statuses[0])
		}
		for _, a := range selected(c, "eventlist-meta-address") {
			if text(a) != "" {
				r.Failure = "unreviewed off-site location"
			}
		}
		for _, e := range find(c, func(n *dom.Node) bool { return class(n, "eventlist-excerpt") || class(n, "eventlist-description") }) {
			s := text(e)
			if restrictionPattern.MatchString(s) {
				r.Restriction = s
			}
		}
		if restrictionPattern.MatchString(r.Title) {
			r.Restriction = r.Title
		}
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows, nil
}
