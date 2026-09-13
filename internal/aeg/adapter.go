// Package aeg normalizes local AEG snapshots. It has no network client.
package aeg

import (
	"bytes"
	"encoding/json"
	"event-calendar/internal/artifact"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode/utf8"
)

var digits = regexp.MustCompile(`^[1-9][0-9]*$`)

type record struct {
	Title                                                                struct{ EventTitleText, HeadlinersText string }
	EventDateTimeISO, EventDateTime, EventDateTimeUTC, EventDateTimeZone string
	DoorDateTime, DoorDateTimeUTC                                        string
	Age                                                                  string
	Ticketing                                                            struct{ Status, EventURL string }
}

// Decode accepts only an uncapped, internally consistent snapshot of a configured
// supported venue. Complete means enumeration of this supplied snapshot, not assurance
// that the provider has announced every event in the requested twelve months.
func Decode(cfg artifact.SourceConfig, b []byte, now time.Time) (artifact.Refresh, error) {
	fail := func(s string) (artifact.Refresh, error) { return artifact.Refresh{}, fmt.Errorf("aeg: %s", s) }
	configBytes, err := json.Marshal(cfg)
	if err != nil {
		return fail("invalid config")
	}
	cfg, err = artifact.DecodeConfigJSON(configBytes)
	if err != nil {
		return artifact.Refresh{}, err
	}
	venues := map[string]struct{ venue, website string }{
		"37": {"2274", "https://www.gothictheatre.com"},
		"89": {"127278", "https://www.missionballroom.com"},
		"2":  {"100811", "https://www.bluebirdtheater.net"},
		"7":  {"101141", "https://www.ogdentheatre.com"},
		"44": {"100869", "https://www.fiddlersgreenamp.com"},
	}
	p, ok := venues[cfg.AdapterOptions["feed_id"]]
	if !ok || cfg.Source.Adapter != "aeg-json" || cfg.AdapterOptions["venue_id"] != p.venue || cfg.Venue.Website != p.website || cfg.Venue.Timezone != "America/Denver" {
		return fail("configuration does not identify a supported feed and venue")
	}
	if len(cfg.AdapterOptions) != 2 {
		return fail("unsupported adapter option")
	}
	if now.IsZero() {
		return fail("clock is required")
	}
	if len(b) > artifact.MaxDocumentBytes || !utf8.Valid(b) {
		return fail("snapshot exceeds 4 MiB or is not UTF-8")
	}
	// Artifact parsing does not reject duplicate JSON keys. Upstream envelopes need
	// that stricter check so conflicting totals or event arrays cannot appear complete.
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err := uniqueJSON(d, 0); err != nil {
		return artifact.Refresh{}, err
	}
	if _, err := d.Token(); err != io.EOF {
		return fail("expected one JSON document")
	}
	var env struct {
		Meta *struct {
			Total *int
			Page  *int
			Rows  *int
		}
		Events []json.RawMessage
	}
	if err := json.Unmarshal(b, &env); err != nil {
		return fail("malformed envelope")
	}
	if env.Meta == nil || env.Meta.Total == nil || env.Meta.Page == nil || env.Meta.Rows == nil || env.Events == nil {
		return fail("missing snapshot metadata or events")
	}
	m := env.Meta
	if *m.Page != 1 || *m.Rows <= 0 || *m.Total != len(env.Events) || *m.Total >= *m.Rows {
		return fail("snapshot is inconsistent, paginated, or capped")
	}
	loc, _ := time.LoadLocation(cfg.Venue.Timezone)
	today := now.In(loc)
	through := today.AddDate(1, 0, 0).Format("2006-01-02")
	result := artifact.Refresh{GeneratedAt: now.UTC().Format(time.RFC3339Nano), Coverage: artifact.Coverage{From: today.Format("2006-01-02"), Through: through, Complete: true}, Observations: []artifact.Observation{}}
	seen := map[string]bool{}
	for _, raw := range env.Events {
		var identity struct {
			EventID         string
			Venue           *struct{ VenueID, Timezone string }
			Active, Private *bool
			PublishStatus   *int
			AdditionalDates []json.RawMessage
		}
		if err := json.Unmarshal(raw, &identity); err != nil || !digits.MatchString(identity.EventID) || identity.Venue == nil || identity.Venue.VenueID != p.venue || identity.Venue.Timezone != cfg.Venue.Timezone {
			return fail("unusable event identity or unexpected venue")
		}
		if seen[identity.EventID] {
			return fail("duplicate event ID")
		}
		seen[identity.EventID] = true
		if identity.Active == nil || identity.Private == nil || identity.PublishStatus == nil || len(identity.AdditionalDates) > 0 {
			return fail("unknown publication scope or unsupported additional dates")
		}
		if !*identity.Active || *identity.Private || *identity.PublishStatus != 1 {
			return fail("unsupported publication state")
		}
		var e record
		parseErr := json.Unmarshal(raw, &e)
		data, normalizeErr := normalize(e, loc, p.website, identity.EventID)
		failure := ""
		if parseErr != nil {
			failure = "unexpected event field type"
		} else if normalizeErr != nil {
			failure = normalizeErr.Error()
		}
		// Keep invalid observations even if their parsed date appears out of range:
		// reconciliation must retain a known valid version, not infer absence.
		if failure == "" && (data.Date < result.Coverage.From || data.Date > through) {
			continue
		}
		normalized, err := json.Marshal(data)
		if err != nil {
			return artifact.Refresh{}, err
		}
		result.Observations = append(result.Observations, artifact.Observation{UpstreamID: identity.EventID, Data: normalized, Failure: failure})
	}
	return result, nil
}

func normalize(e record, loc *time.Location, website, id string) (artifact.EventData, error) {
	data := artifact.EventData{Title: strings.TrimSpace(e.Title.EventTitleText), Status: strings.TrimSpace(e.Ticketing.Status), EventURL: website + "/events/detail?event_id=" + id, TicketURL: strings.TrimSpace(e.Ticketing.EventURL)}
	if data.Title == "" {
		data.Title = strings.TrimSpace(e.Title.HeadlinersText)
	}
	if artist := strings.TrimSpace(e.Title.HeadlinersText); artist != "" {
		data.Performers = []string{artist}
	}
	if e.Age != "" {
		data.AdmissionPolicy = &artifact.AdmissionPolicy{Text: e.Age, Category: map[string]string{"All Ages": "All ages", "13 & Over": "13+", "16 & Over": "16+", "18 & Over": "18+", "21 & Over": "21+"}[e.Age]}
	}
	show, err := time.Parse(time.RFC3339, e.EventDateTimeISO)
	if err != nil {
		return data, fmt.Errorf("invalid eventDateTimeISO")
	}
	data.Date = show.In(loc).Format("2006-01-02")
	if e.EventDateTimeZone != loc.String() || show.Format("-07:00") != show.In(loc).Format("-07:00") {
		return data, fmt.Errorf("event timezone conflicts with configured venue")
	}
	data.ShowAt = show.In(loc).Format(time.RFC3339)
	if e.EventDateTime != "" && show.In(loc).Format("2006-01-02T15:04:05") != e.EventDateTime {
		return data, fmt.Errorf("conflicting local show time")
	}
	if e.EventDateTimeUTC != "" {
		utc, err := time.Parse("2006-01-02T15:04:05", e.EventDateTimeUTC)
		if err != nil || !utc.Equal(show) {
			return data, fmt.Errorf("conflicting UTC show time")
		}
	}
	if e.DoorDateTimeUTC != "" {
		utc, err := time.Parse("2006-01-02T15:04:05", e.DoorDateTimeUTC)
		if err != nil {
			return data, fmt.Errorf("invalid UTC doors time")
		}
		if e.DoorDateTime != "" && utc.In(loc).Format("2006-01-02T15:04:05") != e.DoorDateTime {
			return data, fmt.Errorf("conflicting local doors time")
		}
		data.DoorsAt = utc.In(loc).Format(time.RFC3339)
	} else if e.DoorDateTime != "" {
		return data, fmt.Errorf("doors time lacks UTC disambiguation")
	}
	return data, nil
}

func uniqueJSON(d *json.Decoder, depth int) error {
	if depth > 64 {
		return fmt.Errorf("aeg: excessive JSON nesting")
	}
	token, err := d.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			s, ok := key.(string)
			if !ok || seen[s] {
				return fmt.Errorf("aeg: duplicate or invalid JSON key")
			}
			seen[s] = true
			if err := uniqueJSON(d, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := uniqueJSON(d, depth+1); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("aeg: unexpected JSON delimiter")
	}
	_, err = d.Token()
	return err
}
