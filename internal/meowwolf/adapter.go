// Package meowwolf normalizes scoped browser captures of Meow Wolf event data.
package meowwolf

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
	"strings"
	"time"
	_ "time/tzdata"
	"unicode/utf8"
)

var identity = regexp.MustCompile(`^[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}__[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}$`)
var slug = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var ordinal = regexp.MustCompile(`(\d)(?:st|nd|rd|th)\b`)

type metadata struct{ Metakey, Value string }
type record struct {
	ID, Title, StartDateTime, URL string
	Text                          struct {
		Date                   string
		Banner, SupportingActs *string
	}
	Tags   []string
	Detail struct {
		Error, ID, Title, SellerID, VenueID string
		Meta                                []metadata
		Timeslots                           []struct{ ID, StartTime string }
	}
}

func Decode(cfg artifact.SourceConfig, b []byte, now time.Time) (artifact.Refresh, error) {
	fail := func(s string) (artifact.Refresh, error) { return artifact.Refresh{}, fmt.Errorf("meowwolf: %s", s) }
	raw, err := json.Marshal(cfg)
	if err != nil {
		return fail("invalid configuration")
	}
	cfg, err = artifact.DecodeConfigJSON(raw)
	if err != nil {
		return artifact.Refresh{}, err
	}
	if cfg.Source.ID != "meow-wolf-denver" || cfg.Source.Adapter != "embedded-nextjs" || cfg.Venue.Key != "meow-wolf-denver" || cfg.Venue.Name != "Meow Wolf Denver" || cfg.Venue.Website != "https://tickets.meowwolf.com/events/denver/" || cfg.Venue.Timezone != "America/Denver" || len(cfg.AdapterOptions) != 0 {
		return fail("unsupported configuration")
	}
	if now.IsZero() || len(b) > artifact.MaxDocumentBytes || !utf8.Valid(b) {
		return fail("invalid snapshot")
	}
	var s struct {
		From, Through string
		Total         *int
		Events, Check []json.RawMessage
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	if dec.Decode(&s) != nil || dec.Decode(new(any)) != io.EOF || s.Total == nil || s.Events == nil || s.Check == nil || len(s.Events) > 500 || *s.Total != len(s.Events) {
		return fail("invalid envelope")
	}
	var left, right any
	p, _ := json.Marshal(s.Events)
	q, _ := json.Marshal(s.Check)
	json.Unmarshal(p, &left)
	json.Unmarshal(q, &right)
	if !reflect.DeepEqual(left, right) {
		return fail("capture changed")
	}
	loc, _ := time.LoadLocation(cfg.Venue.Timezone)
	today := now.In(loc)
	if s.From != today.Format("2006-01-02") || s.Through != today.AddDate(1, 0, 0).Format("2006-01-02") {
		return fail("coverage mismatch")
	}
	out := artifact.Refresh{GeneratedAt: now.UTC().Format(time.RFC3339Nano), Coverage: artifact.Coverage{From: s.From, Through: s.Through, Complete: true}, Observations: []artifact.Observation{}}
	seen := map[string]bool{}
	for _, raw := range s.Events {
		var id struct{ ID string }
		if json.Unmarshal(raw, &id) != nil || !identity.MatchString(id.ID) || seen[id.ID] {
			return fail("invalid identity")
		}
		seen[id.ID] = true
		data, err := normalize(raw, loc)
		if err == nil && (data.Date < s.From || data.Date > s.Through) {
			continue
		}
		failure := ""
		if err != nil {
			failure = err.Error()
		}
		b, _ := json.Marshal(data)
		out.Observations = append(out.Observations, artifact.Observation{UpstreamID: id.ID, Data: b, Failure: failure})
	}
	return out, nil
}

func clean(s string) string { return strings.Join(strings.Fields(html.UnescapeString(s)), " ") }
func normalize(raw []byte, loc *time.Location) (artifact.EventData, error) {
	var e record
	d := artifact.EventData{Status: "Scheduled"}
	if json.Unmarshal(raw, &e) != nil {
		return d, fmt.Errorf("invalid fields")
	}
	if e.Detail.Error != "" {
		return d, fmt.Errorf("unrelated detail redirect")
	}
	parts := strings.Split(e.ID, "__")
	if len(parts) != 2 || e.Detail.ID != parts[0] || e.Detail.SellerID != "017a7f54-e443-a261-3c55-46ef4d921efb" || clean(e.Detail.Title) != clean(e.Title) {
		return d, fmt.Errorf("detail identity mismatch")
	}
	meta := map[string]string{}
	for _, m := range e.Detail.Meta {
		if v, ok := meta[m.Metakey]; ok && v != m.Value {
			return d, fmt.Errorf("conflicting metadata")
		}
		meta[m.Metakey] = m.Value
	}
	rooms := map[string]string{"14f4d967-6acc-4d09-2e5b-f2707768c20d": "The Perplexiplex", "d1bdc498-735e-781d-a390-f4c7f234c482": "Sips (with a Z)", "017a7f54-ebc3-c5e1-1499-0887afa464fc": "Convergence Station"}
	if name, ok := rooms[e.Detail.VenueID]; !ok || (meta["venue"] != name && !(e.Detail.VenueID == "017a7f54-ebc3-c5e1-1499-0887afa464fc" && meta["venue"] == "")) {
		return d, fmt.Errorf("unreviewed venue")
	}
	at, err := time.Parse(time.RFC3339, e.StartDateTime)
	if err != nil {
		return d, fmt.Errorf("invalid timestamp")
	}
	matched := 0
	for _, t := range e.Detail.Timeslots {
		if t.ID == parts[1] {
			matched++
			if t.StartTime != e.StartDateTime {
				return d, fmt.Errorf("timeslot mismatch")
			}
		}
	}
	if matched != 1 {
		return d, fmt.Errorf("missing or duplicate timeslot")
	}
	at = at.In(loc)
	d.Date = at.Format("2006-01-02")
	d.Title = clean(e.Title)
	if !slug.MatchString(e.URL) {
		return d, fmt.Errorf("invalid event slug")
	}
	d.EventURL = "https://tickets.meowwolf.com/events/denver/" + e.URL + "/"
	for _, pair := range []struct {
		value  string
		target *string
	}{{meta["doorsOpen"], &d.DoorsAt}, {meta["showStart"], &d.ShowAt}} {
		if pair.value == "" {
			continue
		}
		t, err := time.ParseInLocation("2006-01-02 3:04 PM", d.Date+" "+clean(pair.value), loc)
		if err != nil {
			return d, fmt.Errorf("invalid event clock")
		}
		*pair.target = t.Format(time.RFC3339)
	}
	// The listing timestamp represents doors, not the separately labeled show.
	if d.DoorsAt == "" {
		label := ordinal.ReplaceAllString(clean(e.Text.Date), "$1")
		if label == at.Format("Jan 2")+" Doors @ "+at.Format("3:04 PM") || label == at.Format("Jan 2, 2006")+" Doors @ "+at.Format("3:04 PM") {
			d.DoorsAt = at.Format(time.RFC3339)
		}
	}
	if d.DoorsAt == "" || d.DoorsAt != at.Format(time.RFC3339) {
		return d, fmt.Errorf("doors timestamp mismatch")
	}
	if e.Text.Banner != nil && strings.EqualFold(clean(*e.Text.Banner), "CANCELED") {
		d.Status = "Cancelled"
	}
	if e.Text.Banner != nil && strings.EqualFold(clean(*e.Text.Banner), "CANCELLED") {
		d.Status = "Cancelled"
	}
	for _, key := range []string{"eventNotificationText", "externalBtnTxt"} {
		if strings.EqualFold(clean(meta[key]), "CANCELED") || strings.EqualFold(clean(meta[key]), "CANCELLED") {
			d.Status = "Cancelled"
		}
	}
	if d.Status != "Cancelled" {
		d.TicketURL = d.EventURL
		if ticket := meta["externalTicketUrl"]; ticket != "" {
			u, err := url.Parse(ticket)
			if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
				return d, fmt.Errorf("invalid ticket URL")
			}
			d.TicketURL = ticket
		}
	}
	age := clean(meta["eventAge"])
	policy := &artifact.AdmissionPolicy{Text: "Age restriction unavailable; check event terms", URL: d.EventURL}
	if age != "" {
		policy.Text = age
	}
	switch age {
	case "All Ages (Ticketed Guests Under 16 Admitted with Ticketed Guardian)", "All Ages", "All ages":
		policy.Category = "All ages"
	case "13+", "16+", "18+", "21+":
		policy.Category = age
	}
	d.AdmissionPolicy = policy
	return d, nil
}
