package web

import (
	"encoding/xml"
	"event-calendar/internal/locale"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSEOHomepageSortsWithoutChangingInput(t *testing.T) {
	_, root := fixtureServer(t)
	events := []Event{
		{ID: "later-date", Date: "2026-09-13", Title: "Later date"},
		{ID: "later-time", Date: "2026-09-12", DoorsAt: "2026-09-12T20:00:00-06:00", Title: "Later time"},
		{ID: "venue-z", Date: "2026-09-12", ShowAt: "2026-09-12T19:00:00-06:00", Venue: "Z Hall", Title: "A show"},
		{ID: "title-10", Date: "2026-09-12", DoorsAt: "2026-09-12T19:00:00-06:00", Venue: "A Hall", Title: "Show 10"},
		{ID: "title-2", Date: "2026-09-12", DoorsAt: "2026-09-12T19:00:00-06:00", ShowAt: "2026-09-12T21:00:00-06:00", Venue: "A Hall", Title: "Show 2"},
		{ID: "untimed-z", Date: "2026-09-12", Venue: "Z Hall", Title: "Unknown time"},
		{ID: "untimed-a", Date: "2026-09-12", Venue: "A Hall", Title: "Unknown time"},
		{ID: "earlier-date", Date: "2026-09-11", Title: "Earlier date"},
	}
	for i := range events {
		events[i].PublicPath = "/events/" + events[i].ID
		events[i].Timezone = "America/Denver"
	}
	w := httptest.NewRecorder()
	servePage(w, httptest.NewRequest("GET", "/", nil), filepath.Join(root, "dist"), pageMetadata{Site: locale.Config{Presentation: locale.Presentation{Language: "en-US"}}, Events: events})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	last := -1
	for _, id := range []string{"earlier-date", "untimed-a", "untimed-z", "title-2", "title-10", "venue-z", "later-time", "later-date"} {
		index := strings.Index(w.Body.String(), `href="/events/`+id+`"`)
		if index <= last {
			t.Fatalf("event %s out of order: %s", id, w.Body.String())
		}
		last = index
	}
	if events[0].ID != "later-date" {
		t.Fatal("rendering mutated caller input")
	}
}

func TestSEOInitialHTMLAndDiscovery(t *testing.T) {
	_, root := fixtureServer(t)
	dir := filepath.Join(root, "data")
	a := sourceFixture(t, "gothic")
	a.Events[0].Title = `Test </title><script>alert("x")</script> & show`
	publishFixture(t, dir, "seo", a)
	now := fixedNow()
	h := NewHandler(Config{Site: testSite(), DataDir: dir, AssetsDir: filepath.Join(root, "dist"), Now: func() time.Time { return now }})
	path := a.Events[0].PublicPath
	for _, route := range []string{"/", path} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "http://attacker.example"+route+"?tracking=1", nil)
		h.ServeHTTP(w, r)
		body := w.Body.String()
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", route, w.Code, body)
		}
		for _, want := range []string{`rel="canonical" href="https://denver.withadult.com` + route + `"`, `property="og:title"`, `property="og:image"`, `name="twitter:card"`, `name="description"`, `src="/assets/app.js"`, "Gothic Theatre", "&lt;script&gt;"} {
			if !strings.Contains(body, want) {
				t.Errorf("%s missing %s", route, want)
			}
		}
		if strings.Contains(body, "attacker.example") || strings.Contains(body, "tracking=1") || strings.Contains(body, `<script>alert`) || strings.Count(body, "<title>") != 1 {
			t.Errorf("unsafe or duplicate metadata: %s", body)
		}
		if route == "/" && !strings.Contains(body, `href="`+path+`"`) {
			t.Error("missing crawlable event link")
		}
	}
	w := getResponse(h, "/sitemap.xml")
	var sitemap struct {
		URLs []struct {
			Location string `xml:"loc"`
		} `xml:"url"`
	}
	if w.Code != 200 || xml.Unmarshal(w.Body.Bytes(), &sitemap) != nil || len(sitemap.URLs) != 2 || sitemap.URLs[1].Location != "https://denver.withadult.com"+path {
		t.Fatalf("sitemap: %d %s", w.Code, w.Body.String())
	}
	if w := getResponse(h, "/robots.txt"); w.Code != 200 || !strings.Contains(w.Body.String(), "Sitemap: https://denver.withadult.com/sitemap.xml") {
		t.Fatal("missing robots sitemap")
	}
	a.Events[0].Listed = false
	publishFixture(t, dir, "removed", a)
	if w := getResponse(h, path); w.Code != 200 || !strings.Contains(w.Body.String(), `name="robots" content="noindex"`) {
		t.Fatal("retained unlisted event should remain accessible but noindex")
	}
	if strings.Contains(getResponse(h, "/sitemap.xml").Body.String(), path) {
		t.Fatal("unlisted event in sitemap")
	}
	now = time.Date(2026, 12, 10, 0, 0, 0, 0, time.UTC)
	if getResponse(h, path).Code != 404 {
		t.Fatal("expired event must remain 404")
	}
}

func TestSEODiscoveryUnavailableAndEmpty(t *testing.T) {
	h, root := fixtureServer(t)
	if getResponse(h, "/sitemap.xml").Code != 503 {
		t.Fatal("missing data must not publish an empty sitemap")
	}
	publishFixture(t, filepath.Join(root, "data"), "empty")
	if w := getResponse(h, "/"); w.Code != 200 || !strings.Contains(w.Body.String(), "No events available") {
		t.Fatal("empty homepage fallback")
	}
	if w := getResponse(h, "/sitemap.xml"); w.Code != 200 {
		t.Fatal("empty catalog sitemap")
	}
}
