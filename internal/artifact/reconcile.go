package artifact

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Observation keeps identity outside possibly-invalid normalized content. Adapters
// must supply a stable per-performance ID, not a hash of mutable title/time.
type Observation struct {
	UpstreamID string
	Data       json.RawMessage
	// Failure retains a known occurrence without treating bad source data as absence.
	Failure string
}
type Refresh struct {
	GeneratedAt  string
	Coverage     Coverage
	Observations []Observation
	Failure      string
}
type Rejection struct {
	UpstreamID string `json:"upstream_id"`
	Reason     string `json:"reason"`
}
type Result struct {
	Artifact Artifact
	Rejected []Rejection
}
type Reconciler struct{ NewSuffix func() (string, error) }

func randomSuffix() (string, error) {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)
var suffixShape = regexp.MustCompile(`^[a-f0-9]{12}$`)

func slug(s string) string {
	s = strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if len(s) > 120 {
		s = strings.TrimRight(s[:120], "-")
	}
	if s == "" {
		return "event"
	}
	return s
}

// Reconcile returns a candidate only after the whole candidate validates. On error
// there is no replacement publication. It never changes cfg, prior, or observations.
// today is a validated civil date supplied by the future coordinator's clock.
func (r *Reconciler) Reconcile(cfg SourceConfig, prior *Artifact, run Refresh, today string) (Result, error) {
	fail := func(message string) (Result, error) { return Result{}, fmt.Errorf("reconcile: %s", message) }
	if _, err := time.Parse("2006-01-02", today); err != nil {
		return fail("invalid current date")
	}
	configBytes, err := json.Marshal(cfg)
	if err != nil {
		return Result{}, err
	}
	cfg, err = DecodeConfigJSON(configBytes)
	if err != nil {
		return Result{}, err
	}
	if cfg.State == "established" && prior == nil {
		return fail("established source requires readable prior artifact")
	}
	if cfg.State == "new" && prior != nil {
		return fail("new source cannot overwrite prior state")
	}
	var previous Artifact
	if prior != nil {
		b, err := json.Marshal(prior)
		if err != nil {
			return Result{}, err
		}
		previous, err = DecodeArtifact(b)
		if err != nil {
			return Result{}, err
		}
		if previous.Source != cfg.Source || previous.Venue.Key != cfg.Venue.Key {
			return fail("source or default venue identity changed")
		}
	}
	if run.Failure != "" || !run.Coverage.Complete {
		return fail("source refresh failed or incomplete")
	}
	if run.Coverage.From != today || run.Coverage.Through < today {
		return fail("refresh coverage must start on current date")
	}
	candidate := Artifact{SchemaVersion: 1, Source: cfg.Source, Venue: cfg.Venue, GeneratedAt: run.GeneratedAt, Coverage: run.Coverage, Events: []Event{}}
	// Validate the envelope before any absence reconciliation.
	if _, err := EncodeArtifact(candidate); err != nil {
		return Result{}, err
	}
	byOccurrence := map[string]Event{}
	byProvider := map[string][]Event{}
	usedIDs, usedPaths := map[string]bool{}, map[string]bool{}
	for _, e := range previous.Events {
		k := occurrenceKey(e.UpstreamID, e.Date, EffectiveVenue(previous.Venue, e).Key)
		byOccurrence[k] = e
		byProvider[e.UpstreamID] = append(byProvider[e.UpstreamID], e)
		usedIDs[e.ID] = true
		usedPaths[e.PublicPath] = true
	}
	overrides := map[string]Override{}
	for _, o := range cfg.Overrides {
		overrides[occurrenceKey(o.Match.UpstreamID, o.Match.Date, o.Match.VenueKey)] = o
	}
	seen := map[string]bool{}
	kept := map[string]Event{}
	rejected := []Rejection{}
	for _, obs := range run.Observations {
		if strings.TrimSpace(obs.UpstreamID) == "" || len(obs.UpstreamID) > 2048 {
			return fail("observation has no usable stable identity")
		}
		value, parseErr := parseJSON(obs.Data)
		raw, objectOK := value.(map[string]any)
		var date, venueKey string
		if objectOK {
			date, _ = raw["date"].(string)
			venueKey = cfg.Venue.Key
			if v, ok := raw["venue"].(map[string]any); ok {
				venueKey, _ = v["key"].(string)
			}
		}
		_, dateErr := time.Parse("2006-01-02", date)
		venueIdentified := false
		if objectOK {
			if value, present := raw["venue"]; !present {
				// A dated normalized record with no venue explicitly means on-site.
				venueIdentified = dateErr == nil
			} else if v, ok := value.(map[string]any); ok {
				key, ok := v["key"].(string)
				venueIdentified = ok && key != ""
			}
		}
		occurrence := occurrenceKey(obs.UpstreamID, date, venueKey)
		old, known := byOccurrence[occurrence]
		if o, ok := overrides[occurrence]; ok && objectOK {
			for k, b := range o.Set {
				v, err := parseJSON(b)
				if err != nil {
					return Result{}, err
				}
				raw[k] = v
			}
			for _, k := range o.Remove {
				delete(raw, k)
			}
		}
		var data EventData
		// Retired metadata must not be restored by observations or overrides.
		delete(raw, "price")
		validationErr := parseErr
		if obs.Failure != "" {
			validationErr = fmt.Errorf("adapter: %s", obs.Failure)
		}
		if validationErr == nil {
			b, err := json.Marshal(raw)
			if err != nil {
				validationErr = err
			} else {
				validationErr = decode(b, "event-data", &data)
			}
		}
		if validationErr == nil {
			validationErr = validateData(cfg.Venue, data)
		}
		if validationErr != nil {
			if !known {
				matches := []Event{}
				for _, e := range byProvider[obs.UpstreamID] {
					if date != "" {
						if _, err := time.Parse("2006-01-02", date); err == nil && e.Date != date {
							continue
						}
					}
					if venueIdentified && EffectiveVenue(previous.Venue, e).Key != venueKey {
						continue
					}
					matches = append(matches, e)
				}
				if len(matches) > 1 {
					return fail("invalid observation cannot identify its prior occurrence")
				}
				if len(matches) == 1 {
					old = matches[0]
					known = true
					occurrence = occurrenceKey(old.UpstreamID, old.Date, EffectiveVenue(previous.Venue, old).Key)
				}
			}
			if seen[occurrence] {
				return fail("duplicate or ambiguous observation")
			}
			seen[occurrence] = true
			if known && !Expired(old, today) {
				kept[old.ID] = old
			}
			rejected = append(rejected, Rejection{obs.UpstreamID, "invalid normalized record: " + validationErr.Error()})
			continue
		}
		if seen[occurrence] {
			return fail("duplicate observation occurrence")
		}
		seen[occurrence] = true
		data = EnrichAdmission(cfg, obs.UpstreamID, data)
		expiration, _ := ExpirationDate(data.Date)
		if expiration <= today {
			rejected = append(rejected, Rejection{obs.UpstreamID, "event has expired"})
			continue
		}
		next := Event{EventData: data, UpstreamID: obs.UpstreamID, OffSite: data.Venue != nil, Listed: true, ExpiresOn: expiration}
		if known {
			next.ID = old.ID
			next.PublicPath = old.PublicPath
		} else {
			generator := r.NewSuffix
			if generator == nil {
				generator = randomSuffix
			}
			for attempt := 0; attempt < 100; attempt++ {
				suffix, err := generator()
				if err != nil {
					return Result{}, err
				}
				if !suffixShape.MatchString(suffix) {
					return fail("invalid identity suffix")
				}
				id := cfg.Source.ID + "-" + suffix
				path := "/events/" + venueKey + "/" + data.Date + "-" + slug(data.Title) + "-" + suffix
				if !usedIDs[id] && !usedPaths[path] {
					next.ID = id
					next.PublicPath = path
					usedIDs[id] = true
					usedPaths[path] = true
					break
				}
			}
			if next.ID == "" {
				return fail("identity collision retry limit reached")
			}
		}
		kept[next.ID] = next
	}
	for _, e := range previous.Events {
		if _, exists := kept[e.ID]; exists || Expired(e, today) {
			continue
		}
		if e.Date >= run.Coverage.From && e.Date <= run.Coverage.Through {
			e.Listed = false
		}
		kept[e.ID] = e
	}
	for _, e := range kept {
		candidate.Events = append(candidate.Events, e)
	}
	sort.Slice(candidate.Events, func(i, j int) bool { return candidate.Events[i].ID < candidate.Events[j].ID })
	candidate = WithoutPrices(candidate)
	b, err := EncodeArtifact(candidate)
	if err != nil {
		return Result{}, err
	}
	candidate, err = DecodeArtifact(b)
	if err != nil {
		return Result{}, err
	}
	return Result{Artifact: candidate, Rejected: rejected}, nil
}
