package buzzard

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMapping(t *testing.T) {
	for _, kind := range []string{"normal", "21", "16", "unknown", "cancelled", "soldout", "outside", "date", "venue", "title", "changed", "missing", "empty", "pagination", "url", "override", "unrelated", "price"} {
		t.Run(kind, func(t *testing.T) {
			b, _ := os.ReadFile("testdata/black-buzzard.yaml")
			cfg, err := artifact.DecodeConfigYAML(b)
			if err != nil {
				t.Fatal(err)
			}
			b, _ = os.ReadFile("testdata/calendar.html")
			c := string(b)
			b, _ = os.ReadFile("testdata/home.html")
			h := string(b)
			switch kind {
			case "21", "16", "unknown":
				v := kind
				if kind == "unknown" {
					v = "Invitation only"
				}
				h = strings.ReplaceAll(h, "This is some text inside of a div block.", v)
			case "cancelled":
				h = strings.ReplaceAll(h, "EventScheduled", "EventCancelled")
			case "soldout":
				h = strings.ReplaceAll(h, `"price":"123"`, `"price":"123","availability":"https://schema.org/SoldOut"`)
			case "outside":
				c = strings.ReplaceAll(c, "2026", "2028")
				h = strings.ReplaceAll(h, "2026", "2028")
			case "date":
				c = strings.ReplaceAll(c, "September 12", "September 32")
				h = strings.ReplaceAll(h, "Sep 12", "Sep 32")
			case "venue":
				h = strings.ReplaceAll(h, "1624 Market St", "Elsewhere")
			case "title":
				h = strings.ReplaceAll(h, "Fixture &amp; Friends", "")
				c = strings.ReplaceAll(c, "Fixture &amp; Friends", "")
			case "missing":
				h = strings.ReplaceAll(h, "12345", "12346")
			case "empty":
				h = "<html></html>"
				c = h
			case "pagination":
				h = strings.Replace(h, "</body>", `<a class="w-pagination-next" href="?page=2">Next</a></body>`, 1)
			case "url":
				h = strings.ReplaceAll(h, "https://tixr.com/e/12345", "javascript:alert(1)")
			case "override":
				cfg.Overrides = []artifact.Override{{Match: artifact.Match{UpstreamID: "12345", Date: "2026-09-12", VenueKey: "black-buzzard"}, Set: map[string]json.RawMessage{"admission_policy": json.RawMessage(`{"text":"Override","category":"21+"}`)}}}
			case "unrelated":
				h = strings.Replace(h, "</body>", `<script type="application/ld+json">bad unrelated</script><p>All ages with guardian</p></body>`, 1)
			case "price":
				h = strings.ReplaceAll(h, `"price":"123"`, `"price":"different"`)
			}
			pages := map[string]string{"calendar": c, "home": h}
			check := pages
			if kind == "changed" {
				check = map[string]string{"calendar": c, "home": strings.ReplaceAll(h, "EventScheduled", "EventCancelled")}
			}
			b, _ = json.Marshal(map[string]any{"from": "2026-09-12", "through": "2027-09-12", "pages": pages, "check": check})
			r, err := Decode(cfg, b, time.Date(2026, 9, 12, 18, 0, 0, 0, time.UTC))
			switch kind {
			case "missing", "empty", "pagination", "url", "changed":
				if err == nil {
					t.Fatal("unsafe capture accepted")
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
			case "date", "venue", "title":
				if len(out.Rejected) != 1 {
					t.Fatal(out)
				}
				return
			case "outside":
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
			want := "18+"
			switch kind {
			case "21", "override":
				want = "21+"
			case "16":
				want = "16+"
			case "unknown":
				want = ""
			}
			if p.Category != want || p.WithAdult != nil {
				t.Fatal(p)
			}
			if e.Title != "Fixture & Friends" || e.Price != nil || e.DoorsAt != "" || e.ShowAt != "" {
				t.Fatal(e)
			}
			if kind == "cancelled" && (e.Status != "Cancelled" || e.TicketURL != "") {
				t.Fatal(e)
			}
		})
	}
}
