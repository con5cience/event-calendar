// Package buzzard cross-checks Black Buzzard's public calendar and homepage.
package buzzard

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
	"sort"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode/utf8"
)

const calendarURL = "https://www.theblackbuzzard.com/event-calendar"
const venue = "The Black Buzzard at Oskar Blues Denver"

type pages struct{ Calendar, Home string }
type structured struct {
	Name, StartDate, EventStatus string
	Type                         string `json:"@type"`
	Location                     struct {
		Name    string
		Address struct{ StreetAddress, AddressLocality, AddressRegion, PostalCode string }
	}
	Offers struct{ URL string }
}
type record struct{ ID, Title, Date, Venue, Age, Status, Link, Failure string }

func Decode(cfg artifact.SourceConfig, b []byte, now time.Time) (artifact.Refresh, error) {
	fail := func(s string) (artifact.Refresh, error) { return artifact.Refresh{}, fmt.Errorf("buzzard: %s", s) }
	raw, err := json.Marshal(cfg)
	if err != nil {
		return fail("configuration")
	}
	cfg, err = artifact.DecodeConfigJSON(raw)
	if err != nil {
		return artifact.Refresh{}, err
	}
	if cfg.Source.ID != "black-buzzard" || cfg.Source.Adapter != "html-buzzard" || cfg.Venue.Key != "black-buzzard" || cfg.Venue.Name != "The Black Buzzard" || cfg.Venue.Website != calendarURL || cfg.Venue.Timezone != "America/Denver" || len(cfg.AdapterOptions) != 0 {
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
	rows, err := parse(s.Pages)
	if err != nil {
		return fail(err.Error())
	}
	check, err := parse(s.Check)
	if err != nil || !reflect.DeepEqual(rows, check) {
		return fail("incomplete or changed capture")
	}
	out := artifact.Refresh{GeneratedAt: now.UTC().Format(time.RFC3339Nano), Coverage: artifact.Coverage{From: s.From, Through: s.Through, Complete: true}, Observations: []artifact.Observation{}}
	for _, r := range rows {
		d := artifact.EventData{Title: r.Title, Date: r.Date, Status: "Scheduled", EventURL: r.Link, TicketURL: r.Link}
		failure := r.Failure
		switch r.Status {
		case "https://schema.org/EventScheduled":
		case "https://schema.org/EventCancelled":
			d.Status = "Cancelled"
			d.TicketURL = ""
		default:
			failure = "unreviewed event status"
		}
		if r.Venue != venue {
			failure = "unreviewed venue"
		}
		if r.Age != "" && r.Age != "This is some text inside of a div block." {
			p := &artifact.AdmissionPolicy{Text: r.Age, URL: "https://www.theblackbuzzard.com/"}
			switch r.Age {
			case "13", "16", "18", "21":
				p.Category = r.Age + "+"
				p.Text = p.Category
			case "13+", "16+", "18+", "21+":
				p.Category = r.Age
			case "All ages", "All Ages":
				p.Category = "All ages"
			}
			d.AdmissionPolicy = p
		}
		if failure == "" && (d.Date < s.From || d.Date > s.Through) {
			continue
		}
		raw, _ := json.Marshal(d)
		out.Observations = append(out.Observations, artifact.Observation{UpstreamID: r.ID, Data: raw, Failure: failure})
	}
	return out, nil
}

// Follow the established inert x/net/html traversal; selectors remain scoped.
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
func one(n *dom.Node, k string) (string, error) {
	ns := find(n, func(n *dom.Node) bool { return class(n, k) })
	if len(ns) != 1 {
		return "", fmt.Errorf("missing or duplicate %s", k)
	}
	return text(ns[0]), nil
}

var idURL = regexp.MustCompile(`^https://tixr\.com/e/([1-9][0-9]{0,15})$`)

func identity(s string) (string, error) {
	m := idURL.FindStringSubmatch(s)
	if m == nil {
		return "", fmt.Errorf("invalid ticket identity")
	}
	return m[1], nil
}
func tree(h string) (*dom.Node, error) {
	if h == "" || len(h) > 1024*1024 {
		return nil, fmt.Errorf("missing or oversized page")
	}
	root, err := dom.Parse(strings.NewReader(h))
	if err != nil {
		return nil, err
	}
	if len(find(root, func(n *dom.Node) bool { return class(n, "w-pagination-next") || attr(n, "rel") == "next" })) > 0 {
		return nil, fmt.Errorf("unreviewed pagination")
	}
	return root, nil
}
func parse(p pages) ([]record, error) {
	cal, err := tree(p.Calendar)
	if err != nil {
		return nil, err
	}
	home, err := tree(p.Home)
	if err != nil {
		return nil, err
	}
	containers := find(cal, func(n *dom.Node) bool { return class(n, "cal-filter-list") })
	if len(containers) != 1 {
		return nil, fmt.Errorf("calendar layout changed")
	}
	cards := find(containers[0], func(n *dom.Node) bool { return class(n, "cal-info") })
	if len(cards) == 0 || len(cards) >= 100 {
		return nil, fmt.Errorf("unverified empty or potentially capped calendar")
	}
	rows := map[string]record{}
	for _, card := range cards {
		r := record{}
		r.Title, err = one(card, "b-show")
		if err != nil {
			return nil, err
		}
		names := find(card, func(n *dom.Node) bool { return class(n, "b-venue") && class(n, "name") })
		dates := find(card, func(n *dom.Node) bool { return class(n, "b-venue") && !class(n, "name") && !class(n, "filter") })
		if len(names) != 1 || len(dates) != 1 {
			return nil, fmt.Errorf("calendar fields changed")
		}
		r.Venue = text(names[0])
		day, err := time.Parse("January 2, 2006", text(dates[0]))
		if err != nil {
			r.Failure = "invalid date"
		} else {
			r.Date = day.Format("2006-01-02")
		}
		links := find(card, func(n *dom.Node) bool { return n.Data == "a" })
		if len(links) == 0 {
			return nil, fmt.Errorf("missing event identity")
		}
		for _, a := range links {
			id, err := identity(attr(a, "href"))
			if err != nil {
				return nil, err
			}
			if r.ID != "" && r.ID != id {
				return nil, fmt.Errorf("conflicting card IDs")
			}
			r.ID = id
			r.Link = attr(a, "href")
		}
		if _, ok := rows[r.ID]; ok {
			return nil, fmt.Errorf("duplicate calendar ID")
		}
		rows[r.ID] = r
	}
	lists := find(home, func(n *dom.Node) bool { return class(n, "event-list") })
	if len(lists) != 1 {
		return nil, fmt.Errorf("home layout changed")
	}
	items := find(lists[0], func(n *dom.Node) bool { return class(n, "event-item") })
	if len(items) != len(rows) {
		return nil, fmt.Errorf("home coverage mismatch")
	}
	seen := map[string]bool{}
	for _, item := range items {
		scripts := find(item, func(n *dom.Node) bool { return n.Data == "script" && attr(n, "type") == "application/ld+json" })
		if len(scripts) != 1 || scripts[0].FirstChild == nil {
			return nil, fmt.Errorf("missing event JSON-LD")
		}
		var e structured
		if json.Unmarshal([]byte(scripts[0].FirstChild.Data), &e) != nil || e.Type != "Event" {
			return nil, fmt.Errorf("invalid event JSON-LD")
		}
		id, err := identity(e.Offers.URL)
		if err != nil {
			return nil, err
		}
		r, ok := rows[id]
		if !ok || seen[id] {
			return nil, fmt.Errorf("missing or duplicate home identity")
		}
		seen[id] = true
		if strings.Join(strings.Fields(html.UnescapeString(e.Name)), " ") != r.Title {
			return nil, fmt.Errorf("event titles disagree")
		}
		day, err := time.Parse("Jan 02, 2006", e.StartDate)
		if err != nil {
			r.Failure = "invalid structured date"
		} else if r.Failure == "" && day.Format("2006-01-02") != r.Date {
			return nil, fmt.Errorf("event dates disagree")
		}
		a := e.Location.Address
		if e.Location.Name != "The Black Buzzard at Oskar Blues" || a.StreetAddress != "1624 Market St" || a.AddressLocality != "Denver" || a.AddressRegion != "CO" || a.PostalCode != "80202" {
			r.Failure = "unreviewed structured venue"
		}
		r.Status = e.EventStatus
		rows[id] = r
	}
	features := find(home, func(n *dom.Node) bool { return class(n, "featured-list") })
	if len(features) != 1 {
		return nil, fmt.Errorf("age list changed")
	}
	ages := find(features[0], func(n *dom.Node) bool { return class(n, "event-card") })
	if len(ages) != len(rows) {
		return nil, fmt.Errorf("age coverage mismatch")
	}
	seen = map[string]bool{}
	for _, a := range ages {
		id, err := identity(attr(a, "href"))
		if err != nil {
			return nil, err
		}
		r, ok := rows[id]
		if !ok || seen[id] {
			return nil, fmt.Errorf("age identity mismatch")
		}
		seen[id] = true
		title, err := one(a, "main-title-hover")
		if err != nil || title != r.Title {
			return nil, fmt.Errorf("age title mismatch")
		}
		v, err := one(a, "venue-name")
		if err != nil || v != r.Venue {
			return nil, fmt.Errorf("age venue mismatch")
		}
		r.Age, err = one(a, "number")
		if err != nil {
			return nil, err
		}
		rows[id] = r
	}
	out := []record{}
	for _, r := range rows {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
