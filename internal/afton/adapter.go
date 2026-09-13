// Package afton normalizes the venue-owned Roxy widget and linked Afton pages.
package afton

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
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode/utf8"
)

type record struct {
	Type         string          `json:"event_type"`
	ID           json.RawMessage `json:"event_id"`
	Name         string          `json:"event_name"`
	Start        string          `json:"start_time"`
	Doors        string          `json:"door_time"`
	Venue        string          `json:"venue_name"`
	City         string          `json:"city"`
	State        string          `json:"state_abbreviation"`
	Link         string          `json:"buy_ticket_url"`
	Hide         string          `json:"hide_start_time"`
	DisplayDoors string          `json:"display_door_start_time"`
	Entry        string          `json:"entry_type"`
	SoldOut      bool            `json:"sold_out"`
}
type page struct {
	Page, Total int
	PerPage     int `json:"per_page"`
	Next        bool
	Events      []record
}
type snapshot struct {
	From, Through string
	Pages         []page
	CheckPages    []page `json:"check_pages"`
	Details       map[string]string
	CheckDetails  map[string]string `json:"check_details"`
}

var nativeID = regexp.MustCompile(`^[a-z0-9]{10}$`)

func identity(e record) (string, error) {
	if e.Type == "real_world" {
		var s string
		if json.Unmarshal(e.ID, &s) == nil && nativeID.MatchString(s) && e.Link == "https://aftontickets.com/event/buyticket/"+s {
			return s, nil
		}
	} else if e.Type == "external_event" {
		var n int64
		if json.Unmarshal(e.ID, &n) == nil && n > 0 && n <= 9007199254740991 {
			u, err := url.Parse(e.Link)
			if err == nil && u.Scheme == "https" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && ((u.Host == "tickets.strongsurvivepresents.com" && regexp.MustCompile(`^/tickets/[0-9]+$`).MatchString(u.Path)) || (u.Host == "www.skeletix.com" && regexp.MustCompile(`^/[0-9]+-[a-zA-Z0-9-]+/$`).MatchString(u.Path))) {
				return "external-" + strconv.FormatInt(n, 10), nil
			}
		}
	}
	return "", fmt.Errorf("invalid identity or link")
}
func enumerate(pages []page) ([]record, error) {
	out := []record{}
	seen := map[string]bool{}
	if len(pages) == 0 || len(pages) > 42 {
		return nil, fmt.Errorf("missing pages")
	}
	total := pages[0].Total
	if total < 1 || total > 500 || len(pages) != (total+11)/12 {
		return nil, fmt.Errorf("unverified empty or incomplete pages")
	}
	for i, p := range pages {
		want := min(12, total-i*12)
		if p.Page != i+1 || p.Total != total || p.PerPage != 12 || p.Next != (i < len(pages)-1) || len(p.Events) != want {
			return nil, fmt.Errorf("incomplete page")
		}
		for _, e := range p.Events {
			id, err := identity(e)
			if err != nil {
				return nil, err
			}
			if seen[id] {
				return nil, fmt.Errorf("duplicate ID")
			}
			seen[id] = true
			out = append(out, e)
		}
	}
	return out, nil
}

func Decode(cfg artifact.SourceConfig, b []byte, now time.Time) (artifact.Refresh, error) {
	fail := func(s string) (artifact.Refresh, error) { return artifact.Refresh{}, fmt.Errorf("afton: %s", s) }
	raw, err := json.Marshal(cfg)
	if err != nil {
		return fail("configuration")
	}
	cfg, err = artifact.DecodeConfigJSON(raw)
	if err != nil {
		return artifact.Refresh{}, err
	}
	if cfg.Source.ID != "roxy" || cfg.Source.Adapter != "afton" || cfg.Venue.Key != "roxy" || cfg.Venue.Name != "The Roxy Theatre" || cfg.Venue.Timezone != "America/Denver" || cfg.Venue.Website != "https://www.theroxydenver.com/calendar" || len(cfg.AdapterOptions) != 0 || cfg.Venue.AdmissionPolicy != nil || len(cfg.AdmissionRules) != 0 {
		return fail("unsupported configuration")
	}
	if now.IsZero() || len(b) > artifact.MaxDocumentBytes || !utf8.Valid(b) {
		return fail("invalid snapshot")
	}
	var s snapshot
	dec := json.NewDecoder(bytes.NewReader(b))
	if dec.Decode(&s) != nil || dec.Decode(new(any)) != io.EOF {
		return fail("invalid envelope")
	}
	loc, _ := time.LoadLocation(cfg.Venue.Timezone)
	today := now.In(loc)
	if s.From != today.Format("2006-01-02") || s.Through != today.AddDate(1, 0, 0).Format("2006-01-02") {
		return fail("coverage mismatch")
	}
	rows, err := enumerate(s.Pages)
	if err != nil {
		return fail(err.Error())
	}
	check, err := enumerate(s.CheckPages)
	if err != nil || !reflect.DeepEqual(rows, check) {
		return fail("changed listing")
	}
	native := 0
	for _, e := range rows {
		if e.Type == "real_world" {
			native++
		}
	}
	if len(s.Details) != native || len(s.CheckDetails) != native {
		return fail("detail coverage mismatch")
	}
	out := artifact.Refresh{GeneratedAt: now.UTC().Format(time.RFC3339Nano), Coverage: artifact.Coverage{From: s.From, Through: s.Through, Complete: true}, Observations: []artifact.Observation{}}
	for _, e := range rows {
		id, _ := identity(e)
		if e.Type == "real_world" && (s.Details[id] == "" || s.CheckDetails[id] == "") {
			return fail("missing detail")
		}
		a, ae := normalize(e, s.Details[id], loc)
		c, ce := normalize(e, s.CheckDetails[id], loc)
		if !reflect.DeepEqual(a, c) || fmt.Sprint(ae) != fmt.Sprint(ce) {
			return fail("changed event detail")
		}
		if ae == nil && (a.Date < s.From || a.Date > s.Through) {
			continue
		}
		failure := ""
		if ae != nil {
			failure = ae.Error()
		}
		raw, _ := json.Marshal(a)
		out.Observations = append(out.Observations, artifact.Observation{UpstreamID: id, Data: raw, Failure: failure})
	}
	return out, nil
}

type structured struct {
	Type                         string `json:"@type"`
	Name, StartDate, EventStatus string
	Location                     struct {
		Name    string
		Address struct{ StreetAddress, AddressLocality, AddressRegion, PostalCode string }
	}
	Offers    []struct{ URL string }
	Organizer struct{ URL string }
}

func normalize(e record, h string, loc *time.Location) (artifact.EventData, error) {
	d := artifact.EventData{Title: strings.Join(strings.Fields(e.Name), " "), Status: "Scheduled", EventURL: e.Link, TicketURL: e.Link}
	fail := func(s string) (artifact.EventData, error) { return d, fmt.Errorf("%s", s) }
	t, err := time.Parse("2006-01-02 15:04:05", e.Start)
	if err != nil {
		return fail("invalid listing date")
	}
	d.Date = t.Format("2006-01-02")
	if d.Title == "" {
		return fail("missing title")
	}
	if e.Type == "external_event" {
		return d, nil
	}
	if e.Venue != "The Roxy Theatre" || e.City != "Denver" || e.State != "CO" {
		return fail("unreviewed venue")
	}
	root, err := dom.Parse(strings.NewReader(h))
	if err != nil || len(h) > 1024*1024 {
		return fail("invalid detail HTML")
	}
	nodes := find(root, func(n *dom.Node) bool { return n.Data == "script" && attr(n, "type") == "application/ld+json" })
	if len(nodes) != 1 || nodes[0].FirstChild == nil {
		return fail("missing event JSON-LD")
	}
	var events []structured
	if json.Unmarshal([]byte(nodes[0].FirstChild.Data), &events) != nil || len(events) != 1 || events[0].Type != "Event" {
		return fail("invalid event JSON-LD")
	}
	v := events[0]
	if strings.Join(strings.Fields(v.Name), " ") != d.Title {
		return fail("title mismatch")
	}
	a := v.Location.Address
	if v.Location.Name != "The Roxy Theatre" || a.StreetAddress != "2549 Welton St" || a.AddressLocality != "Denver" || (a.AddressRegion != "CO" && a.AddressRegion != "Colorado") || a.PostalCode != "80205" {
		return fail("detail venue mismatch")
	}
	if len(v.Offers) == 0 && v.Organizer.URL != e.Link {
		return fail("missing identity offers")
	}
	for _, o := range v.Offers {
		if o.URL != e.Link {
			return fail("detail link mismatch")
		}
	}
	start, err := time.Parse(time.RFC3339, v.StartDate)
	if err != nil || start.In(loc).Format("2006-01-02 15:04:05") != e.Start {
		return fail("start time mismatch")
	}
	switch v.EventStatus {
	case "https://schema.org/EventScheduled":
	case "https://schema.org/EventCancelled":
		d.Status = "Cancelled"
		d.TicketURL = ""
	default:
		return fail("unknown status")
	}
	if e.Hide != "yes" && e.Hide != "no" {
		return fail("unreviewed time visibility")
	}
	if e.Hide == "no" {
		d.ShowAt = start.In(loc).Format(time.RFC3339)
	}
	if e.DisplayDoors != "yes" && e.DisplayDoors != "no" {
		return fail("unreviewed doors visibility")
	}
	if e.DisplayDoors == "yes" && e.Doors != "" {
		door, err := time.ParseInLocation("2006-01-02 15:04:05", e.Doors, loc)
		if err != nil || door.Format("2006-01-02 15:04:05") != e.Doors || door.Add(time.Hour).Format("2006-01-02 15:04:05") == e.Doors || door.Add(-time.Hour).Format("2006-01-02 15:04:05") == e.Doors || door.Format("2006-01-02") != d.Date || door.After(start) {
			return fail("invalid doors time")
		}
		d.DoorsAt = door.Format(time.RFC3339)
	}
	labels := find(root, func(n *dom.Node) bool { return class(n, "modal-event-info-label") && text(n) == "Age Restriction:" })
	if len(labels) > 1 {
		return fail("missing or duplicate admission label")
	}
	ages := []string{}
	if len(labels) == 1 {
		values := find(labels[0].Parent, func(n *dom.Node) bool { return class(n, "modal-event-info-value") })
		if len(values) != 1 {
			return fail("missing admission value")
		}
		ages = append(ages, text(values[0]))
	}
	for _, n := range find(root, func(n *dom.Node) bool { return class(n, "age-restriction") }) {
		s := text(n)
		if !strings.HasPrefix(s, "Age Restriction:") {
			return fail("unreviewed admission label")
		}
		ages = append(ages, strings.TrimSpace(strings.TrimPrefix(s, "Age Restriction:")))
	}
	if len(ages) == 0 || ages[0] == "" {
		return fail("missing admission value")
	}
	for _, age := range ages {
		if age != ages[0] {
			return fail("conflicting admission values")
		}
	}
	age := ages[0]
	p := &artifact.AdmissionPolicy{Text: age, URL: e.Link}
	switch age {
	case "All Ages & Bar w/ID", "All Ages":
		p.Category = "All ages"
	case "13+", "16+", "18+", "21+":
		p.Category = age
	case "21+ Only":
		p.Category = "21+"
	}
	if p.Category == "All ages" && d.Status == "Scheduled" {
		p.WithAdult = &artifact.AdultAdmission{URL: e.Link, ReviewedOn: "2026-09-12", Ranges: []artifact.AdmissionRange{{MinAge: 0, MaxAge: 17, Condition: "All ages; event ticket requirements apply"}}}
	}
	d.AdmissionPolicy = p
	return d, nil
}

// Reuse the inert DOM traversal convention of the existing HTML adapters.
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
