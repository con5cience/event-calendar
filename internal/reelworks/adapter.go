// Package reelworks reads ReelWorks' public WordPress listing and event pages.
// It never fetches URLs; the operator capture supplies paired identical reads.
package reelworks

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

const (
	listingURL = "https://reelworksdenver.com/events/"
	origin     = "https://reelworksdenver.com"
	venueName  = "ReelWorks Denver"
)

var (
	eventLink = regexp.MustCompile(`^https://reelworksdenver\.com/event/([a-z0-9]+(?:-[a-z0-9]+)*)/$`)
	spaces    = regexp.MustCompile(`\s+`)
	ageText   = regexp.MustCompile(`(?i)^(all ages|16|18|21)\+?(?:\s*\(([^)]*)\))?$`)
)

// Ticket hosts the venue's own pages link per event. The venue aggregates
// mixed promoters; an unreviewed host fails the record, not the source.
var ticketHost = map[string]bool{
	"www.axs.com":               true,
	"aftontickets.com":          true,
	"reelworks.ticketsauce.com": true,
	"kyd.to":                    true,
	"shotgun.live":              true,
}

type sidebar struct{ Date, Time, Age string }
type eventPage struct {
	JSONLD  string  `json:"jsonld"`
	Sidebar sidebar `json:"sidebar"`
}
type pages struct {
	Listing string               `json:"listing"`
	FAQ     string               `json:"faq"`
	Events  map[string]eventPage `json:"events"`
}
type record struct {
	ID      string
	Data    artifact.EventData
	Failure string
}
type item struct {
	Slug, Title, DateText, TicketURL string
	Date                             time.Time
}
type musicEvent struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	Status    string `json:"eventStatus"`
	Mode      string `json:"eventAttendanceMode"`
	Location  *struct {
		Name string `json:"name"`
	} `json:"location"`
	Performer json.RawMessage `json:"performer"`
	Offers    *struct {
		URL string `json:"url"`
	} `json:"offers"`
}

func Decode(cfg artifact.SourceConfig, b []byte, now time.Time) (artifact.Refresh, error) {
	fail := func(s string) (artifact.Refresh, error) {
		return artifact.Refresh{}, fmt.Errorf("reelworks: %s", s)
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return fail("configuration")
	}
	cfg, err = artifact.DecodeConfigJSON(raw)
	if err != nil {
		return fail("configuration")
	}
	if cfg.Source.ID != "reelworks" || cfg.Source.Adapter != "reelworks-wordpress-events" ||
		cfg.Venue.Key != "reelworks" || cfg.Venue.Name != venueName ||
		cfg.Venue.Timezone != "America/Denver" || cfg.Venue.Website != listingURL ||
		cfg.Venue.Address != "1399 35th St, Denver, CO 80205" ||
		cfg.Venue.Phone != "+1-303-468-5443" || cfg.Venue.AdmissionPolicy != nil ||
		len(cfg.AdapterOptions) != 0 || len(cfg.AdmissionRules) != 0 || len(cfg.Overrides) != 0 {
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
	rows, err := parse(s.Pages, loc, now)
	if err != nil {
		return fail(err.Error())
	}
	check, err := parse(s.Check, loc, now)
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
func stripTags(html string) string {
	root, err := dom.Parse(strings.NewReader(html))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(spaces.ReplaceAllString(text(root), " "))
}

// The venue states its own policy: the promoter sets the age per show and the
// event page carries it. These FAQ markers anchor that per-event reading.
func unreviewedPolicy(faq string) bool {
	root, err := dom.Parse(strings.NewReader(faq))
	if err != nil {
		return true
	}
	body := strings.ToLower(spaces.ReplaceAllString(text(root), " "))
	return !strings.Contains(body, "depends on the promoter") ||
		!strings.Contains(body, "event page for the show")
}

func parseListing(listing string) ([]item, error) {
	root, err := dom.Parse(strings.NewReader(listing))
	if err != nil {
		return nil, err
	}
	var nodes []*dom.Node
	walk(root, func(n *dom.Node) {
		if n.Type == dom.ElementNode && n.Data == "div" && hasClass(n, "event-item") {
			nodes = append(nodes, n)
		}
	})
	if len(nodes) == 0 || len(nodes) > 200 {
		return nil, fmt.Errorf("missing or capped event listing")
	}
	items := make([]item, 0, len(nodes))
	var last time.Time
	seen := map[string]bool{}
	for _, node := range nodes {
		var slug, title, dateText, ticketURL string
		var h1, h3, h4 int
		var links []string
		// Walk only this item's subtree so page content outside the
		// listing cannot leak into the last event's fields.
		walk(node, func(n *dom.Node) {
			if n.Type != dom.ElementNode {
				return
			}
			switch n.Data {
			case "h1":
				h1++
				title = strings.TrimSpace(text(n))
			case "h3":
				h3++
				dateText = strings.TrimSpace(text(n))
			case "h4":
				h4++
			case "a":
				href := strings.TrimSpace(attribute(n, "href"))
				switch {
				case eventLink.MatchString(href):
					links = append(links, href)
				case attribute(n, "target") == "_blank" && strings.HasPrefix(href, "https://"):
					ticketURL = href
				}
			}
		})
		if h3 != 1 || h1 != 1 || h4 > 1 || len(links) == 0 {
			return nil, fmt.Errorf("unreviewed event listing")
		}
		for _, link := range links {
			m := eventLink.FindStringSubmatch(link)
			if m == nil || (slug != "" && slug != m[1]) {
				return nil, fmt.Errorf("conflicting event link")
			}
			slug = m[1]
		}
		day, err := time.Parse("Monday, Jan 2, 2006", dateText)
		if err != nil || title == "" {
			return nil, fmt.Errorf("unreviewed listing title or date")
		}
		if seen[slug] {
			return nil, fmt.Errorf("duplicate event link")
		}
		seen[slug] = true
		if len(items) > 0 && day.Before(last) {
			return nil, fmt.Errorf("unsorted event listing")
		}
		last = day
		items = append(items, item{
			Slug: slug, Title: title, DateText: dateText,
			TicketURL: ticketURL, Date: day,
		})
	}
	return items, nil
}

func parse(p pages, loc *time.Location, now time.Time) ([]record, error) {
	if unreviewedPolicy(p.FAQ) {
		return nil, fmt.Errorf("unreviewed venue policy")
	}
	items, err := parseListing(p.Listing)
	if err != nil {
		return nil, err
	}
	if len(items) != len(p.Events) {
		return nil, fmt.Errorf("listing and event pages differ")
	}
	listed := map[string]item{}
	for _, it := range items {
		listed[it.Slug] = it
	}
	rows := []record{}
	for slug, page := range p.Events {
		it, ok := listed[slug]
		if !ok {
			return nil, fmt.Errorf("listing and event pages differ")
		}
		r := record{ID: origin + "/event/" + slug + "/"}
		data, ferr := normalize(page, it, loc, now)
		r.Data = data
		if ferr != nil {
			r.Failure = ferr.Error()
		}
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows, nil
}

func clockOn(date, raw string, loc *time.Location) (time.Time, error) {
	clock, err := time.Parse("3:04 PM", spaces.ReplaceAllString(strings.ToUpper(raw), " "))
	if err != nil {
		return time.Time{}, err
	}
	wall := date + "T" + clock.Format("15:04")
	at, err := time.ParseInLocation("2006-01-02T15:04", wall, loc)
	if err != nil || at.Format("2006-01-02T15:04") != wall ||
		at.Add(time.Hour).Format("2006-01-02T15:04") == wall ||
		at.Add(-time.Hour).Format("2006-01-02T15:04") == wall {
		return time.Time{}, fmt.Errorf("invalid or ambiguous Denver clock")
	}
	return at, nil
}

func performered(raw json.RawMessage) []string {
	names := []string{}
	if len(raw) == 0 || string(raw) == "null" {
		return names
	}
	var one struct{ Name string }
	if json.Unmarshal(raw, &one) == nil && strings.TrimSpace(one.Name) != "" {
		return append(names, strings.TrimSpace(one.Name))
	}
	var many []struct{ Name string }
	if json.Unmarshal(raw, &many) == nil {
		for _, p := range many {
			if name := strings.TrimSpace(p.Name); name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}

func agePolicy(raw, eventURL, reviewedOn string) (artifact.AdmissionPolicy, error) {
	policy := artifact.AdmissionPolicy{Text: raw, URL: eventURL}
	m := ageText.FindStringSubmatch(raw)
	if m == nil {
		return policy, fmt.Errorf("unreviewed age")
	}
	if strings.EqualFold(m[1], "all ages") {
		policy.Category = "All ages"
	} else {
		policy.Category = m[1] + "+"
	}
	if strings.TrimSpace(m[2]) == "" {
		return policy, nil
	}
	if !strings.EqualFold(spaces.ReplaceAllString(m[2], " "), "Under 16 w/ adult guardian") {
		return policy, fmt.Errorf("unreviewed age condition")
	}
	policy.WithAdult = &artifact.AdultAdmission{
		URL: eventURL, ReviewedOn: reviewedOn,
		Ranges: []artifact.AdmissionRange{{
			MinAge: 0, MaxAge: 15, Condition: "Under 16 w/ adult guardian",
		}},
	}
	return policy, nil
}

func normalize(page eventPage, it item, loc *time.Location, now time.Time) (artifact.EventData, error) {
	d := artifact.EventData{Status: "Scheduled", EventURL: origin + "/event/" + it.Slug + "/"}
	var g struct {
		Graph []json.RawMessage `json:"@graph"`
	}
	if json.Unmarshal([]byte(page.JSONLD), &g) != nil {
		return d, fmt.Errorf("invalid schema graph")
	}
	var found *musicEvent
	for _, node := range g.Graph {
		var kind struct {
			Type string `json:"@type"`
		}
		if json.Unmarshal(node, &kind) != nil || kind.Type != "MusicEvent" {
			continue
		}
		if found != nil {
			return d, fmt.Errorf("duplicate event schema")
		}
		var ev musicEvent
		if json.Unmarshal(node, &ev) != nil {
			return d, fmt.Errorf("invalid event schema")
		}
		found = &ev
	}
	if found == nil {
		return d, fmt.Errorf("missing event schema")
	}
	ev := *found
	if strings.TrimSpace(ev.Name) == "" || strings.TrimSpace(ev.Name) != it.Title {
		return d, fmt.Errorf("conflicting or missing title")
	}
	d.Title = it.Title
	if strings.TrimSpace(ev.URL) != d.EventURL {
		return d, fmt.Errorf("conflicting event link")
	}
	if ev.Location == nil || !strings.EqualFold(strings.TrimSpace(ev.Location.Name), venueName) {
		return d, fmt.Errorf("unreviewed venue")
	}
	if ev.Mode != "https://schema.org/OfflineEventAttendanceMode" {
		return d, fmt.Errorf("unsupported attendance mode")
	}
	switch ev.Status {
	case "https://schema.org/EventScheduled":
		d.Status = "Scheduled"
	case "https://schema.org/EventCancelled":
		d.Status = "Cancelled"
	default:
		return d, fmt.Errorf("unsupported event status")
	}
	start, err := time.Parse(time.RFC3339, ev.StartDate)
	if err != nil {
		return d, fmt.Errorf("invalid start")
	}
	local := start.In(loc).Truncate(time.Minute)
	_, parsed := start.Zone()
	_, zone := local.Zone()
	if parsed != zone {
		return d, fmt.Errorf("invalid or ambiguous Denver clock")
	}
	end, err := time.Parse(time.RFC3339, ev.EndDate)
	if err != nil || !end.After(start) {
		return d, fmt.Errorf("invalid or missing end")
	}
	endLocal := end.In(loc).Truncate(time.Minute)
	_, endParsed := end.Zone()
	_, endZone := endLocal.Zone()
	if endParsed != endZone {
		return d, fmt.Errorf("invalid or ambiguous Denver clock")
	}
	// The labeled sidebar window and the listing date cross-check the schema
	// times. The venue's own prose calls this window's start the doors time,
	// and no separate show time is published.
	sideDate := strings.SplitN(stripTags(page.Sidebar.Date), " - ", 2)
	if len(sideDate) != 2 {
		return d, fmt.Errorf("unreviewed sidebar date")
	}
	first, err := time.Parse("Monday, Jan 2, 2006", strings.TrimSpace(sideDate[0]))
	second, err2 := time.Parse("Monday, Jan 2, 2006", strings.TrimSpace(sideDate[1]))
	if err != nil || err2 != nil ||
		first.Format("2006-01-02") != local.Format("2006-01-02") ||
		second.Format("2006-01-02") != endLocal.Format("2006-01-02") {
		return d, fmt.Errorf("conflicting date")
	}
	if it.Date.Format("2006-01-02") != local.Format("2006-01-02") {
		return d, fmt.Errorf("conflicting listing date")
	}
	sideTime := strings.SplitN(stripTags(page.Sidebar.Time), " - ", 2)
	if len(sideTime) != 2 {
		return d, fmt.Errorf("unreviewed sidebar time")
	}
	doors, err := clockOn(local.Format("2006-01-02"), sideTime[0], loc)
	if err != nil || !doors.Equal(local) {
		return d, fmt.Errorf("conflicting doors")
	}
	closing, err := clockOn(endLocal.Format("2006-01-02"), sideTime[1], loc)
	if err != nil || !closing.Equal(endLocal) {
		return d, fmt.Errorf("conflicting closing")
	}
	d.Date = local.Format("2006-01-02")
	d.DoorsAt = local.Format(time.RFC3339)
	age := stripTags(page.Sidebar.Age)
	if age == "" {
		return d, fmt.Errorf("missing age")
	}
	policy, err := agePolicy(age, d.EventURL, now.Format("2006-01-02"))
	if err != nil {
		return d, err
	}
	d.AdmissionPolicy = &policy
	d.Performers = performered(ev.Performer)
	// Either the structured offer or the listing button is the venue's own
	// ticket link; when both are present they must agree.
	offersURL := ""
	if ev.Offers != nil {
		offersURL = strings.TrimSpace(ev.Offers.URL)
	}
	if offersURL != "" && it.TicketURL != "" && offersURL != it.TicketURL {
		return d, fmt.Errorf("conflicting ticket link")
	}
	chosen := offersURL
	if chosen == "" {
		chosen = it.TicketURL
	}
	if chosen != "" {
		u, err := url.Parse(chosen)
		if err != nil || u.Scheme != "https" || !ticketHost[u.Host] ||
			u.Path == "" || u.User != nil || u.Fragment != "" {
			return d, fmt.Errorf("unsupported ticket provider")
		}
		d.TicketURL = chosen
	}
	return d, nil
}
