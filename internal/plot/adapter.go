// Package plot normalizes operator-captured Hi-Dive listings without fetching.
package plot

import (
	"bytes"
	"encoding/json"
	"event-calendar/internal/artifact"
	"fmt"
	"html"
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

type snapshot struct {
	PageSize   int                 `json:"page_size"`
	Pages      [][]json.RawMessage `json:"pages"`
	Terminal   []json.RawMessage   `json:"terminal"`
	FirstCheck []json.RawMessage   `json:"first_check"`
}
type record struct {
	ID                                             int64
	MaxPages                                       int
	Title, Day, Doors, URL, Permalink, Description string
	Venue                                          json.RawMessage
	IsMultiDay                                     bool
	Ticket                                         *struct{ Link string }
	Lineup                                         *struct{ Standard []struct{ Title string } }
}

func Decode(cfg artifact.SourceConfig, b []byte, now time.Time) (artifact.Refresh, error) {
	fail := func(s string) (artifact.Refresh, error) { return artifact.Refresh{}, fmt.Errorf("plot: %s", s) }
	raw, err := json.Marshal(cfg)
	if err != nil {
		return fail("invalid configuration")
	}
	cfg, err = artifact.DecodeConfigJSON(raw)
	if err != nil {
		return artifact.Refresh{}, err
	}
	if cfg.Source.ID != "hi-dive" || cfg.Source.Adapter != "plot-listings" || cfg.Venue.Key != "hi-dive" || cfg.Venue.Name != "Hi-Dive" || cfg.Venue.Website != "https://hi-dive.com" || cfg.Venue.Timezone != "America/Denver" || len(cfg.AdapterOptions) != 0 {
		return fail("unsupported configuration")
	}
	if now.IsZero() || len(b) > artifact.MaxDocumentBytes || !utf8.Valid(b) {
		return fail("invalid clock or snapshot")
	}
	var s snapshot
	dec := json.NewDecoder(bytes.NewReader(b))
	if dec.Decode(&s) != nil || dec.Decode(new(any)) != io.EOF || s.PageSize != 5 || len(s.Pages) < 1 || len(s.Pages) > 200 || s.Terminal == nil || len(s.Terminal) != 0 || s.FirstCheck == nil {
		return fail("invalid snapshot envelope")
	}
	var first, check any
	p, _ := json.Marshal(s.Pages[0])
	q, _ := json.Marshal(s.FirstCheck)
	if json.Unmarshal(p, &first) != nil || json.Unmarshal(q, &check) != nil || !reflect.DeepEqual(first, check) {
		return fail("first page changed")
	}
	seen := map[int64]bool{}
	loc, _ := time.LoadLocation(cfg.Venue.Timezone)
	today := now.In(loc)
	r := artifact.Refresh{GeneratedAt: now.UTC().Format(time.RFC3339Nano), Coverage: artifact.Coverage{From: today.Format("2006-01-02"), Through: today.AddDate(1, 0, 0).Format("2006-01-02"), Complete: true}, Observations: []artifact.Observation{}}
	for i, page := range s.Pages {
		if page == nil || len(page) > s.PageSize || (i < len(s.Pages)-1 && len(page) != s.PageSize) || (len(s.Pages) > 1 && len(page) == 0) {
			return fail("missing page records")
		}
		for _, raw := range page {
			var identity struct {
				ID       int64
				MaxPages int
			}
			if json.Unmarshal(raw, &identity) != nil || identity.ID < 1 || identity.ID > 9007199254740991 || seen[identity.ID] || identity.MaxPages != len(s.Pages) {
				return fail("invalid identity or pagination")
			}
			seen[identity.ID] = true
			data, err := normalize(raw, loc)
			failure := ""
			if err != nil {
				failure = err.Error()
			}
			if failure == "" && (data.Date < r.Coverage.From || data.Date > r.Coverage.Through) {
				continue
			}
			value, _ := json.Marshal(data)
			r.Observations = append(r.Observations, artifact.Observation{UpstreamID: strconv.FormatInt(identity.ID, 10), Data: value, Failure: failure})
		}
	}
	return r, nil
}

var age = regexp.MustCompile(`(?i)\bthis is (?:an? )?(13|16|18|21)\s*\+`)
var cancelled = regexp.MustCompile(`(?i)\b(cancelled|canceled)\b`)
var doorsPattern = regexp.MustCompile(`(?i)^Doors:\s*([0-9]{1,2}(?::[0-9]{2})?\s*[ap]m)$`)

func normalize(raw []byte, loc *time.Location) (artifact.EventData, error) {
	var e record
	data := artifact.EventData{Status: "Scheduled"}
	if json.Unmarshal(raw, &e) != nil {
		return data, fmt.Errorf("invalid record fields")
	}
	data.Title = strings.TrimSpace(html.UnescapeString(e.Title))
	if e.IsMultiDay {
		return data, fmt.Errorf("multi-day listing requires review")
	}
	if len(e.Venue) > 0 && string(e.Venue) != "null" {
		return data, fmt.Errorf("off-site venue requires review")
	}
	u, err := url.Parse(e.URL)
	if err != nil || u.User != nil || u.Scheme != "https" || u.Host != "hi-dive.com" || !strings.HasPrefix(u.Path, "/listing/") || len(u.Path) <= 9 || u.RawQuery != "" || u.Fragment != "" || e.Permalink != e.URL {
		return data, fmt.Errorf("invalid venue event URL")
	}
	data.EventURL = e.URL
	date, err := time.ParseInLocation("20060102", e.Day, loc)
	if err != nil || date.Format("20060102") != e.Day {
		return data, fmt.Errorf("invalid event date")
	}
	data.Date = date.Format("2006-01-02")
	if strings.TrimSpace(e.Doors) != "" {
		match := doorsPattern.FindStringSubmatch(strings.TrimSpace(e.Doors))
		if match == nil {
			return data, fmt.Errorf("invalid doors time")
		}
		clock := strings.ToUpper(strings.ReplaceAll(match[1], " ", ""))
		layout := "2006-01-02 3PM"
		if strings.Contains(clock, ":") {
			layout = "2006-01-02 3:04PM"
		}
		at, err := time.ParseInLocation(layout, data.Date+" "+clock, loc)
		if err != nil {
			return data, fmt.Errorf("invalid doors time")
		}
		data.DoorsAt = at.Format(time.RFC3339)
	}
	if e.Ticket != nil {
		data.TicketURL = e.Ticket.Link
	}
	if e.Lineup != nil {
		for _, p := range e.Lineup.Standard {
			if title := strings.TrimSpace(html.UnescapeString(p.Title)); title != "" {
				data.Performers = append(data.Performers, title)
			}
		}
	}
	if cancelled.MatchString(data.Title) {
		data.Status = "Cancelled"
	}
	if matches := age.FindAllStringSubmatch(html.UnescapeString(e.Description), -1); len(matches) > 0 {
		category := matches[0][1] + "+"
		for _, m := range matches {
			if m[1]+"+" != category {
				return data, fmt.Errorf("conflicting admission restrictions")
			}
		}
		data.AdmissionPolicy = &artifact.AdmissionPolicy{Text: category, Category: category, URL: e.URL}
	}
	return data, nil
}
