// Package seventhcircle reads paired public listing, detail and policy HTML.
package seventhcircle

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

type pages struct {
	Calendar, Policy string
	Details          map[string]string
}
type record struct {
	ID      string
	Data    artifact.EventData
	Failure string
}

var postID = regexp.MustCompile(`^post_([1-9][0-9]*)$`)
var doorSuffix = regexp.MustCompile(`(?i)\s+(\d{1,2}(?::\d{2})?\s*(?:am|pm))\s+doors!?$`)
var ageText = regexp.MustCompile(`(?i)\b(\d{1,2})\s*\+|\b(?:adults? only|age restricted)\b`)
var cancelled = regexp.MustCompile(`(?i)\bcancelled\b|\bcanceled\b`)

func Decode(cfg artifact.SourceConfig, b []byte, now time.Time) (artifact.Refresh, error) {
	fail := func(s string) (artifact.Refresh, error) {
		return artifact.Refresh{}, fmt.Errorf("seventhcircle: %s", s)
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return fail("configuration")
	}
	cfg, err = artifact.DecodeConfigJSON(raw)
	if err != nil {
		return fail("configuration")
	}
	origin, e := url.Parse(cfg.Venue.Website)
	if cfg.Source.Adapter != "html-seventh-circle" || cfg.Source.ID != cfg.Venue.Key || e != nil || origin.Scheme != "https" || origin.Host == "" || origin.User != nil || origin.Path != "" || len(cfg.AdapterOptions) != 0 {
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
	rows, err := parse(s.Pages, cfg, loc)
	if err != nil {
		return fail(err.Error())
	}
	check, err := parse(s.Check, cfg, loc)
	if err != nil || !reflect.DeepEqual(rows, check) {
		return fail("incomplete or changed capture")
	}
	out := artifact.Refresh{GeneratedAt: now.UTC().Format(time.RFC3339Nano), Coverage: artifact.Coverage{From: s.From, Through: s.Through, Complete: true}, Observations: []artifact.Observation{}}
	for _, r := range rows {
		if r.Failure == "" && (r.Data.Date < s.From || r.Data.Date > s.Through) {
			continue
		}
		b, _ := json.Marshal(r.Data)
		out.Observations = append(out.Observations, artifact.Observation{UpstreamID: r.ID, Data: b, Failure: r.Failure})
	}
	return out, nil
}

// Reuse the HTML adapters' inert DOM traversal convention; selectors stay local.
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
func nodes(n *dom.Node, match func(*dom.Node) bool) []*dom.Node {
	var result []*dom.Node
	var walk func(*dom.Node)
	walk = func(n *dom.Node) {
		if n.Data == "script" || n.Data == "style" {
			return
		}
		if n.Type == dom.ElementNode && match(n) {
			result = append(result, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return result
}
func one(n *dom.Node, k string) (string, error) {
	v := nodes(n, func(n *dom.Node) bool { return class(n, k) })
	if len(v) != 1 {
		return "", fmt.Errorf("missing or duplicate %s", k)
	}
	s := text(v[0])
	if s == "" {
		return "", fmt.Errorf("empty %s", k)
	}
	return s, nil
}

func parse(p pages, cfg artifact.SourceConfig, loc *time.Location) ([]record, error) {
	policy, _ := dom.Parse(strings.NewReader(p.Policy))
	pt := text(policy)
	if !strings.Contains(pt, "Seventh Circle is committed to welcoming all ages at every show.") || !strings.Contains(pt, "$5 a year for membership") {
		return nil, fmt.Errorf("unreviewed venue policy")
	}
	root, err := dom.Parse(strings.NewReader(p.Calendar))
	if err != nil {
		return nil, err
	}
	grids := nodes(root, func(n *dom.Node) bool { return attr(n, "id") == "posts" })
	cards := nodes(root, func(n *dom.Node) bool { return class(n, "post-container") })
	if len(grids) != 1 || len(cards) == 0 || len(cards) > 500 || !strings.Contains(text(root), "upcoming events") {
		return nil, fmt.Errorf("unknown listing layout or empty calendar")
	}
	if len(nodes(grids[0], func(n *dom.Node) bool { return class(n, "post-container") })) != len(cards) {
		return nil, fmt.Errorf("unscoped event cards")
	}
	for _, a := range nodes(root, func(n *dom.Node) bool { return n.Data == "a" }) {
		h := attr(a, "href")
		if strings.Contains(h, "page=") || strings.Contains(h, "/page/") || attr(a, "rel") == "next" {
			return nil, fmt.Errorf("unreviewed pagination")
		}
	}
	if len(p.Details) != len(cards) {
		return nil, fmt.Errorf("detail coverage mismatch")
	}
	seen := map[string]bool{}
	rows := []record{}
	for _, card := range cards {
		m := postID.FindStringSubmatch(attr(card, "id"))
		if m == nil || seen[m[1]] {
			return nil, fmt.Errorf("invalid or duplicate identity")
		}
		id := m[1]
		seen[id] = true
		anchors := nodes(card, func(n *dom.Node) bool { return class(n, "post-link") })
		if len(anchors) != 1 || attr(anchors[0], "href") != "/posts/"+id {
			return nil, fmt.Errorf("invalid event link")
		}
		detail, ok := p.Details[id]
		if !ok {
			return nil, fmt.Errorf("missing detail observation")
		}
		r := record{ID: id}
		data, rerr := normalize(card, detail, cfg, loc, id)
		r.Data = data
		if rerr != nil {
			r.Failure = rerr.Error()
		}
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows, nil
}
func normalize(card *dom.Node, detail string, cfg artifact.SourceConfig, loc *time.Location, id string) (artifact.EventData, error) {
	d := artifact.EventData{Status: "Scheduled", EventURL: cfg.Venue.Website + "/posts/" + id}
	title, err := one(card, "event-title")
	if err != nil {
		return d, err
	}
	d.Title = title
	if detail == "" {
		return d, fmt.Errorf("detail unavailable")
	}
	root, err := dom.Parse(strings.NewReader(detail))
	if err != nil {
		return d, err
	}
	dt, err := one(root, "event-title")
	if err != nil || dt != title {
		return d, fmt.Errorf("listing/detail title mismatch")
	}
	stamp, err := one(root, "event-time")
	if err != nil {
		return d, err
	}
	parts := strings.Split(stamp, " @ ")
	if len(parts) != 2 {
		return d, fmt.Errorf("unreviewed detail date")
	}
	day, err := time.ParseInLocation("January 02, 2006", parts[0], loc)
	if err != nil {
		return d, fmt.Errorf("invalid full date")
	}
	d.Date = day.Format("2006-01-02")
	for _, pair := range []struct{ key, want string }{{"date", day.Format("02")}, {"month", day.Format("Jan")}, {"day", day.Format("Mon")}, {"time", parts[1]}} {
		v, e := one(card, pair.key)
		if e != nil || v != pair.want {
			return d, fmt.Errorf("listing/detail date or time mismatch")
		}
	}
	// Printed times without an explicit doors label are intentionally not published.
	clock, err := time.Parse("3:04 PM", parts[1])
	if err != nil {
		return d, fmt.Errorf("invalid clock")
	}
	if m := doorSuffix.FindStringSubmatch(title); m != nil {
		raw := strings.ToUpper(strings.ReplaceAll(m[1], " ", ""))
		layout := "3PM"
		if strings.Contains(raw, ":") {
			layout = "3:04PM"
		}
		door, e := time.Parse(layout, raw)
		if e != nil || door.Format("15:04") != clock.Format("15:04") {
			return d, fmt.Errorf("conflicting doors")
		}
		wall := d.Date + " " + door.Format("15:04")
		at, e := time.ParseInLocation("2006-01-02 15:04", wall, loc)
		if e != nil || at.Format("2006-01-02 15:04") != wall || at.Add(time.Hour).Format("2006-01-02 15:04") == wall || at.Add(-time.Hour).Format("2006-01-02 15:04") == wall {
			return d, fmt.Errorf("invalid or ambiguous doors")
		}
		d.DoorsAt = at.Format(time.RFC3339)
		d.Title = strings.TrimSpace(doorSuffix.ReplaceAllString(title, ""))
	}
	if cancelled.MatchString(title) {
		d.Status = "Cancelled"
		d.AdmissionPolicy = &artifact.AdmissionPolicy{Text: "Cancelled; check venue", URL: d.EventURL}
	}
	if ageText.MatchString(title) {
		d.AdmissionPolicy = &artifact.AdmissionPolicy{Text: title, URL: d.EventURL}
	}
	return d, nil
}
