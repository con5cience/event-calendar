// Package livenation decodes browser-captured Live Nation venue event pages.
package livenation

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

type record struct {
	ID                        string `json:"tm_id"`
	Name, Type, URL, Timezone string
	DataType                  string `json:"event_data_type"`
	Date                      string `json:"start_date_local"`
	Time                      string `json:"start_time_local"`
	UTC                       string `json:"start_datetime_utc"`
	Status                    string `json:"status_code"`
	Info                      string `json:"important_info"`
	Multi                     bool   `json:"span_multiple_days"`
	Venue                     struct {
		ID string `json:"discovery_id"`
	}
	Artists []struct{ Name string }
}

type venueProfile struct {
	name, website string
	ids           map[string]bool
}

var profiles = map[string]venueProfile{
	"fillmore": {"Fillmore Auditorium", "https://www.fillmoredenver.com", map[string]bool{"KovZpZAE6eJA": true}},
	"marquis":  {"Marquis", "https://www.marquisdenver.com", map[string]bool{"KovZpZAJeFkA": true}},
	"summit":   {"Summit", "https://www.summitdenver.com", map[string]bool{"KovZpZAFFt1A": true, "KovZ917AQXY": true}},
}

func Decode(cfg artifact.SourceConfig, b []byte, now time.Time) (artifact.Refresh, error) {
	fail := func(s string) (artifact.Refresh, error) { return artifact.Refresh{}, fmt.Errorf("livenation: %s", s) }
	config, err := json.Marshal(cfg)
	if err != nil {
		return fail("invalid config")
	}
	cfg, err = artifact.DecodeConfigJSON(config)
	if err != nil {
		return artifact.Refresh{}, err
	}
	profile, ok := profiles[cfg.Source.ID]
	if !ok || cfg.Source.Adapter != "livenation-venue-events" || cfg.Venue.Key != cfg.Source.ID || cfg.Venue.Name != profile.name || cfg.Venue.Timezone != "America/Denver" || cfg.Venue.Website != profile.website || len(cfg.AdapterOptions) != 0 {
		return fail("unsupported config")
	}
	if now.IsZero() || len(b) > artifact.MaxDocumentBytes || !utf8.Valid(b) {
		return fail("invalid snapshot or clock")
	}
	var s struct{ Pages, Check [][]json.RawMessage }
	dec := json.NewDecoder(bytes.NewReader(b))
	if dec.Decode(&s) != nil || dec.Decode(new(any)) != io.EOF || len(s.Pages) == 0 || len(s.Pages) > 32 {
		return fail("invalid envelope")
	}
	var left, right any
	p, _ := json.Marshal(s.Pages)
	q, _ := json.Marshal(s.Check)
	json.Unmarshal(p, &left)
	json.Unmarshal(q, &right)
	if !reflect.DeepEqual(left, right) {
		return fail("capture changed")
	}
	loc, _ := time.LoadLocation(cfg.Venue.Timezone)
	today := now.In(loc)
	r := artifact.Refresh{GeneratedAt: now.UTC().Format(time.RFC3339Nano), Coverage: artifact.Coverage{From: today.Format("2006-01-02"), Through: today.AddDate(1, 0, 0).Format("2006-01-02"), Complete: true}, Observations: []artifact.Observation{}}
	seen := map[string]bool{}
	for i, rows := range s.Pages {
		if rows == nil || len(rows) > 36 || (i == len(s.Pages)-1 && len(rows) != 0) || (i < len(s.Pages)-1 && len(rows) == 0) || (i < len(s.Pages)-2 && len(rows) != 36) {
			return fail("incomplete pages")
		}
		for _, raw := range rows {
			var id struct {
				ID string `json:"tm_id"`
			}
			if json.Unmarshal(raw, &id) != nil || id.ID == "" || seen[id.ID] {
				return fail("invalid or duplicate identity")
			}
			seen[id.ID] = true
			d, err := normalize(raw, loc, profile)
			failure := ""
			if err != nil {
				failure = err.Error()
			}
			if failure == "" && (d.Date < r.Coverage.From || d.Date > r.Coverage.Through) {
				continue
			}
			data, _ := json.Marshal(d)
			r.Observations = append(r.Observations, artifact.Observation{UpstreamID: id.ID, Data: data, Failure: failure})
		}
	}
	return r, nil
}

var age = regexp.MustCompile(`(?i)\b(\d{1,2})\s*\+`)
var allAges = regexp.MustCompile(`(?i)\ball\s+ages\b`)
var policyMention = regexp.MustCompile(`(?i)\b(ages?|adult|under|parent|guardian|this show is)\b`)
var clockText = regexp.MustCompile(`(?i)\b(doors|show)\s*:?\s*(\d{1,2})(?::(\d{2}))?\s*(AM|PM)\b`)

func normalize(raw []byte, loc *time.Location, profile venueProfile) (artifact.EventData, error) {
	var e record
	d := artifact.EventData{Status: "Scheduled"}
	if json.Unmarshal(raw, &e) != nil {
		return d, fmt.Errorf("invalid record")
	}
	if e.Type != "REGULAR" || e.DataType != "discovery" || e.Multi || !profile.ids[e.Venue.ID] {
		return d, fmt.Errorf("not a supported on-site event")
	}
	d.Title = strings.TrimSpace(html.UnescapeString(e.Name))
	d.Date = e.Date
	u, err := url.Parse(e.URL)
	if err != nil || u.Scheme != "https" || u.Host != "www.ticketmaster.com" || u.User != nil || !strings.HasSuffix(u.Path, "/event/"+e.ID) {
		return d, fmt.Errorf("invalid event URL")
	}
	d.EventURL = e.URL
	d.TicketURL = e.URL
	instant, err := time.Parse(time.RFC3339, e.UTC)
	if err != nil || e.Timezone != "America/Denver" || instant.In(loc).Format("2006-01-02") != e.Date || instant.In(loc).Format("15:04:05") != e.Time {
		return d, fmt.Errorf("inconsistent date/time")
	}
	// The public listing timestamp matches doors in the reviewed records. Explicit
	// labeled times take precedence; never turn the doors timestamp into show time.
	d.DoorsAt = instant.In(loc).Format(time.RFC3339)
	clocks := map[string]string{}
	for _, m := range clockText.FindAllStringSubmatch(e.Info, -1) {
		hour, _ := strconv.Atoi(m[2])
		minute := 0
		if m[3] != "" {
			minute, _ = strconv.Atoi(m[3])
		}
		if hour < 1 || hour > 12 || minute > 59 {
			return d, fmt.Errorf("invalid labeled time")
		}
		hour %= 12
		if strings.EqualFold(m[4], "PM") {
			hour += 12
		}
		at, err := time.ParseInLocation("2006-01-02 15:04", fmt.Sprintf("%s %02d:%02d", e.Date, hour, minute), loc)
		if err != nil {
			return d, err
		}
		key := strings.ToLower(m[1])
		value := at.Format(time.RFC3339)
		if prior, ok := clocks[key]; ok && prior != value {
			return d, fmt.Errorf("conflicting labeled times")
		}
		clocks[key] = value
	}
	if doors, ok := clocks["doors"]; ok {
		if doors != d.DoorsAt {
			return d, fmt.Errorf("doors/listing mismatch")
		}
	}
	if show, ok := clocks["show"]; ok {
		d.ShowAt = show
		at, _ := time.Parse(time.RFC3339, show)
		if at.Before(instant) {
			return d, fmt.Errorf("show precedes doors")
		}
	}
	switch e.Status {
	case "onsale", "offsale", "rescheduled", "postponed":
	case "cancelled", "canceled":
		d.Status = "Cancelled"
	default:
		return d, fmt.Errorf("unknown status")
	}
	text := html.UnescapeString(e.Name + " " + e.Info)
	matches := age.FindAllStringSubmatch(text, -1)
	isAll := allAges.MatchString(text)
	if len(matches) > 0 {
		category := matches[0][1] + "+"
		for _, m := range matches {
			if m[1]+"+" != category {
				return d, fmt.Errorf("conflicting age restrictions")
			}
		}
		if isAll {
			return d, fmt.Errorf("conflicting all-ages restriction")
		}
		p := &artifact.AdmissionPolicy{Text: category, URL: e.URL}
		if category == "13+" || category == "16+" || category == "18+" || category == "21+" {
			p.Category = category
		}
		d.AdmissionPolicy = p
	} else if isAll {
		d.AdmissionPolicy = &artifact.AdmissionPolicy{Text: "All Ages", Category: "All ages", URL: e.URL}
	} else if policyMention.MatchString(e.Info) {
		d.AdmissionPolicy = &artifact.AdmissionPolicy{Text: "Age policy unclear; check event listing", URL: e.URL}
	}
	for _, a := range e.Artists {
		if name := strings.TrimSpace(html.UnescapeString(a.Name)); name != "" {
			d.Performers = append(d.Performers, name)
		}
	}
	return d, nil
}
