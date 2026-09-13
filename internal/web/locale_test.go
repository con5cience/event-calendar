package web

import (
	"event-calendar/internal/locale"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLocaleIsExplicitAtStartup(t *testing.T) {
	t.Setenv("SITE_DIR", "")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("production startup must require an explicit locale")
	}
}

func TestForeignCatalogCannotReplaceLastValidLocale(t *testing.T) {
	dir := t.TempDir()
	a := sourceFixture(t, "gothic")
	site := testSite()
	site.Sources = map[string]locale.Source{"gothic": {Adapter: a.Source.Adapter, VenueKey: a.Venue.Key}}
	publishFixture(t, dir, "first", a)
	h := NewHandler(Config{Site: site, DataDir: dir, Now: fixedNow})
	before := calendarResponse(h)
	if before.Code != 200 {
		t.Fatal(before.Code, before.Body.String())
	}
	foreign := sourceFixture(t, "harbor")
	publishFixture(t, dir, "foreign", foreign)
	after := calendarResponse(h)
	if after.Code != 200 || after.Body.String() != before.Body.String() {
		t.Fatal(after.Code, after.Body.String())
	}
	fresh := calendarResponse(NewHandler(Config{Site: site, DataDir: dir, Now: fixedNow}))
	if fresh.Code != 503 {
		t.Fatal("foreign catalog accepted at startup", fresh.Code)
	}
}

func TestLocaleMetadataUsesSelectedConfiguration(t *testing.T) {
	h, _ := fixtureServer(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/site", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"id":"denver"`) {
		t.Fatalf("locale endpoint: %d %s", w.Code, w.Body.String())
	}
}
