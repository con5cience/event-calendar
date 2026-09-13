package kse

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"os"
	"strings"
	"testing"
	"time"
)

const ballTicket = "https://www.ticketmaster.com/test/event/1E006434E7FB9F65"

func ballCard(title, date, genre, status, detail string) string {
	return `<div class="card-wrap" data-subgenre="` + genre + `"><h5 class="card-title">` + title + `</h5><span class="datetime">` + date + `</span><div class="status-code">` + status + `</div><a href="` + ballTicket + `?utm_source=test">Find Tickets</a>` + detail + `</div>`
}
func ballFixture(t *testing.T) (artifact.SourceConfig, map[string]any, map[string]any) {
	t.Helper()
	b, err := os.ReadFile("testdata/ball-arena.yaml")
	if err != nil {
		t.Fatal(err)
	}
	c, err := artifact.DecodeConfigYAML(b)
	if err != nil {
		t.Fatal(err)
	}
	row := map[string]any{"title": "Test & Friends", "url": ballTicket, "start": "2027-01-12T19:00:00", "end": nil}
	listing := ballCard("Test &amp; Friends", "Tue • Jan 12 2027 • 7:00 PM", "Pop", "", `<a href="/event-pages/test/">More Info</a>`)
	return c, map[string]any{"events": []any{row}, "check": []any{row}, "listing": listing, "listing_check": listing}, row
}
func ballDecode(t *testing.T, c artifact.SourceConfig, s map[string]any) (artifact.Refresh, error) {
	t.Helper()
	b, _ := json.Marshal(s)
	return DecodeBall(c, b, time.Date(2026, 9, 11, 20, 0, 0, 0, time.UTC))
}
func TestBallMapping(t *testing.T) {
	c, s, _ := ballFixture(t)
	r, err := ballDecode(t, c, s)
	if err != nil {
		t.Fatal(err)
	}
	o, err := (&artifact.Reconciler{}).Reconcile(c, nil, r, "2026-09-11")
	if err != nil || len(o.Rejected) != 0 || len(o.Artifact.Events) != 1 {
		t.Fatal(err, o)
	}
	e := o.Artifact.Events[0]
	if e.ShowAt != "2027-01-12T19:00:00-07:00" || e.DoorsAt != "" || e.EventURL != "https://www.ballarena.com/event-pages/test/" || e.TicketURL != ballTicket || e.Price != nil {
		t.Fatal(e)
	}
	if p := artifact.EffectivePolicy(o.Artifact.Venue, e); p.WithAdult != nil || p.Category != "" {
		t.Fatal("invented concert clearance", p)
	}
}
func TestBallCoverage(t *testing.T) {
	for _, kind := range []string{"changed", "missing", "duplicate", "html-gap", "dated-extra", "listing-change"} {
		t.Run(kind, func(t *testing.T) {
			c, s, row := ballFixture(t)
			switch kind {
			case "changed":
				s["check"] = []any{}
			case "missing":
				delete(s, "listing_check")
			case "duplicate":
				s["events"] = []any{row, row}
				s["check"] = s["events"]
			case "html-gap":
				s["listing"] = "<html>broken</html>"
				s["listing_check"] = s["listing"]
			case "dated-extra":
				s["events"] = []any{}
				s["check"] = s["events"]
			case "listing-change":
				s["listing_check"] = strings.ReplaceAll(s["listing"].(string), "Test", "Changed")
			}
			if _, err := ballDecode(t, c, s); err == nil {
				t.Fatal("accepted incomplete capture")
			}
		})
	}
}
func TestBallGamePolicyAndOverride(t *testing.T) {
	for _, override := range []bool{false, true} {
		c, s, row := ballFixture(t)
		row["title"] = "Denver Nuggets vs. Test"
		s["listing"] = ballCard(row["title"].(string), "Tue • Jan 12 2027 • 7:00 PM", "NBA", "", "")
		s["listing_check"] = s["listing"]
		if override {
			c.Overrides = []artifact.Override{{Match: artifact.Match{UpstreamID: "1E006434E7FB9F65", Date: "2027-01-12", VenueKey: "ball-arena"}, Set: map[string]json.RawMessage{"admission_policy": json.RawMessage(`{"text":"No clearance","category":"18+"}`)}}}
		}
		r, err := ballDecode(t, c, s)
		if err != nil {
			t.Fatal(err)
		}
		o, err := (&artifact.Reconciler{}).Reconcile(c, nil, r, "2026-09-11")
		if err != nil || len(o.Artifact.Events) != 1 {
			t.Fatal(err, o)
		}
		p := artifact.EffectivePolicy(o.Artifact.Venue, o.Artifact.Events[0])
		if override {
			if p.WithAdult != nil || p.Category != "18+" {
				t.Fatal(p)
			}
		} else if p.Category != "All ages" || p.WithAdult == nil || p.WithAdult.Ranges[0].MaxAge != 2 || p.WithAdult.Ranges[1].MinAge != 3 || p.WithAdult.Ranges[1].MaxAge != 17 {
			t.Fatal(p)
		}
	}
}
func TestBallTBAAndDateOnlyPass(t *testing.T) {
	c, s, row := ballFixture(t)
	row["start"] = "2026-09-18"
	row["end"] = "2026-09-20"
	s["listing"] = ballCard("Test &amp; Friends", "Fri • Sep 18 2026 - Sat • Sep 19 2026", "Bluegrass", "", "")
	s["listing_check"] = s["listing"]
	r, err := ballDecode(t, c, s)
	if err != nil {
		t.Fatal(err)
	}
	o, err := (&artifact.Reconciler{}).Reconcile(c, nil, r, "2026-09-11")
	if err != nil || len(o.Artifact.Events) != 1 || o.Artifact.Events[0].Date != "2026-09-18" || o.Artifact.Events[0].ShowAt != "" {
		t.Fatal(err, o)
	}
	s["events"] = []any{}
	s["check"] = s["events"]
	s["listing"] = ballCard("Test", "TBA", "NBA", "", "")
	s["listing_check"] = s["listing"]
	r, err = ballDecode(t, c, s)
	if err != nil || len(r.Observations) != 1 || r.Observations[0].Failure == "" {
		t.Fatal(err, r)
	}
}
func TestBallInvalidAndCancelled(t *testing.T) {
	for _, kind := range []string{"date", "link", "status", "cancelled", "title"} {
		t.Run(kind, func(t *testing.T) {
			c, s, row := ballFixture(t)
			switch kind {
			case "date":
				row["start"] = "2027-01-13T19:00:00"
			case "link":
				s["listing"] = strings.ReplaceAll(s["listing"].(string), "/event-pages/test/", "https://evil.invalid/event/")
			case "status":
				s["listing"] = strings.ReplaceAll(s["listing"].(string), `class="status-code">`, `class="status-code">Unrecognized`)
			case "cancelled":
				s["listing"] = strings.ReplaceAll(s["listing"].(string), `class="status-code">`, `class="status-code">Cancelled`)
			case "title":
				row["title"] = ""
			}
			s["listing_check"] = s["listing"]
			r, err := ballDecode(t, c, s)
			if err != nil {
				t.Fatal(err)
			}
			o, err := (&artifact.Reconciler{}).Reconcile(c, nil, r, "2026-09-11")
			if err != nil {
				t.Fatal(err)
			}
			if kind == "cancelled" {
				if len(o.Artifact.Events) != 1 || o.Artifact.Events[0].Status != "Cancelled" {
					t.Fatal(o)
				}
			} else if len(o.Rejected) != 1 {
				t.Fatal(o)
			}
		})
	}
}
