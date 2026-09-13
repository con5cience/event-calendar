package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ics "github.com/arran4/golang-ical"
)

func getResponse(h http.Handler, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	return w
}

func TestEventRoutesUseRetainedIdentityAndReadTimeExpiry(t *testing.T) {
	_, root := fixtureServer(t)
	dir := filepath.Join(root, "data")
	a := sourceFixture(t, "gothic")
	path := a.Events[0].PublicPath
	publishFixture(t, dir, "one", a)
	now := fixedNow()
	h := NewHandler(Config{DataDir: dir, AssetsDir: filepath.Join(root, "dist"), Now: func() time.Time { return now }})
	for _, route := range []string{path, "/api" + path, path + ".ics"} {
		if w := getResponse(h, route); w.Code != 200 {
			t.Fatalf("%s: %d %s", route, w.Code, w.Body.String())
		}
	}
	if w := getResponse(h, path); !strings.Contains(w.Body.String(), "calendar shell") {
		t.Fatal("event route must serve calendar shell")
	}
	var listing struct {
		Events []map[string]any `json:"events"`
	}
	if err := json.Unmarshal(calendarResponse(h).Body.Bytes(), &listing); err != nil {
		t.Fatal(err)
	}
	if listing.Events[0]["public_path"] != path || listing.Events[0]["listed"] != true {
		t.Fatal("missing route metadata")
	}
	a.Events[0].Title = "Renamed show"
	a.Events[0].Listed = false
	publishFixture(t, dir, "two", a)
	if len(responseEvents(t, h)) != 0 {
		t.Fatal("unlisted event leaked into calendar")
	}
	w := getResponse(h, "/api"+path)
	var detail map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || detail["title"] != "Renamed show" || detail["listed"] != false || detail["public_path"] != path {
		t.Fatalf("retained detail: %s", w.Body.String())
	}
	writeTestFile(t, dir, "catalog.json", []byte(`{`))
	for _, route := range []string{path, "/api" + path, path + ".ics"} {
		if w := getResponse(h, route); w.Code != 200 {
			t.Fatalf("last valid %s: %d", route, w.Code)
		}
		cold := NewHandler(Config{DataDir: dir, Now: fixedNow})
		if w := getResponse(cold, route); w.Code != 503 {
			t.Fatalf("cold failure %s: %d", route, w.Code)
		}
	}
	now = time.Date(2026, 12, 9, 6, 59, 0, 0, time.UTC)
	if getResponse(h, path).Code != 200 {
		t.Fatal("expired before venue midnight")
	}
	now = now.Add(time.Minute)
	for _, route := range []string{path, "/api" + path, path + ".ics", "/events/gothic/unknown"} {
		if w := getResponse(h, route); w.Code != 404 {
			t.Fatalf("expired/unknown %s: %d", route, w.Code)
		}
	}
}

func TestSingleEventCalendarDownload(t *testing.T) {
	dir := t.TempDir()
	a := sourceFixture(t, "gothic")
	publishFixture(t, dir, "one", a)
	h := catalogHandler(dir, fixedNow)
	path := a.Events[0].PublicPath + ".ics"
	w := getResponse(h, path)
	if w.Code != 200 {
		t.Fatalf("ICS: %d %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Content-Type") != "text/calendar; charset=utf-8" || !strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment;") {
		t.Fatal(w.Header())
	}
	body := w.Body.String()
	parsed, err := ics.ParseCalendar(strings.NewReader(body))
	if err != nil || len(parsed.Events()) != 1 {
		t.Fatalf("parse export: %v", err)
	}
	for _, required := range []string{"BEGIN:VCALENDAR\r\n", "BEGIN:VEVENT\r\n", "UID:gothic-012345abcdef@event-calendar\r\n", "DTSTART:20260911T010000Z\r\n", "SUMMARY:Test show\r\n", "URL:https://example.com/event\r\n", "https://example.com/tickets?id=123", "STATUS:CONFIRMED\r\n"} {
		if !strings.Contains(body, required) {
			t.Fatalf("missing %q: %s", required, body)
		}
	}
	if strings.Count(body, "BEGIN:VEVENT") != 1 || strings.Contains(body, "DTEND") || strings.Contains(body, "METHOD:REQUEST") {
		t.Fatal(body)
	}
	a.Events[0].DoorsAt = ""
	a.Events[0].Status = "canceled"
	publishFixture(t, dir, "two", a)
	body = getResponse(h, path).Body.String()
	if !strings.Contains(body, "DTSTART:20260911T020000Z") || !strings.Contains(body, "STATUS:CANCELLED") {
		t.Fatal(body)
	}
	a.Events[0].ShowAt = ""
	publishFixture(t, dir, "three", a)
	body = getResponse(h, path).Body.String()
	if !strings.Contains(body, "DTSTART;VALUE=DATE:20260910") || !strings.Contains(body, "Time not provided") || strings.Contains(body, "DTEND") {
		t.Fatal(body)
	}
}
