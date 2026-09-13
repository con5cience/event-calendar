package rhp

import (
	"event-calendar/internal/artifact"
	"testing"
	"time"
)

func TestConfiguredNonDenverVenue(t *testing.T) {
	c := artifact.SourceConfig{SchemaVersion: 1, Source: artifact.Source{ID: "harbor", Adapter: "rhp-calendar"}, State: "new", Venue: artifact.Venue{Key: "harbor", Name: "Harbor", Website: "https://harbor.example", Timezone: "Pacific/Auckland"}}
	if _, err := Decode(c, []byte(`{"Calendar":{"Success":true,"Data":{"Events":[]}},"Details":{}}`), time.Now()); err != nil {
		t.Fatal(err)
	}
}
