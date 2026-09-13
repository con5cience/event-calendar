package kse

import (
	"bytes"
	"encoding/json"
	"event-calendar/internal/artifact"
	"fmt"
	dom "golang.org/x/net/html"
	"html"
	"io"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const ballPolicyURL = "https://www.ballarena.com/arena-information/arena-policies-faq/"

type ballRow struct {
	Title, URL, Start string
	End               *string
}
type ballListing struct{ Title, Date, Genre, Status, Ticket, Detail string }

var ballIDPattern = regexp.MustCompile(`^[A-Fa-f0-9]{16}$`)

func ballID(link string) (string, error) {
	u, err := url.Parse(link)
	if err != nil || u.Scheme != "https" || u.Host != "www.ticketmaster.com" || u.User != nil {
		return "", fmt.Errorf("invalid ticket URL")
	}
	parts := strings.Split(u.Path, "/")
	if len(parts) < 3 || parts[len(parts)-2] != "event" || !ballIDPattern.MatchString(parts[len(parts)-1]) {
		return "", fmt.Errorf("missing ticket identity")
	}
	return parts[len(parts)-1], nil
}
func ballAttr(n *dom.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func ballHasClass(n *dom.Node, class string) bool {
	for _, v := range strings.Fields(ballAttr(n, "class")) {
		if v == class {
			return true
		}
	}
	return false
}
func ballText(n *dom.Node) string {
	if n.Type == dom.TextNode {
		return n.Data
	}
	if n.Data == "script" || n.Data == "style" {
		return ""
	}
	s := ""
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		s += ballText(c)
	}
	return s
}
func ballClean(s string) string { return strings.Join(strings.Fields(html.UnescapeString(s)), " ") }
func ballListings(body string) (map[string]ballListing, error) {
	root, err := dom.Parse(strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	out := map[string]ballListing{}
	var failure error
	var walk func(*dom.Node)
	walk = func(n *dom.Node) {
		if ballHasClass(n, "card-wrap") {
			card := ballListing{Genre: ballAttr(n, "data-subgenre")}
			statusFound := false
			var read func(*dom.Node)
			read = func(x *dom.Node) {
				switch {
				case ballHasClass(x, "card-title"):
					card.Title = ballClean(ballText(x))
				case ballHasClass(x, "datetime"):
					card.Date = ballClean(ballText(x))
				case ballHasClass(x, "status-code"):
					card.Status = ballClean(ballText(x))
					statusFound = true
				}
				if x.Type == dom.ElementNode && x.Data == "a" {
					switch ballClean(ballText(x)) {
					case "Find Tickets":
						card.Ticket = ballAttr(x, "href")
					case "More Info":
						card.Detail = ballAttr(x, "href")
					}
				}
				for c := x.FirstChild; c != nil; c = c.NextSibling {
					read(c)
				}
			}
			read(n)
			id, err := ballID(card.Ticket)
			if err != nil || !statusFound || card.Title == "" || card.Date == "" {
				failure = fmt.Errorf("invalid listing card")
				return
			}
			if _, ok := out[id]; ok {
				failure = fmt.Errorf("duplicate listing identity")
				return
			}
			out[id] = card
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	if failure != nil {
		return nil, failure
	}
	if len(out) == 0 || len(out) >= 1000 {
		return nil, fmt.Errorf("missing or capped listing cards")
	}
	return out, nil
}

func DecodeBall(cfg artifact.SourceConfig, b []byte, now time.Time) (artifact.Refresh, error) {
	fail := func(s string) (artifact.Refresh, error) { return artifact.Refresh{}, fmt.Errorf("kse-calendar: %s", s) }
	raw, err := json.Marshal(cfg)
	if err != nil {
		return fail("invalid config")
	}
	cfg, err = artifact.DecodeConfigJSON(raw)
	if err != nil {
		return artifact.Refresh{}, err
	}
	if cfg.Source.ID != "ball-arena" || cfg.Source.Adapter != "kse-calendar" || cfg.Venue.Key != "ball-arena" || cfg.Venue.Name != "Ball Arena" || cfg.Venue.Timezone != "America/Denver" || cfg.Venue.Website != "https://www.ballarena.com" || len(cfg.AdapterOptions) != 0 {
		return fail("unsupported config")
	}
	if now.IsZero() || len(b) > artifact.MaxDocumentBytes || !utf8.Valid(b) {
		return fail("invalid snapshot or clock")
	}
	var s struct {
		Events, Check []json.RawMessage
		Listing       string
		ListingCheck  string `json:"listing_check"`
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	if dec.Decode(&s) != nil || dec.Decode(new(any)) != io.EOF || s.Events == nil || s.Check == nil || len(s.Events) >= 1000 {
		return fail("invalid envelope")
	}
	var a, z any
	p, _ := json.Marshal(s.Events)
	q, _ := json.Marshal(s.Check)
	json.Unmarshal(p, &a)
	json.Unmarshal(q, &z)
	if !reflect.DeepEqual(a, z) {
		return fail("calendar changed")
	}
	cards, err := ballListings(s.Listing)
	if err != nil {
		return artifact.Refresh{}, err
	}
	check, err := ballListings(s.ListingCheck)
	if err != nil || !reflect.DeepEqual(cards, check) {
		return fail("listing changed or incomplete")
	}
	loc, _ := time.LoadLocation(cfg.Venue.Timezone)
	today := now.In(loc)
	r := artifact.Refresh{GeneratedAt: now.UTC().Format(time.RFC3339Nano), Coverage: artifact.Coverage{From: today.Format("2006-01-02"), Through: today.AddDate(1, 0, 0).Format("2006-01-02"), Complete: true}, Observations: []artifact.Observation{}}
	seen := map[string]bool{}
	for _, raw := range s.Events {
		var identity struct{ URL string }
		if json.Unmarshal(raw, &identity) != nil {
			return fail("invalid identity")
		}
		id, err := ballID(identity.URL)
		if err != nil || seen[id] {
			return fail("invalid or duplicate identity")
		}
		seen[id] = true
		card, ok := cards[id]
		if !ok {
			return fail("calendar event missing from listing")
		}
		var e ballRow
		d := artifact.EventData{}
		if err = json.Unmarshal(raw, &e); err == nil {
			d, err = normalizeBall(e, card, loc)
		}
		failure := ""
		if err != nil {
			failure = err.Error()
		}
		if failure == "" && (d.Date < r.Coverage.From || d.Date > r.Coverage.Through) {
			continue
		}
		data, _ := json.Marshal(d)
		r.Observations = append(r.Observations, artifact.Observation{UpstreamID: id, Data: data, Failure: failure})
	}
	for id, card := range cards {
		if !seen[id] {
			if card.Date != "TBA" && card.Date != "TBD" {
				return fail("dated listing missing from calendar")
			}
			r.Observations = append(r.Observations, artifact.Observation{UpstreamID: id, Failure: "event date is TBA"})
		}
	}
	return r, nil
}

func normalizeBall(e ballRow, c ballListing, loc *time.Location) (artifact.EventData, error) {
	d := artifact.EventData{Title: ballClean(e.Title), TicketURL: e.URL, EventURL: e.URL, Status: "Scheduled"}
	if d.Title == "" || d.Title != c.Title {
		return d, fmt.Errorf("title mismatch")
	}
	if c.Detail != "" {
		u, err := url.Parse(c.Detail)
		if err != nil {
			return d, err
		}
		u = (&url.URL{Scheme: "https", Host: "www.ballarena.com"}).ResolveReference(u)
		if u.Scheme != "https" || u.Host != "www.ballarena.com" || u.User != nil || !strings.HasPrefix(u.Path, "/event-pages/") {
			return d, fmt.Errorf("invalid venue detail URL")
		}
		d.EventURL = u.String()
	}
	layout := "2006-01-02T15:04:05"
	if len(e.Start) == 10 {
		layout = "2006-01-02"
	}
	at, err := time.ParseInLocation(layout, e.Start, loc)
	if err != nil || at.Format(layout) != e.Start {
		return d, fmt.Errorf("invalid start")
	}
	d.Date = at.Format("2006-01-02")
	label := at.Format("Mon • Jan 2 2006 • 3:04 PM")
	if layout == "2006-01-02" {
		label = at.Format("Mon • Jan 2 2006")
	} else {
		d.ShowAt = at.Format(time.RFC3339)
	}
	if e.End != nil && *e.End != "" {
		end, err := time.ParseInLocation("2006-01-02", *e.End, loc)
		if err != nil || !end.After(at) || layout != "2006-01-02" {
			return d, fmt.Errorf("unsupported end")
		}
		label = at.Format("Mon • Jan 2 2006") + " - " + end.AddDate(0, 0, -1).Format("Mon • Jan 2 2006")
	}
	if label != c.Date {
		return d, fmt.Errorf("calendar/listing date mismatch")
	}
	switch strings.ToLower(c.Status) {
	case "", "scheduled", "rescheduled", "postponed", "sold out":
	case "cancelled", "canceled":
		d.Status = "Cancelled"
	default:
		return d, fmt.Errorf("unrecognized listing status")
	}
	game := (c.Genre == "NBA" && strings.HasPrefix(d.Title, "Denver Nuggets vs.")) || (c.Genre == "NHL" && strings.HasPrefix(d.Title, "Colorado Avalanche vs.")) || (c.Genre == "Lacrosse" && strings.HasPrefix(d.Title, "Colorado Mammoth vs."))
	if game {
		d.AdmissionPolicy = &artifact.AdmissionPolicy{Text: "All ages; ticket required from age 3", Category: "All ages", URL: ballPolicyURL, WithAdult: &artifact.AdultAdmission{URL: ballPolicyURL, ReviewedOn: "2026-09-11", Ranges: []artifact.AdmissionRange{{MinAge: 0, MaxAge: 2, Condition: "Must sit in a ticketed guest's lap"}, {MinAge: 3, MaxAge: 17, Condition: "Ticket required"}}}}
	}
	return d, nil
}
