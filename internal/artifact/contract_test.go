package artifact

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("../../tests/contracts", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func baseline(t *testing.T) Artifact {
	t.Helper()
	a, err := DecodeArtifact(readFixture(t, "source.json"))
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func config(t *testing.T) SourceConfig {
	t.Helper()
	c, err := DecodeConfigYAML(readFixture(t, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	c.Overrides = nil
	return c
}
func TestSharedContractCorpus(t *testing.T) {
	var cases []struct {
		Name, Kind string
		Valid      bool
		Document   json.RawMessage
	}
	if err := json.Unmarshal(readFixture(t, "cases.json"), &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			var err error
			switch tc.Kind {
			case "source-artifact":
				_, err = DecodeArtifact(tc.Document)
			case "catalog":
				_, err = DecodeCatalog(tc.Document)
			case "source-config":
				_, err = DecodeConfigJSON(tc.Document)
			default:
				t.Fatalf("unknown fixture kind %s", tc.Kind)
			}
			if (err == nil) != tc.Valid {
				t.Fatalf("valid=%v, error=%v", tc.Valid, err)
			}
		})
	}
}
func TestStrictBytesAndYAML(t *testing.T) {
	good := string(readFixture(t, "config.yaml"))
	if _, err := DecodeConfigYAML([]byte(good)); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		good + "\nstate: established\n", good + "\nunknown: value\n", good + "\n---\nstate: new\n",
		strings.Replace(good, "text: 16+", "text: null", 1), strings.Replace(good, "state: new", "state: &x new", 1),
	} {
		if _, err := DecodeConfigYAML([]byte(body)); err == nil {
			t.Fatal("accepted invalid YAML")
		}
	}
	a := readFixture(t, "source.json")
	for _, body := range [][]byte{append(append([]byte{}, a...), []byte(`{}`)...), []byte(`{"schema_version":1,"schema_version":1}`), []byte(`null`), []byte(strings.Repeat(" ", MaxDocumentBytes+1))} {
		if _, err := DecodeArtifact(body); err == nil {
			t.Fatal("accepted invalid JSON")
		}
	}
}
func TestPolicyInheritanceAndOffSite(t *testing.T) {
	a := baseline(t)
	e := a.Events[0]
	if got := EffectivePolicy(a.Venue, e); got == nil || got.Text != "16+" {
		t.Fatalf("inherited %#v", got)
	}
	e.AdmissionPolicy = &AdmissionPolicy{Text: "21+"}
	if got := EffectivePolicy(a.Venue, e); got.URL != "" || got.Text != "21+" {
		t.Fatal("event policy did not replace default")
	}
	e.AdmissionPolicy = nil
	e.Venue = &Venue{Key: "warehouse", Name: "Warehouse"}
	e.OffSite = true
	if EffectivePolicy(a.Venue, e) != nil || EffectiveVenue(a.Venue, e).Timezone != "" {
		t.Fatal("off-site record inherited on-site metadata")
	}
}
func TestExpiryCalendarDays(t *testing.T) {
	for _, tc := range []struct{ date, want string }{{"2026-09-10", "2026-12-09"}, {"2026-03-08", "2026-06-06"}, {"2026-11-01", "2027-01-30"}, {"2028-02-29", "2028-05-29"}} {
		got, err := ExpirationDate(tc.date)
		if err != nil || got != tc.want {
			t.Fatalf("%s: %s %v", tc.date, got, err)
		}
	}
	a := baseline(t)
	e := a.Events[0]
	if Expired(e, "2026-12-08") || !Expired(e, "2026-12-09") {
		t.Fatal("expiry boundary must be the start of date + 90 days")
	}
	if _, err := ExpirationDate("2026-02-31"); err == nil {
		t.Fatal("accepted impossible date")
	}
}
func TestDeterministicSerialization(t *testing.T) {
	a := baseline(t)
	first, err := EncodeArtifact(a)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeArtifact(first)
	if err != nil {
		t.Fatal(err)
	}
	second, err := EncodeArtifact(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("serialization changed after round trip")
	}
}
