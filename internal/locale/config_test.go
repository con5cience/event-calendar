package locale

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"os"
	"testing"
)

func fixture(t *testing.T) Config {
	t.Helper()
	b, err := os.ReadFile("../../tests/contracts/site.json")
	if err != nil {
		t.Fatal(err)
	}
	c, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func TestConfigValidation(t *testing.T) {
	for _, change := range []func(*Config){func(c *Config) { c.Origin = "https://example.com/path" }, func(c *Config) { c.Origin = "https://user@example.com" }, func(c *Config) { c.Timezone = "Local" }, func(c *Config) { c.ID = "../denver" }, func(c *Config) { c.VenueColors["bad"] = "red; background:url(x)" }, func(c *Config) { c.Sources = nil }} {
		c := fixture(t)
		change(&c)
		b, _ := json.Marshal(c)
		if _, err := Decode(b); err == nil {
			t.Fatal("invalid locale accepted")
		}
	}
}
func TestLocalesCannotConsumeEachOthersSources(t *testing.T) {
	c := fixture(t)
	a := artifact.Artifact{Source: artifact.Source{ID: "gothic", Adapter: "aeg-json"}, Venue: artifact.Venue{Key: "gothic"}}
	if err := c.ValidateArtifacts([]artifact.Artifact{a}); err != nil {
		t.Fatal(err)
	}
	c.ID = "coastal"
	c.Sources = map[string]Source{"harbor": {Adapter: "aeg-json", VenueKey: "harbor"}}
	if c.ValidateArtifacts([]artifact.Artifact{a}) == nil {
		t.Fatal("cross-locale source accepted")
	}
}
