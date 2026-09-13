package meowwolf

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"testing"
	"time"
)

func TestConfiguredNonDenverVenue(t *testing.T) {
	c := artifact.SourceConfig{SchemaVersion: 1, Source: artifact.Source{ID: "harbor", Adapter: "embedded-nextjs"}, State: "new", Venue: artifact.Venue{Key: "harbor", Name: "Harbor", Website: "https://harbor.example/events/", Timezone: "Pacific/Auckland"}, AdapterOptions: map[string]string{"seller_id": "seller", "rooms": `{"room":"Hall"}`, "default_room": "room"}}
	now := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	b, _ := json.Marshal(map[string]any{"from": "2026-09-08", "through": "2027-09-08", "total": 0, "events": []any{}, "check": []any{}})
	if _, err := Decode(c, b, now); err != nil {
		t.Fatal(err)
	}
}
