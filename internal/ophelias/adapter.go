// Package ophelias implements the scoped server-rendered calendar HTML profile.
package ophelias

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

const listingURL = "https://opheliasdenver.com/calendar/"

type record struct{ ID, Title, Age, Excerpt, Day, Month, Year, Clock, Zone, Link, Action string }

func Decode(cfg artifact.SourceConfig, b []byte, now time.Time) (artifact.Refresh, error) {
	fail := func(s string) (artifact.Refresh, error) { return artifact.Refresh{}, fmt.Errorf("ophelias: %s", s) }
	raw, err := json.Marshal(cfg)
	if err != nil {
		return fail("invalid configuration")
	}
	cfg, err = artifact.DecodeConfigJSON(raw)
	if err != nil {
		return artifact.Refresh{}, err
	}
	if cfg.Source.ID != "ophelias" || cfg.Source.Adapter != "html-ophelias" || cfg.Venue.Key != "ophelias" || cfg.Venue.Name != "Ophelia's Electric Soapbox" || cfg.Venue.Website != listingURL || cfg.Venue.Timezone != "America/Denver" || len(cfg.AdapterOptions) != 0 {
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
	events, err := parse(s.HTML)
	if err != nil {
		return fail(err.Error())
	}
	check, err := parse(s.CheckHTML)
	if err != nil || !reflect.DeepEqual(events, check) {
		return fail("incomplete or changed capture")
	}
	out := artifact.Refresh{GeneratedAt: now.UTC().Format(time.RFC3339Nano), Coverage: artifact.Coverage{From: s.From, Through: s.Through, Complete: true}, Observations: []artifact.Observation{}}
	for _, e := range events {
		d, err := normalize(e, loc)
		if err == nil && (d.Date < s.From || d.Date > s.Through) {
			continue
		}
		failure := ""
		if err != nil {
			failure = err.Error()
		}
		raw, _ := json.Marshal(d)
		out.Observations = append(out.Observations, artifact.Observation{UpstreamID: e.ID, Data: raw, Failure: failure})
	}
	return out, nil
}

// Reuse the established x/net/html traversal convention; these selectors are
// source-specific and do not change the other adapters' parsing contracts.
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

var ticketID = regexp.MustCompile(`/event/([A-Fa-f0-9]{16})/?$`)

func parse(h string) ([]record, error) {
	if h == "" || len(h) > 1024*1024 {
		return nil, fmt.Errorf("missing or oversized HTML")
	}
	root, err := dom.Parse(strings.NewReader(h))
	if err != nil {
		return nil, err
	}
	for _, n := range find(root, func(n *dom.Node) bool { return n.Data == "a" || n.Data == "button" }) {
		if attr(n, "rel") == "next" || class(n, "next") || class(n, "page-numbers") || regexp.MustCompile(`(?i)^(load more|next|next page)$`).MatchString(text(n)) {
			return nil, fmt.Errorf("unreviewed pagination")
		}
	}
	mains := find(root, func(n *dom.Node) bool { return attr(n, "id") == "primary" && class(n, "eventspage") })
	if len(mains) != 1 {
		return nil, fmt.Errorf("calendar container changed")
	}
	heads := find(mains[0], func(n *dom.Node) bool { return n.Data == "h1" })
	if len(heads) != 1 || text(heads[0]) != "Upcoming Events" {
		return nil, fmt.Errorf("calendar heading changed")
	}
	cards := find(mains[0], func(n *dom.Node) bool { return class(n, "events-item") })
	if len(cards) == 0 || len(cards) > 500 {
		return nil, fmt.Errorf("unverified empty calendar or cap exceeded")
	}
	out := []record{}
	seen := map[string]bool{}
	for _, card := range cards {
		one := func(k string) (string, error) {
			ns := find(card, func(n *dom.Node) bool { return class(n, k) })
			if len(ns) != 1 {
				return "", fmt.Errorf("missing or duplicate %s", k)
			}
			return text(ns[0]), nil
		}
		e := record{}
		titles := find(card, func(n *dom.Node) bool { return n.Data == "h4" })
		if len(titles) != 1 {
			return nil, fmt.Errorf("title element changed")
		}
		e.Title = text(titles[0])
		ages := find(card, func(n *dom.Node) bool { return class(n, "age_r") })
		if len(ages) > 1 {
			return nil, fmt.Errorf("duplicate age field")
		}
		if len(ages) == 1 {
			e.Age = text(ages[0])
		}
		for _, p := range []struct {
			k   string
			dst *string
		}{{"ei-excerpt", &e.Excerpt}, {"ei-day", &e.Day}} {
			v, err := one(p.k)
			if err != nil {
				return nil, err
			}
			*p.dst = v
		}
		for _, p := range []struct {
			k    string
			a, b *string
		}{{"ei-m", &e.Month, &e.Year}, {"ei-time", &e.Clock, &e.Zone}} {
			ns := find(card, func(n *dom.Node) bool { return class(n, p.k) })
			if len(ns) != 1 {
				return nil, fmt.Errorf("date/time layout changed")
			}
			ps := find(ns[0], func(n *dom.Node) bool { return n.Data == "p" })
			if len(ps) != 2 {
				return nil, fmt.Errorf("date/time fields changed")
			}
			*p.a = text(ps[0])
			*p.b = text(ps[1])
		}
		actions := find(card, func(n *dom.Node) bool { return class(n, "ei-action") })
		if len(actions) != 1 {
			return nil, fmt.Errorf("action layout changed")
		}
		links := find(actions[0], func(n *dom.Node) bool { return n.Data == "a" })
		if len(links) != 1 {
			return nil, fmt.Errorf("missing identity link")
		}
		e.Link = attr(links[0], "href")
		e.Action = text(links[0])
		u, err := url.Parse(e.Link)
		if err != nil || u.Scheme != "https" || u.Host != "www.ticketmaster.com" || u.User != nil || u.Fragment != "" {
			return nil, fmt.Errorf("unreviewed event URL")
		}
		m := ticketID.FindStringSubmatch(u.Path)
		if m == nil {
			return nil, fmt.Errorf("missing event ID")
		}
		e.ID = strings.ToUpper(m[1])
		if seen[e.ID] {
			return nil, fmt.Errorf("duplicate event ID")
		}
		seen[e.ID] = true
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

var ageLabel = regexp.MustCompile(`^Ages (13|16|18|21)\+$`)
var statedAge = regexp.MustCompile(`(?i)\bAges (13|16|18|21)\+`)
var guardian = regexp.MustCompile(`(?i)\bAnyone under (?:the age of )?16 (?:must|needs to) be accompanied by a (?:legal )?guardian\.`)
var cancelled = regexp.MustCompile(`(?i)\bcancell?ed\b`)
var beforeClock = regexp.MustCompile(`(?i)\b(doors|show)\s*:?\s*(\d{1,2}(?::\d{2})?\s*[ap](?:m)?)\b`)
var afterClock = regexp.MustCompile(`(?i)\b(\d{1,2}(?::\d{2})?\s*[ap](?:m)?)\s+(doors|show)\b`)
var clockFormat = regexp.MustCompile(`^\d{1,2}(?::\d{2})?[AP]M?$`)

func normalize(e record, loc *time.Location) (artifact.EventData, error) {
	d := artifact.EventData{Title: e.Title, Status: "Scheduled", EventURL: e.Link, TicketURL: e.Link}
	day, err := time.ParseInLocation("2 January 2006", e.Day+" "+e.Month+" "+e.Year, loc)
	if err != nil {
		return d, fmt.Errorf("invalid date")
	}
	d.Date = day.Format("2006-01-02")
	if cancelled.MatchString(e.Action) || cancelled.MatchString(e.Title) || cancelled.MatchString(e.Excerpt) {
		d.Status = "Cancelled"
		d.TicketURL = ""
	} else if e.Action != "Buy Tickets" && e.Action != "Sold Out" {
		return d, fmt.Errorf("unreviewed action status")
	}
	clock := func(s string) (string, error) {
		s = strings.ToUpper(strings.ReplaceAll(s, " ", ""))
		if !clockFormat.MatchString(s) {
			return "", fmt.Errorf("invalid clock")
		}
		if strings.HasSuffix(s, "A") || strings.HasSuffix(s, "P") {
			s += "M"
		}
		if !strings.Contains(s, ":") {
			s = s[:len(s)-2] + ":00" + s[len(s)-2:]
		}
		at, err := time.ParseInLocation("2006-01-02 3:04PM", d.Date+" "+s, loc)
		if err != nil {
			return "", fmt.Errorf("invalid clock")
		}
		if at.Format("3:04PM") != strings.TrimLeft(s, "0") {
			return "", fmt.Errorf("nonexistent local clock")
		}
		for _, delta := range []time.Duration{-time.Hour, time.Hour} {
			if at.Add(delta).Format("2006-01-02 3:04PM") == at.Format("2006-01-02 3:04PM") {
				return "", fmt.Errorf("ambiguous local clock")
			}
		}
		return at.Format(time.RFC3339), nil
	}
	if e.Clock != "" {
		if e.Zone != "MT" {
			return d, fmt.Errorf("unknown timezone")
		}
		d.ShowAt, err = clock(e.Clock)
		if err != nil {
			return d, err
		}
	}
	times := map[string]string{}
	put := func(kind, value string) error {
		v, err := clock(value)
		if err != nil {
			return err
		}
		kind = strings.ToLower(kind)
		if old := times[kind]; old != "" && old != v {
			return fmt.Errorf("conflicting labeled times")
		}
		times[kind] = v
		return nil
	}
	for _, m := range beforeClock.FindAllStringSubmatch(e.Excerpt, -1) {
		if err := put(m[1], m[2]); err != nil {
			return d, err
		}
	}
	for _, m := range afterClock.FindAllStringSubmatch(e.Excerpt, -1) {
		if err := put(m[2], m[1]); err != nil {
			return d, err
		}
	}
	d.DoorsAt = times["doors"]
	if show := times["show"]; show != "" {
		if d.ShowAt != "" && d.ShowAt != show {
			return d, fmt.Errorf("conflicting show clock")
		}
		d.ShowAt = show
	}
	category := ""
	if m := ageLabel.FindStringSubmatch(e.Age); m != nil {
		category = m[1] + "+"
	}
	for _, m := range statedAge.FindAllStringSubmatch(e.Excerpt, -1) {
		if category != "" && category != m[1]+"+" {
			return d, fmt.Errorf("conflicting admission")
		}
		category = m[1] + "+"
	}
	if e.Age == "" && category == "" {
		return d, nil
	} // Reviewed venue default, not an All Ages inference.
	policy := &artifact.AdmissionPolicy{Text: e.Age, Category: category, URL: listingURL}
	if policy.Text == "" {
		policy.Text = category
	}
	if policy.Text == "" {
		policy.Text = "Age restriction unavailable"
	}
	if category == "16+" && guardian.MatchString(e.Excerpt) {
		policy.WithAdult = &artifact.AdultAdmission{URL: listingURL, ReviewedOn: "2026-09-12", Ranges: []artifact.AdmissionRange{{MinAge: 13, MaxAge: 15, Condition: "Ticket and legal guardian required"}, {MinAge: 16, MaxAge: 17, Condition: "Ticket and valid identification required"}}}
	}
	d.AdmissionPolicy = policy
	return d, nil
}
