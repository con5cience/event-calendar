package artifact

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	_ "time/tzdata"
	"unicode/utf8"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const MaxDocumentBytes = 4 << 20
const schemaBase = "https://event-calendar.invalid/schema/v1/"

//go:embed schema/v1/*.json
var schemas embed.FS
var compiled = sync.OnceValues(compileSchemas)

type bundledOnly struct{}

func (bundledOnly) Load(uri string) (any, error) {
	return nil, fmt.Errorf("unbundled schema reference: %s", uri)
}

var timezoneShape = regexp.MustCompile(`^(UTC|[A-Za-z_]+(/[A-Za-z0-9_+.-]+)+)$`)

func validTimezone(value string) error {
	if !timezoneShape.MatchString(value) {
		return fmt.Errorf("invalid IANA timezone")
	}
	_, err := time.LoadLocation(value)
	return err
}
func validURL(value string) error {
	if strings.ContainsAny(value, "\\ \t\r\n") {
		return fmt.Errorf("invalid HTTP URL")
	}
	u, err := url.Parse(value)
	if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("invalid HTTP URL")
	}
	return nil
}
func compileSchemas() (map[string]*jsonschema.Schema, error) {
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(bundledOnly{})
	c.RegisterFormat(&jsonschema.Format{Name: "safe-http-url", Validate: func(v any) error {
		if s, ok := v.(string); ok {
			return validURL(s)
		}
		return nil
	}})
	c.RegisterFormat(&jsonschema.Format{Name: "iana-time-zone", Validate: func(v any) error {
		if s, ok := v.(string); ok {
			return validTimezone(s)
		}
		return nil
	}})
	names := []string{"common", "source-artifact", "catalog", "source-config", "event-data"}
	for _, name := range names {
		b, err := schemas.ReadFile("schema/v1/" + name + ".schema.json")
		if err != nil {
			return nil, err
		}
		var doc any
		if err = json.Unmarshal(b, &doc); err != nil {
			return nil, err
		}
		if err = c.AddResource(schemaBase+name+".schema.json", doc); err != nil {
			return nil, err
		}
	}
	out := map[string]*jsonschema.Schema{}
	for _, name := range names {
		s, err := c.Compile(schemaBase + name + ".schema.json")
		if err != nil {
			return nil, err
		}
		out[name] = s
	}
	return out, nil
}
func parseJSON(b []byte) (any, error) {
	if len(b) > MaxDocumentBytes || !utf8.Valid(b) {
		return nil, fmt.Errorf("document too large or invalid UTF-8")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var value any
	if err := d.Decode(&value); err != nil {
		return nil, err
	}
	var tail any
	if err := d.Decode(&tail); err != io.EOF {
		return nil, fmt.Errorf("expected one JSON document")
	}
	return value, nil
}

// JSON Schema accepts integral exponents and decimal notation. Canonicalize those
// exact numbers before decoding into Go integer fields, without float rounding.
func normalizeNumbers(value any) any {
	switch v := value.(type) {
	case json.Number:
		if r, ok := new(big.Rat).SetString(string(v)); ok && r.IsInt() {
			return json.Number(r.Num().String())
		}
	case map[string]any:
		for k, x := range v {
			v[k] = normalizeNumbers(x)
		}
	case []any:
		for i, x := range v {
			v[i] = normalizeNumbers(x)
		}
	}
	return value
}
func decode(b []byte, kind string, dst any) error {
	value, err := parseJSON(b)
	if err != nil {
		return err
	}
	all, err := compiled()
	if err != nil {
		return fmt.Errorf("compile contract: %w", err)
	}
	if err = all[kind].Validate(value); err != nil {
		return err
	}
	canonical, err := json.Marshal(normalizeNumbers(value))
	if err != nil {
		return err
	}
	return json.Unmarshal(canonical, dst)
}
func DecodeArtifact(b []byte) (Artifact, error) {
	var a Artifact
	if err := decode(b, "source-artifact", &a); err != nil {
		return a, err
	}
	if a.Coverage.From > a.Coverage.Through {
		return a, fmt.Errorf("reversed coverage")
	}
	if a.Venue.AdmissionPolicy != nil {
		if err := validateAdmission(a.Venue.AdmissionPolicy.WithAdult); err != nil {
			return a, err
		}
	}
	ids, paths, occurrences := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, e := range a.Events {
		if err := validateData(a.Venue, e.EventData); err != nil {
			return a, err
		}
		if e.OffSite != (e.Venue != nil) {
			return a, fmt.Errorf("off-site flag differs from venue attribution")
		}
		expiry, err := ExpirationDate(e.Date)
		if err != nil {
			return a, err
		}
		if e.ExpiresOn != expiry {
			return a, fmt.Errorf("invalid expiration date")
		}
		parts := strings.Split(e.PublicPath, "/")
		suffix := strings.TrimPrefix(e.ID, a.Source.ID+"-")
		if !strings.HasPrefix(e.ID, a.Source.ID+"-") || !suffixShape.MatchString(suffix) {
			return a, fmt.Errorf("event ID must belong to source and contain its assigned suffix")
		}
		if len(parts) != 4 || parts[2] != EffectiveVenue(a.Venue, e).Key || !strings.HasPrefix(parts[3], e.Date+"-") || !strings.HasSuffix(parts[3], "-"+suffix) {
			return a, fmt.Errorf("public path differs from occurrence identity")
		}
		occurrence := occurrenceKey(e.UpstreamID, e.Date, EffectiveVenue(a.Venue, e).Key)
		if ids[e.ID] || paths[e.PublicPath] || occurrences[occurrence] {
			return a, fmt.Errorf("duplicate event identity, path, or occurrence")
		}
		ids[e.ID] = true
		paths[e.PublicPath] = true
		occurrences[occurrence] = true
	}
	return a, nil
}
func DecodeCatalog(b []byte) (Catalog, error) {
	var c Catalog
	if err := decode(b, "catalog", &c); err != nil {
		return c, err
	}
	seen := map[string]bool{}
	for _, r := range c.Sources {
		if seen[r.SourceID] {
			return c, fmt.Errorf("duplicate catalog source")
		}
		seen[r.SourceID] = true
		if !strings.HasPrefix(r.Artifact, "sources/"+r.SourceID+"/") {
			return c, fmt.Errorf("artifact belongs to another source")
		}
	}
	return c, nil
}
func DecodeConfigJSON(b []byte) (SourceConfig, error) {
	var c SourceConfig
	if err := decode(b, "source-config", &c); err != nil {
		return c, err
	}
	seen := map[string]bool{}
	for _, o := range c.Overrides {
		k := occurrenceKey(o.Match.UpstreamID, o.Match.Date, o.Match.VenueKey)
		if seen[k] {
			return c, fmt.Errorf("duplicate override selector")
		}
		seen[k] = true
		for _, name := range o.Remove {
			if _, ok := o.Set[name]; ok {
				return c, fmt.Errorf("override both sets and removes %s", name)
			}
		}
		if p, ok := o.Set["price"]; ok {
			var price Price
			if err := json.Unmarshal(p, &price); err != nil {
				return c, err
			}
			if err := validatePrice(&price); err != nil {
				return c, err
			}
		}
		if p, ok := o.Set["admission_policy"]; ok {
			var policy AdmissionPolicy
			json.Unmarshal(p, &policy)
			if err := validateAdmission(policy.WithAdult); err != nil {
				return c, err
			}
		}
	}
	for _, rule := range c.AdmissionRules {
		if err := validateAdmission(&rule); err != nil {
			return c, err
		}
	}
	if c.Venue.AdmissionPolicy != nil {
		if err := validateAdmission(c.Venue.AdmissionPolicy.WithAdult); err != nil {
			return c, err
		}
	}
	return c, nil
}
func validatePrice(p *Price) error {
	if p != nil && p.MinMinor != nil && p.MaxMinor != nil && *p.MinMinor > *p.MaxMinor {
		return fmt.Errorf("reversed price range")
	}
	return nil
}
func validateData(defaultVenue Venue, d EventData) error {
	for _, policy := range []*AdmissionPolicy{defaultVenue.AdmissionPolicy, d.AdmissionPolicy} {
		if policy != nil {
			if err := validateAdmission(policy.WithAdult); err != nil {
				return err
			}
		}
	}
	venue := defaultVenue
	if d.Venue != nil {
		venue = *d.Venue
		if venue.AdmissionPolicy != nil {
			if err := validateAdmission(venue.AdmissionPolicy.WithAdult); err != nil {
				return err
			}
		}
		if venue.Key == defaultVenue.Key {
			return fmt.Errorf("off-site venue has default venue key")
		}
	}
	for _, value := range []string{d.DoorsAt, d.ShowAt} {
		if value == "" {
			continue
		}
		if venue.Timezone == "" {
			return fmt.Errorf("timed event requires actual venue timezone")
		}
		zone, err := time.LoadLocation(venue.Timezone)
		if err != nil {
			return err
		}
		instant, err := time.Parse(time.RFC3339Nano, value)
		if err != nil || instant.In(zone).Format("2006-01-02") != d.Date {
			return fmt.Errorf("instant differs from actual venue date")
		}
	}
	return validatePrice(d.Price)
}
func EncodeArtifact(a Artifact) ([]byte, error) {
	b, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	if _, err = DecodeArtifact(b); err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
func ExpirationDate(date string) (string, error) {
	d, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "", err
	}
	return d.AddDate(0, 0, 90).Format("2006-01-02"), nil
}

// Expired uses a caller-supplied civil date, not elapsed 24-hour periods.
// Callers must validate today once and use the intended lifecycle timezone.
func Expired(e Event, today string) bool { return today >= e.ExpiresOn }
func occurrenceKey(id, date, venue string) string {
	b, _ := json.Marshal([]string{id, date, venue})
	return string(b)
}
