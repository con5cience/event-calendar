package web

import (
	"event-calendar/internal/artifact"
	"event-calendar/internal/locale"
	"event-calendar/internal/store"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// catalogReader serializes refreshes so a slower request cannot install an older
// snapshot after a newer request. A failed refresh never replaces lastGood.
type catalogReader struct {
	site        locale.Config
	mu          sync.Mutex
	dir         string
	lastGood    []artifact.Artifact
	established bool
}

func (r *catalogReader) load() ([]artifact.Artifact, error) {
	root, err := os.OpenRoot(r.dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	b, err := store.ReadDocument(root, "catalog.json")
	if os.IsNotExist(err) && !r.established {
		dir, openErr := root.Open(".")
		if openErr != nil {
			return nil, openErr
		}
		defer dir.Close()
		entries, listErr := dir.Readdirnames(1)
		if listErr == io.EOF && len(entries) == 0 {
			return nil, nil
		}
		if listErr != nil && listErr != io.EOF {
			return nil, listErr
		}
	}
	// Once any publication state is observed, missing state is never empty startup.
	r.established = true
	if err != nil {
		return nil, err
	}
	_, next, err := store.Validate(b, func(name string) ([]byte, error) { return store.ReadDocument(root, name) })
	if err == nil && r.site.Sources != nil {
		err = r.site.ValidateArtifacts(next)
	}
	return next, err
}

func (r *catalogReader) events(now time.Time) ([]Event, error) {
	all, err := r.retained(now)
	if all == nil {
		return nil, err
	}
	listed := []Event{}
	for _, e := range all {
		if e.Listed {
			listed = append(listed, e)
		}
	}
	return listed, err
}

func (r *catalogReader) retained(now time.Time) ([]Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	next, err := r.load()
	if err == nil && next != nil {
		r.lastGood = next
	}
	if err != nil && r.lastGood == nil {
		return nil, err
	}
	events := []Event{}
	for _, a := range r.lastGood {
		for _, e := range a.Events {
			venue := artifact.EffectiveVenue(a.Venue, e)
			zone := venue.Timezone
			// Untimed off-site records may lack an actual timezone. Use the source's
			// venue timezone for lifecycle dates only; never present it as known metadata.
			if zone == "" {
				zone = a.Venue.Timezone
			}
			loc, zoneErr := time.LoadLocation(zone)
			if zoneErr != nil {
				return nil, zoneErr
			}
			if artifact.Expired(e, now.In(loc).Format("2006-01-02")) {
				continue
			}
			out := Event{ID: e.ID, PublicPath: e.PublicPath, Listed: e.Listed, Title: e.Title, Venue: venue.Name, Date: e.Date, Timezone: venue.Timezone,
				Artist: strings.Join(e.Performers, ", "), DoorsAt: e.DoorsAt, ShowAt: e.ShowAt,
				EventURL: e.EventURL, TicketURL: e.TicketURL, OffSite: e.OffSite, Status: e.Status}
			if policy := artifact.EffectivePolicy(a.Venue, e); policy != nil {
				out.AgePolicy = policy.Text
				out.AgePolicyURL = policy.URL
				out.AgeCategory = policy.Category
				out.WithAdult = policy.WithAdult
			}
			events = append(events, out)
		}
	}
	return events, err
}
