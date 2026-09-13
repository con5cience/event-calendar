// Package locale validates the configuration for one independently deployed site.
package locale

import (
	"bytes"
	"encoding/json"
	"event-calendar/internal/artifact"
	"event-calendar/internal/store"
	"fmt"
	"golang.org/x/text/language"
	"image/png"
	"io"
	"net/url"
	"os"
	"regexp"
	"time"
)

type Limits struct {
	Week  int `json:"week"`
	Month int `json:"month"`
	Day   int `json:"day"`
}
type Source struct {
	Adapter  string `json:"adapter"`
	VenueKey string `json:"venue_key"`
}
type Presentation struct {
	SchemaVersion    int               `json:"schema_version"`
	ID               string            `json:"id"`
	City             string            `json:"city"`
	Name             string            `json:"name"`
	Tagline          string            `json:"tagline"`
	Description      string            `json:"description"`
	Origin           string            `json:"origin"`
	Timezone         string            `json:"timezone"`
	Language         string            `json:"language"`
	WeekStart        int               `json:"week_start"`
	DefaultView      string            `json:"default_view"`
	StorageNamespace string            `json:"storage_namespace"`
	ICSNamespace     string            `json:"ics_namespace"`
	ImageAlt         string            `json:"image_alt"`
	ImageWidth       int               `json:"image_width"`
	ImageHeight      int               `json:"image_height"`
	Limits           Limits            `json:"limits"`
	VenueColors      map[string]string `json:"venue_colors"`
}
type Config struct {
	Presentation
	Sources map[string]Source `json:"sources"`
}

var key = regexp.MustCompile(`^[a-z0-9]+(?:[.-][a-z0-9]+)*$`)
var color = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func Load(dir string) (Config, error) {
	var c Config
	if dir == "" {
		return c, fmt.Errorf("SITE_DIR is required")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return c, err
	}
	defer root.Close()
	b, err := store.ReadDocument(root, "site.json")
	if err != nil {
		return c, err
	}
	c, err = Decode(b)
	if err != nil {
		return c, err
	}
	b, err = store.ReadDocument(root, "assets/favicon.png")
	if err != nil {
		return c, err
	}
	dimensions, err := png.DecodeConfig(bytes.NewReader(b))
	if err != nil || dimensions.Width != c.ImageWidth || dimensions.Height != c.ImageHeight {
		return c, fmt.Errorf("locale image dimensions do not match site.json")
	}
	return c, nil
}
func Decode(b []byte) (Config, error) {
	var c Config
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if len(b) > artifact.MaxDocumentBytes || d.Decode(&c) != nil || d.Decode(new(any)) != io.EOF {
		return c, fmt.Errorf("invalid site.json")
	}
	u, err := url.Parse(c.Origin)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return c, fmt.Errorf("origin must be a fixed HTTPS origin without path, query or credentials")
	}
	if c.SchemaVersion != 1 || !key.MatchString(c.ID) || !key.MatchString(c.StorageNamespace) || !key.MatchString(c.ICSNamespace) || c.City == "" || c.Name == "" || c.Description == "" || c.ImageAlt == "" || c.ImageWidth < 1 || c.ImageHeight < 1 || c.WeekStart < 0 || c.WeekStart > 6 || c.Limits.Week < 1 || c.Limits.Month < 1 || c.Limits.Day < 0 {
		return c, fmt.Errorf("invalid locale identity or display settings")
	}
	if c.DefaultView != "week" && c.DefaultView != "month" && c.DefaultView != "day" {
		return c, fmt.Errorf("invalid default view")
	}
	if c.Timezone == "" || c.Timezone == "Local" {
		return c, fmt.Errorf("explicit IANA timezone required")
	}
	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return c, err
	}
	if _, err := language.Parse(c.Language); err != nil || c.Language == "" {
		return c, fmt.Errorf("invalid formatting language")
	}
	if c.Sources == nil || c.VenueColors == nil {
		return c, fmt.Errorf("sources and venue_colors must be explicit objects")
	}
	for id, s := range c.Sources {
		if !key.MatchString(id) || !key.MatchString(s.VenueKey) || !key.MatchString(s.Adapter) {
			return c, fmt.Errorf("invalid source identity")
		}
	}
	for name, v := range c.VenueColors {
		if name == "" || !color.MatchString(v) {
			return c, fmt.Errorf("invalid venue color")
		}
	}
	return c, nil
}
func (c Config) ValidateArtifacts(all []artifact.Artifact) error {
	for _, a := range all {
		s, ok := c.Sources[a.Source.ID]
		if !ok || s.Adapter != a.Source.Adapter || s.VenueKey != a.Venue.Key {
			return fmt.Errorf("source %q does not belong to locale %q", a.Source.ID, c.ID)
		}
	}
	return nil
}
func (c Config) Title() string { return c.Name + ": " + c.Tagline }

// Operational source profiles are not loaded or exposed by the application.
// Packaging validates them separately, before any output is created.
func (c Config) ValidateSourceFiles(dir string) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	for id, expected := range c.Sources {
		b, err := store.ReadDocument(root, "sources/"+id+".yaml")
		if err != nil {
			return err
		}
		cfg, err := artifact.DecodeConfigYAML(b)
		if err != nil {
			return err
		}
		if cfg.Source.ID != id || cfg.Source.Adapter != expected.Adapter || cfg.Venue.Key != expected.VenueKey {
			return fmt.Errorf("source profile %q differs from locale registry", id)
		}
	}
	return nil
}
