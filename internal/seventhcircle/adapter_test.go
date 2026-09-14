package seventhcircle

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"fmt"
	"strings"
	"testing"
	"time"
)

const fixturePolicy = `<main>Seventh Circle is committed to welcoming all ages at every show. $5 a year for membership.</main>`

func fixtureCard(id, title string) string {
	return fmt.Sprintf(`<div id="post_%s" class="post-container"><a class="post-link" href="/posts/%s"></a><p class="event-title">%s</p><span class="day">Sat</span><span class="date">19</span><span class="month">Sep</span><p class="time">6:00 PM</p></div>`, id, id, title)
}
func fixtureDetail(title string) string {
	return `<div class="post-show-container"><p class="event-title">` + title + `</p></div><p class="event-time">September 19, 2026 @ 6:00 PM</p><p class="membership-required">$5 annual membership required</p>`
}
func config(t *testing.T) artifact.SourceConfig {
	t.Helper()
	b := []byte(`{"schema_version":1,"source":{"id":"seventh-circle","adapter":"html-seventh-circle"},"state":"new","venue":{"key":"seventh-circle","name":"Seventh Circle Music Collective","timezone":"America/Denver","website":"https://www.7thcirclemusiccollective.org","admission_policy":{"text":"All ages","category":"All ages","url":"https://www.7thcirclemusiccollective.org/home/about"}},"admission_rules":{"All ages":{"url":"https://www.7thcirclemusiccollective.org/home/about","reviewed_on":"2026-09-14","ranges":[{"min_age":0,"max_age":17,"condition":"$5 annual fee; show donations encouraged"}]}}}`)
	cfg, err := artifact.DecodeConfigJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}
func TestMapping(t *testing.T) {
	for _, kind := range []string{"normal", "doors", "mismatch", "missing", "policy", "empty", "duplicate", "pagination", "changed", "invalid-date", "restricted", "cancelled", "override", "removal", "failure"} {
		t.Run(kind, func(t *testing.T) {
			cfg := config(t)
			title := "Fixture &amp; Friends"
			if kind == "doors" {
				title = "Fixture &amp; Friends 6pm doors!"
			}
			if kind == "restricted" {
				title = "Fixture 21+"
			}
			if kind == "cancelled" {
				title = "CANCELLED: Fixture"
			}
			p := pages{Calendar: `<h1>upcoming events</h1><div id="posts">` + fixtureCard("12", title) + fixtureCard("13", "Second Band") + `</div><a href="/posts/past">Past Shows</a>`, Policy: fixturePolicy, Details: map[string]string{"12": fixtureDetail(title), "13": fixtureDetail("Second Band")}}
			switch kind {
			case "mismatch":
				p.Details["12"] = fixtureDetail("Different artist")
			case "missing":
				p.Details["12"] = ""
			case "policy":
				p.Policy = "All shows are 21+"
			case "empty":
				p.Calendar = `<h1>upcoming events</h1><div id="posts"></div>`
			case "duplicate":
				p.Calendar += fixtureCard("12", title)
			case "pagination":
				p.Calendar += `<a href="/posts/?page=2" rel="next">Next</a>`
			case "invalid-date":
				p.Details["12"] = strings.ReplaceAll(p.Details["12"], "September 19, 2026", "September 99, 2026")
			case "override":
				cfg.Overrides = []artifact.Override{{Match: artifact.Match{UpstreamID: "12", Date: "2026-09-19", VenueKey: "seventh-circle"}, Set: map[string]json.RawMessage{"admission_policy": json.RawMessage(`{"text":"Override","category":"21+"}`)}}}
			}
			check := p
			if kind == "changed" {
				check.Policy = "changed"
			}
			now := time.Date(2026, 9, 14, 18, 0, 0, 0, time.UTC)
			raw, _ := json.Marshal(map[string]any{"from": "2026-09-14", "through": "2027-09-14", "pages": p, "check": check})
			run, err := Decode(cfg, raw, now)
			if kind == "policy" || kind == "empty" || kind == "duplicate" || kind == "pagination" || kind == "changed" {
				if err == nil {
					t.Fatal("bad capture accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			out, err := (&artifact.Reconciler{}).Reconcile(cfg, nil, run, "2026-09-14")
			if err != nil {
				t.Fatal(err)
			}
			if kind == "missing" || kind == "mismatch" || kind == "invalid-date" {
				if len(out.Rejected) != 1 || len(out.Artifact.Events) != 1 {
					t.Fatal(out)
				}
				return
			}
			if len(out.Rejected) != 0 || len(out.Artifact.Events) != 2 {
				t.Fatal(out)
			}
			for _, e := range out.Artifact.Events {
				if e.UpstreamID != "12" {
					continue
				}
				if e.Price != nil || e.TicketURL != "" || e.ShowAt != "" {
					t.Fatal(e)
				}
				if (e.DoorsAt != "") != (kind == "doors") {
					t.Fatal(e)
				}
				if kind == "doors" && (e.Title != "Fixture & Friends" || e.DoorsAt != "2026-09-19T18:00:00-06:00") {
					t.Fatal(e)
				}
				pol := artifact.EffectivePolicy(out.Artifact.Venue, e)
				if (pol.WithAdult != nil) != (kind != "restricted" && kind != "override" && kind != "cancelled") {
					t.Fatal(pol)
				}
				if kind == "cancelled" && e.Status != "Cancelled" {
					t.Fatal(e)
				}
			}
			if kind == "removal" || kind == "failure" {
				cfg.State = "established"
				if kind == "removal" {
					run.Observations = run.Observations[1:]
				} else {
					run.Failure = "network failed"
				}
				next, err := (&artifact.Reconciler{}).Reconcile(cfg, &out.Artifact, run, "2026-09-14")
				if kind == "failure" {
					if err == nil || len(out.Artifact.Events) != 2 {
						t.Fatal("failed refresh must not produce replacement", err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				for _, e := range next.Artifact.Events {
					if e.UpstreamID == "12" && e.Listed != (kind == "failure") {
						t.Fatal(e)
					}
				}
			}
		})
	}
}
