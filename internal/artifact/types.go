// Package artifact defines the version-one publication contract. It performs no I/O
// to source sites or artifact stores. The web reader validates stored bytes here.
package artifact

import "encoding/json"

type Source struct {
	ID      string `json:"id"`
	Adapter string `json:"adapter"`
}
type AdmissionPolicy struct {
	WithAdult *AdultAdmission `json:"with_adult,omitempty"`
	Text      string          `json:"text"`
	URL       string          `json:"url,omitempty"`
	Category  string          `json:"category,omitempty"`
}
type Venue struct {
	Key             string           `json:"key"`
	Name            string           `json:"name"`
	Timezone        string           `json:"timezone,omitempty"`
	Address         string           `json:"address,omitempty"`
	Phone           string           `json:"phone,omitempty"`
	Website         string           `json:"website,omitempty"`
	AdmissionPolicy *AdmissionPolicy `json:"admission_policy,omitempty"`
}
type AdmissionRange struct {
	MinAge    int    `json:"min_age"`
	MaxAge    int    `json:"max_age"`
	Condition string `json:"condition"`
}
type AdultAdmission struct {
	URL        string           `json:"url"`
	ReviewedOn string           `json:"reviewed_on"`
	Ranges     []AdmissionRange `json:"ranges"`
}
type Price struct {
	Text        string `json:"text"`
	AmountMinor *int64 `json:"amount_minor,omitempty"`
	MinMinor    *int64 `json:"min_minor,omitempty"`
	MaxMinor    *int64 `json:"max_minor,omitempty"`
	Currency    string `json:"currency,omitempty"`
}

// EventData is adapter-normalized content, before publication identity is assigned.
// Venue omission means on-site; presence describes the actual off-site venue.
type EventData struct {
	Status          string           `json:"status,omitempty"`
	Date            string           `json:"date"`
	Title           string           `json:"title"`
	Venue           *Venue           `json:"venue,omitempty"`
	Performers      []string         `json:"performers,omitempty"`
	DoorsAt         string           `json:"doors_at,omitempty"`
	ShowAt          string           `json:"show_at,omitempty"`
	Price           *Price           `json:"price,omitempty"`
	AdmissionPolicy *AdmissionPolicy `json:"admission_policy,omitempty"`
	EventURL        string           `json:"event_url,omitempty"`
	TicketURL       string           `json:"ticket_url,omitempty"`
}
type Event struct {
	EventData
	ID         string `json:"id"`
	UpstreamID string `json:"upstream_id"`
	PublicPath string `json:"public_path"`
	OffSite    bool   `json:"off_site"`
	Listed     bool   `json:"listed"`
	ExpiresOn  string `json:"expires_on"`
}
type Coverage struct {
	From     string `json:"from"`
	Through  string `json:"through"`
	Complete bool   `json:"complete"`
}
type Artifact struct {
	SchemaVersion int      `json:"schema_version"`
	Source        Source   `json:"source"`
	Venue         Venue    `json:"venue"`
	GeneratedAt   string   `json:"generated_at"`
	Coverage      Coverage `json:"coverage"`
	Events        []Event  `json:"events"`
}
type ArtifactRef struct {
	SourceID string `json:"source_id"`
	Artifact string `json:"artifact"`
	SHA256   string `json:"sha256"`
}
type Catalog struct {
	SchemaVersion int           `json:"schema_version"`
	Generation    string        `json:"generation"`
	GeneratedAt   string        `json:"generated_at"`
	Sources       []ArtifactRef `json:"sources"`
}
type Match struct {
	UpstreamID string `json:"upstream_id"`
	Date       string `json:"date"`
	VenueKey   string `json:"venue_key"`
}
type Override struct {
	Match  Match                      `json:"match"`
	Set    map[string]json.RawMessage `json:"set,omitempty"`
	Remove []string                   `json:"remove,omitempty"`
}
type SourceConfig struct {
	AdmissionRules map[string]AdultAdmission `json:"admission_rules,omitempty"`
	SchemaVersion  int                       `json:"schema_version"`
	Source         Source                    `json:"source"`
	Venue          Venue                     `json:"venue"`
	State          string                    `json:"state"`
	AdapterOptions map[string]string         `json:"adapter_options,omitempty"`
	Overrides      []Override                `json:"overrides,omitempty"`
}

func EffectiveVenue(defaultVenue Venue, e Event) Venue {
	if e.Venue != nil {
		return *e.Venue
	}
	return defaultVenue
}
func EffectivePolicy(defaultVenue Venue, e Event) *AdmissionPolicy {
	if e.AdmissionPolicy != nil {
		p := *e.AdmissionPolicy
		return &p
	}
	if p := EffectiveVenue(defaultVenue, e).AdmissionPolicy; p != nil {
		copy := *p
		return &copy
	}
	return nil
}
