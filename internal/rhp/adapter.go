// Package rhp normalizes captured RHP calendars and event-page HTML. It does
// not fetch URLs or execute scripts. Prices and placeholder end times are ignored.
package rhp

import (
	"bytes"
	"encoding/json"
	"event-calendar/internal/artifact"
	"fmt"
	"html"
	"io"
	"net/url"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode/utf8"

	dom "golang.org/x/net/html"
)

type listing struct{ PlainTitle, URL, Start, VenueName, StrCTAHTML string }
type snapshot struct {
	Calendar struct {
		Success bool
		Data    struct{ Events []listing }
	}
	Details map[string]string
}
type detail struct {
	Type                              string `json:"@type"`
	Name, URL, StartDate, EventStatus string
	Location                          struct{ Name, Address string }
	Offers                            struct{ URL string }
}

func Decode(cfg artifact.SourceConfig, b []byte, now time.Time) (artifact.Refresh, error) {
	fail := func(s string) (artifact.Refresh, error) { return artifact.Refresh{}, fmt.Errorf("rhp: %s", s) }
	raw, err := json.Marshal(cfg)
	if err != nil {
		return fail("invalid configuration")
	}
	cfg, err = artifact.DecodeConfigJSON(raw)
	if err != nil {
		return artifact.Refresh{}, err
	}
	profiles := map[string]struct{ name, origin string }{"lost-lake": {"Lost Lake", "https://lost-lake.com"}, "larimer": {"Larimer Lounge", "https://larimerlounge.com"}, "globe-hall": {"Globe Hall", "https://globehall.com"}, "cervantes": {"Cervantes", "https://cervantesmasterpiece.com"}}
	p, ok := profiles[cfg.Source.ID]
	if !ok || cfg.Source.Adapter != "rhp-calendar" || cfg.Venue.Key != cfg.Source.ID || cfg.Venue.Name != p.name || cfg.Venue.Website != p.origin || cfg.Venue.Timezone != "America/Denver" || len(cfg.AdapterOptions) != 0 {
		return fail("unsupported source configuration")
	}
	if now.IsZero() || len(b) > artifact.MaxDocumentBytes || !utf8.Valid(b) {
		return fail("invalid clock or snapshot size/encoding")
	}
	var s snapshot
	dec := json.NewDecoder(bytes.NewReader(b))
	if dec.Decode(&s) != nil || dec.Decode(new(any)) != io.EOF || !s.Calendar.Success || s.Calendar.Data.Events == nil || s.Details == nil {
		return fail("invalid snapshot or unsuccessful calendar envelope")
	}
	if len(s.Calendar.Data.Events) >= 10000 {
		return fail("calendar reached request cap; completeness unknown")
	}
	loc, _ := time.LoadLocation(cfg.Venue.Timezone)
	today := now.In(loc)
	r := artifact.Refresh{GeneratedAt: now.UTC().Format(time.RFC3339Nano), Coverage: artifact.Coverage{From: today.Format("2006-01-02"), Through: today.AddDate(1, 0, 0).Format("2006-01-02"), Complete: true}, Observations: []artifact.Observation{}}
	seen := map[string]bool{}
	for _, e := range s.Calendar.Data.Events {
		if !eventURL(e.URL, p.origin) || seen[e.URL] {
			return fail("invalid or duplicate event identity")
		}
		seen[e.URL] = true
		d, err := normalize(e, s.Details[e.URL], cfg, loc)
		failure := ""
		if err != nil {
			failure = err.Error()
		}
		if failure == "" && (d.Date < r.Coverage.From || d.Date > r.Coverage.Through) {
			continue
		}
		raw, _ := json.Marshal(d)
		r.Observations = append(r.Observations, artifact.Observation{UpstreamID: e.URL, Data: raw, Failure: failure})
	}
	for u := range s.Details {
		if !seen[u] {
			return fail("detail does not belong to calendar")
		}
	}
	return r, nil
}

func eventURL(s, origin string) bool {
	u, err := url.Parse(s)
	return err == nil && u.User == nil && u.Scheme+"://"+u.Host == origin && strings.HasPrefix(u.Path, "/event/") && len(u.Path) > 7 && u.RawQuery == "" && u.Fragment == ""
}
func ticketURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.User == nil && u.Scheme == "https" && u.Host == "www.etix.com" && strings.HasPrefix(u.Path, "/ticket/p/") && len(u.Path) > 10 && u.Fragment == ""
}
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
func localTime(s string, loc *time.Location) (time.Time, error) {
	const layout = "2006-01-02T15:04:05"
	t, err := time.ParseInLocation(layout, s, loc)
	if err != nil || t.Format(layout) != s {
		return time.Time{}, fmt.Errorf("invalid local time")
	}
	for _, delta := range []time.Duration{-time.Hour, time.Hour} {
		if t.Add(delta).Format(layout) == s {
			return time.Time{}, fmt.Errorf("ambiguous local time")
		}
	}
	return t, nil
}

var clockText = regexp.MustCompile(`(?i)^(?:Doors: (\d{1,2}(?::\d{2})? [ap]m) (?:\| )?)?Show: (\d{1,2}(?::\d{2})? [ap]m)$`)
var venueSlug = regexp.MustCompile(`[^a-z0-9]+`)

const guardianText = "All ages, ticketed guests under 16 ONLY ADMITTED WITH TICKETED GUARDIAN 21+"

// Only reviewed on-site room routes are in the first Cervantes integration.
// External promotions remain rejected observations, never mislabeled on-site.
func cervantesRoom(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 4 || parts[0] != "event" || parts[3] != "denver-colorado" {
		return ""
	}
	return map[string]string{
		"cervantes-other-side":                "Cervantes’ Other Side",
		"cervantes-masterpiece-ballroom":      "Cervantes’ Masterpiece Ballroom",
		"cervantes-and-other-side-dual-venue": "Cervantes’ and Other Side – Dual Venue",
	}[parts[2]]
}

func normalize(e listing, page string, cfg artifact.SourceConfig, loc *time.Location) (artifact.EventData, error) {
	data := artifact.EventData{EventURL: e.URL, Title: strings.TrimSpace(html.UnescapeString(e.PlainTitle)), Status: "Scheduled"}
	bad := func(s string) (artifact.EventData, error) { return data, fmt.Errorf("rhp: %s", s) }
	start, err := localTime(e.Start, loc)
	if err != nil {
		return bad("invalid or ambiguous calendar time")
	}
	data.Date = start.Format("2006-01-02")
	data.ShowAt = start.Format(time.RFC3339)
	if cfg.Source.ID == "cervantes" && cervantesRoom(e.URL) == "" {
		return bad("deferred external venue: Cervantes on-site scope only")
	}
	if page == "" {
		return bad("missing event detail")
	}
	tree, err := dom.Parse(strings.NewReader(page))
	if err != nil {
		return bad("invalid event HTML")
	}
	var d detail
	count := 0
	age, times := "", ""
	guardian := false
	for n := range tree.Descendants() {
		if n.Type != dom.ElementNode {
			continue
		}
		if n.Data == "script" && attr(n, "type") == "application/ld+json" && n.FirstChild != nil {
			var candidate detail
			if json.Unmarshal([]byte(n.FirstChild.Data), &candidate) == nil && candidate.Type == "Event" {
				d = candidate
				count++
			}
		}
		if class(n, "eventAgeRestriction") {
			v := text(n)
			if age != "" && age != v {
				return bad("conflicting age restrictions")
			}
			age = v
		}
		if class(n, "eventDoorStartDate") {
			v := text(n)
			if times != "" && times != v {
				return bad("conflicting doors/show times")
			}
			times = v
		}
		if n.Data == "li" && text(n) == guardianText {
			for parent := n.Parent; parent != nil; parent = parent.Parent {
				if class(parent, "singleEventDescription") || class(parent, "eventDescription") {
					guardian = true
				}
			}
		}
	}
	if count != 1 || d.URL != e.URL || strings.TrimSpace(html.UnescapeString(d.Name)) != data.Title || data.Title == "" {
		return bad("missing or conflicting Event JSON-LD")
	}
	dt, err := time.Parse("2006-01-02T15:04:05-0700", d.StartDate)
	if err != nil {
		dt, err = time.Parse(time.RFC3339, d.StartDate)
	}
	if err != nil || !dt.Equal(start) || dt.Format("-07:00") != start.Format("-07:00") {
		return bad("calendar/detail start conflict")
	}
	if times != "" {
		m := clockText.FindStringSubmatch(times)
		if m == nil {
			return bad("unsupported doors/show text")
		}
		parse := func(s string) (time.Time, error) {
			layout := "3 pm"
			if strings.Contains(s, ":") {
				layout = "3:04 pm"
			}
			t, e := time.Parse(layout, strings.ToLower(s))
			if e != nil {
				return t, e
			}
			return localTime(data.Date+"T"+t.Format("15:04:05"), loc)
		}
		show, e := parse(m[2])
		if e != nil || !show.Equal(start) {
			return bad("visible show time conflicts")
		}
		if m[1] != "" {
			door, e := parse(m[1])
			if e != nil || door.After(start) {
				return bad("invalid doors time")
			}
			data.DoorsAt = door.Format(time.RFC3339)
		}
	}
	venue := strings.TrimSpace(html.UnescapeString(d.Location.Name))
	if venue == "" {
		return bad("missing detail venue")
	}
	if e.VenueName != "" {
		v, _ := dom.Parse(strings.NewReader(e.VenueName))
		if text(v) != venue {
			return bad("calendar/detail venue conflict")
		}
	}
	if cfg.Source.ID == "cervantes" {
		if venue != cervantesRoom(e.URL) {
			return bad("Cervantes room route/detail mismatch")
		}
		venue = cfg.Venue.Name
	}
	if venue != cfg.Venue.Name {
		key := strings.Trim(venueSlug.ReplaceAllString(strings.ToLower(venue), "-"), "-")
		if key == "" {
			return bad("unsupported venue name")
		}
		data.Venue = &artifact.Venue{Key: key, Name: venue, Timezone: loc.String(), Address: strings.TrimSpace(d.Location.Address)}
	}
	switch strings.ToLower(strings.TrimSuffix(d.EventStatus, "/")) {
	case "", "https://schema.org/eventscheduled", "http://schema.org/eventscheduled":
	case "https://schema.org/eventcancelled", "http://schema.org/eventcancelled":
		data.Status = "Cancelled"
	default:
		return bad("unsupported event status")
	}
	cta, _ := dom.Parse(strings.NewReader(e.StrCTAHTML))
	for n := range cta.Descendants() {
		if n.Type != dom.ElementNode {
			continue
		}
		if class(n, "cancelled") || class(n, "canceled") || class(n, "Canceled") || class(n, "Cancelled") {
			data.Status = "Cancelled"
		}
		if n.Data == "a" {
			u := attr(n, "href")
			if u != "" {
				if !ticketURL(u) {
					return bad("unsupported CTA ticket URL")
				}
				if data.TicketURL != "" && data.TicketURL != u {
					return bad("conflicting CTA tickets")
				}
				data.TicketURL = u
			}
		}
	}
	if d.Offers.URL != "" {
		if !ticketURL(d.Offers.URL) {
			return bad("unsupported detail ticket URL")
		}
		if data.TicketURL != "" && data.TicketURL != d.Offers.URL {
			return bad("calendar/detail ticket conflict")
		}
		data.TicketURL = d.Offers.URL
	}
	categories := map[string]string{"All Ages": "All ages", "All ages": "All ages", "Ages 13 and up": "13+", "Ages 16 and up": "16+", "Ages 18 and up": "18+", "Ages 21 and up": "21+"}
	category := categories[age]
	if age == "" && guardian && cfg.Source.ID != "cervantes" {
		age = guardianText
		category = "16+"
	}
	if age != "" {
		data.AdmissionPolicy = &artifact.AdmissionPolicy{Text: age, Category: category, URL: e.URL}
		// Cervantes has its own reviewed FAQ rules (minimum age 14 for the
		// 16+ exception), applied by reconciliation and overridden by config.
		if cfg.Source.ID == "cervantes" {
			return data, nil
		}
		// The reviewed exact event condition is narrower than general FAQ prose.
		// Do not infer exceptions for 18+/21+, unknown labels, or off-site venues.
		if guardian && (category == "16+" || category == "All ages") && data.Venue == nil {
			data.AdmissionPolicy.WithAdult = &artifact.AdultAdmission{URL: e.URL, ReviewedOn: "2026-09-10", Ranges: []artifact.AdmissionRange{
				{MinAge: 0, MaxAge: 15, Condition: "Ticketed guardian aged 21+ required"},
				{MinAge: 16, MaxAge: 17, Condition: "Permitted at this age"},
			}}
		} else if category == "All ages" && data.Venue == nil {
			data.AdmissionPolicy.WithAdult = &artifact.AdultAdmission{URL: e.URL, ReviewedOn: "2026-09-10", Ranges: []artifact.AdmissionRange{{MinAge: 0, MaxAge: 17, Condition: "All ages event; attend with adult and follow event conditions"}}}
		}
	}
	return data, nil
}
