package livenation

import (
	"event-calendar/internal/artifact"
	"testing"
	"time"
)

func TestConfiguredNonDenverVenue(t *testing.T) {
	c := artifact.SourceConfig{SchemaVersion: 1, Source: artifact.Source{ID: "harbor", Adapter: "livenation-venue-events"}, State: "new", Venue: artifact.Venue{Key: "harbor", Name: "Harbor", Website: "https://harbor.example", Timezone: "Pacific/Auckland"}, AdapterOptions: map[string]string{"venue_ids": "KovTest"}}
	if _, err := Decode(c, []byte(`{"Pages":[[]],"Check":[[]]}`), time.Now()); err != nil {
		t.Fatal(err)
	}
}
