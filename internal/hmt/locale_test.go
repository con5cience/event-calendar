package hmt

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"testing"
	"time"
)

func TestConfiguredNonDenverVenue(t *testing.T) {
	c := artifact.SourceConfig{SchemaVersion: 1, Source: artifact.Source{ID: "harbor", Adapter: "holdmyticket-ical"}, State: "new", Venue: artifact.Venue{Key: "harbor", Name: "Harbor", Website: "https://harbor.example", Timezone: "Pacific/Auckland"}, AdapterOptions: map[string]string{"feed_id": "987"}}
	b, _ := json.Marshal(map[string]any{"Calendar": "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nX-WR-TIMEZONE:Pacific/Auckland\r\nX-WR-CALNAME:Harbor\r\nEND:VCALENDAR\r\n", "Details": map[string]string{}})
	if _, err := Decode(c, b, time.Now()); err != nil {
		t.Fatal(err)
	}
}
