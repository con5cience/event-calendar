package hmt

import (
	"bytes"
	"encoding/json"
	"event-calendar/internal/artifact"
	"strings"
	"testing"
)

func federalFixture(t *testing.T) (artifact.SourceConfig, []byte) {
	c, b := fixture(t)
	c.Source.ID, c.Venue.Key = "federal", "federal"
	c.Venue.Name, c.Venue.Website = "The Federal Theatre", "https://thefederaltheatre.com"
	c.AdapterOptions["feed_id"] = "8693"
	c.AdapterOptions["layout"] = "federal"
	b = bytes.ReplaceAll(b, []byte("HQ"), []byte("The Federal Theatre"))
	return c, b
}

func TestFederalAdmission(t *testing.T) {
	for _, label := range []string{"All Ages", "All Ages / Bar with ID", "13+ / Bar with ID", "16+ / Bar with ID", "18+ / Bar with ID", "21+ / Bar with ID", "All Ages (special conditions)", ""} {
		t.Run(label, func(t *testing.T) {
			c, b := federalFixture(t)
			b = bytes.ReplaceAll(b, []byte("18+ Ages"), []byte(label))
			r, err := Decode(c, b, clock)
			if err != nil {
				t.Fatal(err)
			}
			out, err := (&artifact.Reconciler{}).Reconcile(c, nil, r, r.Coverage.From)
			if err != nil || len(out.Rejected) != 0 || len(out.Artifact.Events) != 1 {
				t.Fatalf("%v %+v", err, out)
			}
			encoded, err := artifact.EncodeArtifact(out.Artifact)
			if err != nil {
				t.Fatal(err)
			}
			var decoded artifact.Artifact
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatal(err)
			}
			e := decoded.Events[0]
			allowed := label == "All Ages" || label == "All Ages / Bar with ID"
			if !allowed {
				if e.AdmissionPolicy != nil && e.AdmissionPolicy.WithAdult != nil {
					t.Fatal("inferred restricted/unknown permission")
				}
				return
			}
			a := e.AdmissionPolicy.WithAdult
			if a == nil || a.URL != e.EventURL || a.ReviewedOn != "2026-09-10" || len(a.Ranges) != 1 {
				t.Fatalf("%+v", a)
			}
			for _, age := range []int{0, 13, 14, 16, 17, 18} {
				matches := age >= a.Ranges[0].MinAge && age <= a.Ranges[0].MaxAge
				if matches != (age <= 17) {
					t.Fatalf("age %d: %+v", age, a)
				}
			}
		})
	}
}

func TestFederalGuardsAndOverride(t *testing.T) {
	c, b := federalFixture(t)
	b = bytes.ReplaceAll(b, []byte("18+ Ages"), []byte("All Ages"))
	r, err := Decode(c, b, clock)
	if err != nil {
		t.Fatal(err)
	}
	c.Overrides = []artifact.Override{{Match: artifact.Match{UpstreamID: "1001", Date: "2026-09-12", VenueKey: "federal"}, Set: map[string]json.RawMessage{"admission_policy": json.RawMessage(`{"text":"18+ only","category":"18+"}`)}}}
	out, err := (&artifact.Reconciler{}).Reconcile(c, nil, r, r.Coverage.From)
	if err != nil || len(out.Rejected) != 0 {
		t.Fatalf("%v %+v", err, out)
	}
	if out.Artifact.Events[0].AdmissionPolicy.WithAdult != nil {
		t.Fatal("override lost")
	}
	var raw map[string]any
	json.Unmarshal(b, &raw)
	raw["details"].(map[string]any)["1001"].(map[string]any)["location"] = map[string]any{"name": "Other venue"}
	b, _ = json.Marshal(raw)
	r, err = Decode(c, b, clock)
	if err == nil && r.Observations[0].Failure == "" {
		t.Fatal("off-site detail accepted")
	}
	// Feed IDs are supplied by the locale profile, not a built-in venue map.
	c.AdapterOptions["feed_id"] = "invalid"
	if _, err := Decode(c, b, clock); err == nil {
		t.Fatal("wrong feed accepted")
	}
}

func TestFederalMalformedDescriptions(t *testing.T) {
	for _, tc := range []struct {
		name, block string
		valid       bool
	}{
		{"paragraphs", "DESCRIPTION:First paragraph.\r\n\r\nChapel Perilous is an upgrade.\r\n\r\nLast paragraph.\r\nCREATED:20260803T110044Z\r\n", true},
		{"empty", "DESCRIPTION:\r\nCREATED:20260803T110044Z\r\n", true},
		{"folded", "DESCRIPTION:First\\nsecond\r\n continuation\r\nCREATED:20260803T110044Z\r\n", true},
		{"missing boundary", "DESCRIPTION:First\r\nUnclosed paragraph\r\n", false},
		{"bad timestamp", "DESCRIPTION:First\r\nCREATED:not-a-time\r\n", false},
		{"recurrence inside", "DESCRIPTION:First\r\nRRULE:FREQ=DAILY\r\nCREATED:20260803T110044Z\r\n", false},
		{"status inside", "DESCRIPTION:First\r\nSTATUS:CANCELLED\r\nCREATED:20260803T110044Z\r\n", false},
		{"unknown property inside", "DESCRIPTION:First\r\nX-UNKNOWN:value\r\nCREATED:20260803T110044Z\r\n", false},
		{"duplicate description", "DESCRIPTION:First\r\nDESCRIPTION:Second\r\nCREATED:20260803T110044Z\r\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, b := federalFixture(t)
			var raw map[string]any
			json.Unmarshal(b, &raw)
			raw["calendar"] = strings.Replace(raw["calendar"].(string), "URL;VALUE=URI:", tc.block+"URL;VALUE=URI:", 1)
			b, _ = json.Marshal(raw)
			r, err := Decode(c, b, clock)
			if !tc.valid {
				if err == nil {
					t.Fatal("accepted ambiguous description boundary")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			out, err := (&artifact.Reconciler{}).Reconcile(c, nil, r, r.Coverage.From)
			if err != nil || len(out.Rejected) != 0 || len(out.Artifact.Events) != 1 {
				t.Fatalf("%v %+v", err, out)
			}
			e := out.Artifact.Events[0]
			if e.UpstreamID != "1001" || e.Title != "Fixture, Ensemble" || e.ShowAt != "2026-09-12T20:00:00-06:00" || e.TicketURL != "https://tickets.holdmyticket.com/tickets/1001" {
				t.Fatalf("fields changed: %+v", e)
			}
			if tc.name == "paragraphs" {
				// The same malformed data must still fail for HQ.
				hq, _ := fixture(t)
				b = bytes.ReplaceAll(b, []byte("The Federal Theatre"), []byte("HQ"))
				if _, err := Decode(hq, b, clock); err == nil {
					t.Fatal("cleanup leaked to HQ")
				}
			}
		})
	}
}
