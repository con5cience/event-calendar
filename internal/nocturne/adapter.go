// Package nocturne normalizes paired monthly Nocturne calendars. It does not fetch or execute HTML.
package nocturne

import (
	"bytes"
	"encoding/json"
	"event-calendar/internal/artifact"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode/utf8"

	ics "github.com/arran4/golang-ical"
	dom "golang.org/x/net/html"
)

type pass struct {
	Policy string
	Months map[string]string
}
type snapshot struct {
	From, Through string
	Pages, Check  pass
}
type record struct {
	ID      string
	Data    artifact.EventData
	Failure string
}

const condition = "Parent/guardian and table reservation required; confirm admission with venue."

var eventPath = regexp.MustCompile(`^/music/[0-9]+-[a-zA-Z0-9-]+/([1-9][0-9]*)-[a-zA-Z0-9-]+$`)
var setPattern = regexp.MustCompile(`(?i)(set\s+(?:1|2|one|two)|(?:first|second)\s+seating\s+set|(?:special\s+)?single\s+set|sunday summer show)\s*[:\-]?\s*(\d{1,2}):(\d{2})\s*(am|pm)?\s*[-–]\s*(\d{1,2}):(\d{2})\s*(am|pm)?`)

// Omitted meridiems inherit the explicitly evening calendar context. Only the
// observed 5–11pm set window is accepted; other schedules require review.
func sets(s string) ([]string, error) {
	matches := setPattern.FindAllStringSubmatch(s, -1)
	if len(matches) < 1 || len(matches) > 2 {
		return nil, fmt.Errorf("unrecognized set schedule")
	}
	result := []string{}
	for i, m := range matches {
		label := strings.ToLower(m[1])
		second := strings.Contains(label, "2") || strings.Contains(label, "two") || strings.Contains(label, "second")
		if second != (i == 1) {
			return nil, fmt.Errorf("duplicate or unordered sets")
		}
		h, _ := strconv.Atoi(m[2])
		minute, _ := strconv.Atoi(m[3])
		end, _ := strconv.Atoi(m[5])
		endMinute, _ := strconv.Atoi(m[6])
		if h < 5 || h > 11 || minute > 59 || end < h || end > 11 || endMinute > 59 || strings.EqualFold(m[4], "am") || strings.EqualFold(m[7], "am") || end*60+endMinute <= h*60+minute {
			return nil, fmt.Errorf("uncertain evening set time")
		}
		result = append(result, fmt.Sprintf("%02d:%02d:00", h+12, minute))
	}
	if len(matches) == 1 && !strings.Contains(strings.ToLower(matches[0][1]), "single") && !strings.EqualFold(matches[0][1], "sunday summer show") && !strings.Contains(strings.ToLower(s), "no second set") {
		return nil, fmt.Errorf("second set not resolved")
	}
	if len(result) == 2 && result[1] <= result[0] {
		return nil, fmt.Errorf("unordered sets")
	}
	return result, nil
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
func plain(s string) string { n, _ := dom.Parse(strings.NewReader(s)); return text(n) }
func ticket(s, date string) string {
	root, _ := dom.Parse(strings.NewReader(s))
	found := map[string]bool{}
	var walk func(*dom.Node)
	walk = func(n *dom.Node) {
		if n.Data == "script" || n.Data == "style" {
			return
		}
		label := text(n)
		for _, a := range n.Attr {
			if a.Key == "title" {
				label += " " + a.Val
			}
		}
		if n.Type == dom.ElementNode && n.Data == "a" && strings.Contains(strings.ToLower(label), "dinner and a show") {
			for _, a := range n.Attr {
				if a.Key != "href" {
					continue
				}
				u, err := url.Parse(a.Val)
				if err == nil && u.Scheme == "https" && u.Host == "www.exploretock.com" && u.User == nil && strings.HasPrefix(u.Path, "/nocturnejazz/experience/") && u.Query().Get("date") == date {
					found[u.String()] = true
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	if len(found) == 1 {
		for u := range found {
			return u
		}
	}
	return ""
}
func unescape(s string) string {
	return strings.NewReplacer(`\n`, "\n", `\N`, "\n", `\,`, ",", `\;`, ";", `\\`, `\`).Replace(s)
}

func Decode(cfg artifact.SourceConfig, b []byte, now time.Time) (artifact.Refresh, error) {
	fail := func(s string) (artifact.Refresh, error) { return artifact.Refresh{}, fmt.Errorf("nocturne: %s", s) }
	raw, err := json.Marshal(cfg)
	if err != nil {
		return fail("configuration")
	}
	cfg, err = artifact.DecodeConfigJSON(raw)
	if err != nil || cfg.Source.Adapter != "nocturne-ical" || cfg.Source.ID != cfg.Venue.Key || cfg.Venue.Website != "https://nocturnejazz.com" || cfg.Venue.Timezone != "America/Denver" || len(cfg.AdapterOptions) != 0 || len(cfg.AdmissionRules) != 0 {
		return fail("unsupported configuration")
	}
	if now.IsZero() || len(b) > artifact.MaxDocumentBytes || !utf8.Valid(b) {
		return fail("invalid snapshot")
	}
	var s snapshot
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if dec.Decode(&s) != nil || dec.Decode(new(any)) != io.EOF {
		return fail("snapshot envelope")
	}
	loc, _ := time.LoadLocation(cfg.Venue.Timezone)
	today := now.In(loc)
	if s.From != today.Format("2006-01-02") || s.Through != today.AddDate(1, 0, 0).Format("2006-01-02") {
		return fail("coverage mismatch")
	}
	rows, err := parse(s.Pages, s.From, s.Through, loc)
	if err != nil {
		return fail(err.Error())
	}
	check, err := parse(s.Check, s.From, s.Through, loc)
	if err != nil || !reflect.DeepEqual(rows, check) {
		return fail("changed or incomplete capture")
	}
	out := artifact.Refresh{GeneratedAt: now.UTC().Format(time.RFC3339Nano), Coverage: artifact.Coverage{From: s.From, Through: s.Through, Complete: true}, Observations: []artifact.Observation{}}
	for _, r := range rows {
		data, _ := json.Marshal(r.Data)
		out.Observations = append(out.Observations, artifact.Observation{UpstreamID: r.ID, Data: data, Failure: r.Failure})
	}
	return out, nil
}

func parse(p pass, from, through string, loc *time.Location) ([]record, error) {
	fail := func(s string) ([]record, error) { return nil, fmt.Errorf("%s", s) }
	pt := plain(p.Policy)
	for _, required := range []string{"adults 21 and older", "between the age of 10 and 21", "parent or guardian", "table reservation we try to accommodate", "We do not allow access to guests under the age of 10"} {
		if !strings.Contains(pt, required) {
			return fail("unreviewed admission policy")
		}
	}
	start, _ := time.Parse("2006-01-02", from[:7]+"-01")
	end, _ := time.Parse("2006-01-02", through[:7]+"-01")
	rows := []record{}
	seen := map[string]bool{}
	months := 0
	for month := start; !month.After(end); month = month.AddDate(0, 1, 0) {
		months++
		raw, ok := p.Months[month.Format("2006-01-02")]
		if !ok {
			return fail("missing month")
		}
		raw = strings.TrimSpace(raw)
		if !strings.HasPrefix(raw, "BEGIN:VCALENDAR") || !strings.HasSuffix(raw, "END:VCALENDAR") || strings.Count(raw, "BEGIN:VCALENDAR") != 1 || strings.Count(raw, "BEGIN:VEVENT") != strings.Count(raw, "END:VEVENT") {
			return fail("incomplete calendar")
		}
		cal, err := ics.ParseCalendar(strings.NewReader(raw))
		if err != nil {
			return fail("invalid iCalendar")
		}
		zone := ""
		for _, p := range cal.CalendarProperties {
			if p.IANAToken == "X-WR-TIMEZONE" {
				zone = p.Value
			}
		}
		if zone != loc.String() {
			return fail("calendar timezone mismatch")
		}
		for _, ev := range cal.Events() {
			fields := map[string]string{}
			tz := ""
			for _, p := range ev.Properties {
				if _, ok := fields[p.IANAToken]; ok {
					return fail("duplicate property")
				}
				fields[p.IANAToken] = unescape(p.Value)
				if p.IANAToken == "DTSTART" {
					tz = strings.Join(p.ICalParameters["TZID"], "")
				}
				switch p.IANAToken {
				case "RRULE", "RDATE", "EXDATE", "RECURRENCE-ID":
					return fail("unsupported event recurrence")
				}
			}
			u, err := url.Parse(fields["URL"])
			if err != nil || u.Scheme != "https" || u.Host != "nocturnejazz.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
				return fail("event URL")
			}
			id := eventPath.FindStringSubmatch(u.Path)
			if len(id) != 2 || fields["UID"] == "" {
				return fail("event identity")
			}
			dt, err := time.ParseInLocation("20060102T150405", fields["DTSTART"], loc)
			if err != nil || tz != loc.String() || dt.Format("20060102T150405") != fields["DTSTART"] || dt.Format("2006-01") != month.Format("2006-01") {
				return fail("event date or zone")
			}
			date := dt.Format("2006-01-02")
			key := id[1] + "-" + date
			if seen[key] {
				return fail("duplicate occurrence")
			}
			seen[key] = true
			if date < from || date > through {
				continue
			}
			title := strings.TrimSpace(fields["SUMMARY"])
			if title == "Holiday Closure" || title == "Closed for a Private Event" {
				continue
			}
			schedule, err := sets(plain(fields["DESCRIPTION"]))
			if title == "" {
				err = fmt.Errorf("missing title")
			}
			if err != nil {
				for i := 1; i <= 2; i++ {
					rows = append(rows, record{ID: fmt.Sprintf("%s-set-%d", key, i), Failure: err.Error()})
				}
				continue
			}
			status := "scheduled"
			if fields["STATUS"] == "CANCELLED" || strings.Contains(strings.ToLower(title), "cancelled") || strings.Contains(strings.ToLower(title), "canceled") {
				status = "cancelled"
			} else if fields["STATUS"] != "" && fields["STATUS"] != "CONFIRMED" {
				return fail("unknown status")
			}
			for i, clock := range schedule {
				show, e := time.ParseInLocation("2006-01-02T15:04:05", date+"T"+clock, loc)
				if e != nil {
					return fail("set clock")
				}
				policy := &artifact.AdmissionPolicy{Text: "21+", Category: "21+", URL: "https://nocturnejazz.com/faq"}
				if status == "scheduled" {
					policy.WithAdult = &artifact.AdultAdmission{URL: policy.URL, ReviewedOn: "2026-09-14", Ranges: []artifact.AdmissionRange{{MinAge: 10, MaxAge: 17, Condition: condition}}}
				}
				rows = append(rows, record{ID: fmt.Sprintf("%s-set-%d", key, i+1), Data: artifact.EventData{Date: date, Title: title, Status: status, ShowAt: show.Format(time.RFC3339), EventURL: u.String(), TicketURL: ticket(fields["DESCRIPTION"], date), AdmissionPolicy: policy}})
			}
		}
	}
	if len(p.Months) != months {
		return fail("unexpected month")
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows, nil
}
