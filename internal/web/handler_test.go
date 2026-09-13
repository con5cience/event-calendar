package web

import (
	"encoding/json"
	"event-calendar/internal/locale"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureServer(t *testing.T) (http.Handler, string) {
	t.Helper()
	root := t.TempDir()
	assets := filepath.Join(root, "dist")
	if err := os.Mkdir(assets, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "index.html"), []byte(`<html><head><title>calendar shell</title></head><body><div id="root"></div><script src="/assets/app.js"></script><!-- calendar shell --></body></html>`), 0644); err != nil {
		t.Fatal(err)
	}
	return NewHandler(Config{Site: testSite(), DataDir: filepath.Join(root, "data"), AssetsDir: assets, WeekLimit: 10, MonthLimit: 5}), root
}
func testSite() locale.Config {
	b, err := os.ReadFile("../../tests/contracts/site.json")
	if err != nil {
		panic(err)
	}
	c, err := locale.Decode(b)
	if err != nil {
		panic(err)
	}
	c.Sources = nil // These existing tests exercise catalog validation independently.
	return c
}
func TestEmptyDirectoryReturnsEmptyCalendar(t *testing.T) {
	h, root := fixtureServer(t)
	if err := os.Mkdir(filepath.Join(root, "data"), 0755); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/calendar", nil))
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var data struct {
		Events []json.RawMessage `json:"events"`
		Limits map[string]any    `json:"limits"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if data.Events == nil || len(data.Events) != 0 || data.Limits["week"] != float64(10) {
		t.Fatalf("unexpected data: %s", w.Body.String())
	}
}
func TestReadsCatalogAndRejectsInvalidInput(t *testing.T) {
	_, root := fixtureServer(t)
	dir := filepath.Join(root, "data")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "catalog.json")
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"valid", `{"schema_version":1,"generation":"empty","generated_at":"2026-09-08T12:00:00Z","sources":[]}`, 200},
		{"invalid-json", `{"sources":`, 503},
		{"unknown-version", `{"schema_version":2,"generation":"empty","generated_at":"2026-09-08T12:00:00Z","sources":[]}`, 503},
		{"prototype-format", `{"events":[]}`, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := catalogHandler(dir, fixedNow)
			if err := os.WriteFile(path, []byte(tc.body), 0644); err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("GET", "/api/calendar", nil))
			if w.Code != tc.status {
				t.Fatalf("status %d want %d: %s", w.Code, tc.status, w.Body.String())
			}
		})
	}
}
func TestRoutesDoNotExposeArtifactsOrInventSPARoutes(t *testing.T) {
	h, _ := fixtureServer(t)
	for _, tc := range []struct {
		path   string
		status int
	}{{"/", 200}, {"/healthz", 200}, {"/api/missing", 404}, {"/events.json", 404}, {"/nonexistent.js", 404}, {"/events/unknown", 503}} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
		if w.Code != tc.status {
			t.Errorf("%s = %d want %d", tc.path, w.Code, tc.status)
		}
	}
}
func TestConfigDefaultsAndValidation(t *testing.T) {
	t.Setenv("SITE_DIR", "../../tests/contracts")
	t.Setenv("WEEK_EVENT_LIMIT", "3")
	t.Setenv("MONTH_EVENT_LIMIT", "2")
	t.Setenv("DAY_EVENT_LIMIT", "0")
	c, err := ConfigFromEnv()
	if err != nil || c.WeekLimit != 3 || c.MonthLimit != 2 || c.DayLimit != 0 {
		t.Fatalf("config %#v err %v", c, err)
	}
	t.Setenv("WEEK_EVENT_LIMIT", "-1")
	if _, err = ConfigFromEnv(); err == nil || !strings.Contains(err.Error(), "WEEK_EVENT_LIMIT") {
		t.Fatal("expected explicit invalid configuration error")
	}
}

func TestRejectsOversizedArtifact(t *testing.T) {
	_, root := fixtureServer(t)
	dir := filepath.Join(root, "data")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	h := catalogHandler(dir, fixedNow)
	body := `{"sources":[]}` + strings.Repeat(" ", 4<<20) + `{}`
	if err := os.WriteFile(filepath.Join(dir, "catalog.json"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/calendar", nil))
	if w.Code != 503 {
		t.Fatalf("oversized artifact returned %d", w.Code)
	}
}
