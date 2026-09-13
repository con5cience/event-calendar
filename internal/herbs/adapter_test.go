package herbs

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMapping(t *testing.T) {
	for _, kind := range []string{"normal", "before", "cutoff", "after", "missing-time", "winter", "spring-gap", "fall-fold", "invalid-time", "clock-conflict", "date", "changed", "empty", "policy-changed", "pagination", "id", "duplicate", "url", "title", "cancelled", "soldout", "restriction", "override", "offsite", "past", "outside"} {
		t.Run(kind, func(t *testing.T) {
			b, _ := os.ReadFile("testdata/herbs.yaml")
			cfg, err := artifact.DecodeConfigYAML(b)
			if err != nil {
				t.Fatal(err)
			}
			b, _ = os.ReadFile("testdata/calendar.html")
			h := string(b)
			clock := func(a, b string) { h = strings.ReplaceAll(h, "9:30 PM", a); h = strings.ReplaceAll(h, "21:30", b) }
			switch kind {
			case "before":
				clock("10:29 PM", "22:29")
			case "cutoff":
				clock("10:30 PM", "22:30")
			case "after":
				clock("11:00 PM", "23:00")
			case "missing-time":
				clock("", "")
			case "winter":
				h = strings.ReplaceAll(h, "2026-09-12", "2026-12-12")
			case "spring-gap":
				h = strings.ReplaceAll(h, "2026-09-12", "2027-03-14")
				clock("2:30 AM", "02:30")
			case "fall-fold":
				h = strings.ReplaceAll(h, "2026-09-12", "2026-11-01")
				clock("1:30 AM", "01:30")
			case "invalid-time":
				clock("garbage", "25:70")
			case "clock-conflict":
				clock("9:30 PM", "22:30")
			case "date":
				h = strings.ReplaceAll(h, "2026-09-12", "2026-09-32")
			case "empty":
				h = "<html></html>"
			case "policy-changed":
				h = strings.ReplaceAll(h, "10:30pm", "10:00pm")
			case "pagination":
				h = strings.Replace(h, "</body>", `<a rel="next" href="?offset=123">Next</a></body>`, 1)
			case "id":
				h = strings.ReplaceAll(h, "6a43042df1569c5e55a2b237", "")
			case "duplicate":
				start := strings.Index(h, "<article ")
				end := strings.Index(h, "</article>") + len("</article>")
				h = strings.Replace(h, "</article>", "</article>"+h[start:end], 1)
			case "url":
				h = strings.ReplaceAll(h, "/live-music-calendar-1/2017/3/29/fixture", "https://other.example/event")
			case "title":
				h = strings.ReplaceAll(h, "Fixture &amp; Friends", "")
			case "cancelled":
				h = strings.ReplaceAll(h, `class="eventlist-datetag-status"></div>`, `class="eventlist-datetag-status">Cancelled</div>`)
			case "soldout":
				h = strings.ReplaceAll(h, `class="eventlist-datetag-status"></div>`, `class="eventlist-datetag-status">Sold Out</div>`)
			case "restriction":
				h = strings.ReplaceAll(h, "All genres welcome", "Ages 18+; no minors")
			case "override":
				cfg.Overrides = []artifact.Override{{Match: artifact.Match{UpstreamID: "6a43042df1569c5e55a2b237", Date: "2026-09-12", VenueKey: "herbs"}, Set: map[string]json.RawMessage{"admission_policy": json.RawMessage(`{"text":"Override","category":"21+"}`)}}}
			case "offsite":
				h = strings.Replace(h, "</ul>", `<li class="eventlist-meta-address">Other venue</li></ul>`, 1)
			case "past":
				h = strings.ReplaceAll(h, "2026-09-12", "2026-09-10")
			case "outside":
				h = strings.ReplaceAll(h, "2026-09-12", "2028-09-12")
			}
			check := h
			if kind == "changed" {
				check = strings.ReplaceAll(h, "Fixture &amp; Friends", "Changed")
			}
			b, _ = json.Marshal(map[string]string{"from": "2026-09-12", "through": "2027-09-12", "html": h, "check_html": check})
			r, err := Decode(cfg, b, time.Date(2026, 9, 12, 18, 0, 0, 0, time.UTC))
			switch kind {
			case "changed", "empty", "policy-changed", "pagination", "id", "duplicate", "url":
				if err == nil {
					t.Fatal("unsafe snapshot accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			out, err := (&artifact.Reconciler{}).Reconcile(cfg, nil, r, "2026-09-12")
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "spring-gap", "fall-fold", "invalid-time", "clock-conflict", "date", "title", "offsite":
				if len(out.Rejected) != 1 {
					t.Fatal(out)
				}
				return
			case "past", "outside":
				if len(out.Artifact.Events) != 0 {
					t.Fatal(out)
				}
				return
			}
			if len(out.Artifact.Events) != 1 || len(out.Rejected) != 0 {
				t.Fatal(out)
			}
			e := out.Artifact.Events[0]
			p := artifact.EffectivePolicy(out.Artifact.Venue, e)
			allowed := kind != "cutoff" && kind != "after" && kind != "missing-time" && kind != "restriction" && kind != "override" && kind != "cancelled"
			if (p.WithAdult != nil) != allowed {
				t.Fatal(p)
			}
			if p.WithAdult != nil {
				if p.WithAdult.Ranges[0].Condition != "Parent required; minors must leave by 10:30 PM" || p.WithAdult.Ranges[0].MinAge != 0 || p.WithAdult.Ranges[0].MaxAge != 17 {
					t.Fatal(p)
				}
			}
			want := "2026-09-12T21:30:00-06:00"
			switch kind {
			case "before":
				want = "2026-09-12T22:29:00-06:00"
			case "cutoff":
				want = "2026-09-12T22:30:00-06:00"
			case "after":
				want = "2026-09-12T23:00:00-06:00"
			case "winter":
				want = "2026-12-12T21:30:00-07:00"
			case "missing-time":
				want = ""
			}
			if e.ShowAt != want || e.DoorsAt != "" || e.Price != nil || e.TicketURL != "" || e.Title != "Fixture & Friends" {
				t.Fatal(e)
			}
			if !strings.Contains(e.EventURL, "/2017/3/29/fixture") {
				t.Fatal(e)
			}
			if kind == "cancelled" && e.Status != "Cancelled" {
				t.Fatal(e)
			}
		})
	}
}
