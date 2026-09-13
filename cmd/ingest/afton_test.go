package main

import (
	"bytes"
	"encoding/json"
	"event-calendar/internal/artifact"
	"event-calendar/internal/store"
	calendar "event-calendar/internal/web"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAftonReplayRetentionAndAPI(t *testing.T) {
	dir := t.TempDir()
	b, _ := os.ReadFile("../../internal/afton/testdata/detail.html")
	c := string(b)
	h := c
	snapshot := filepath.Join(t.TempDir(), "snapshot.json")
	save := func(c, h string) {
		t.Helper()
		_ = h
		id := "3px8g401j1"
		if strings.Contains(c, "3px8g401j2") {
			id = "3px8g401j2"
		}
		title := "Fixture & Friends"
		if !strings.Contains(c, title) {
			title = ""
		}
		e := map[string]any{"event_type": "real_world", "event_id": id, "event_name": title, "start_time": "2026-09-26 19:00:00", "door_time": "2026-09-26 18:00:00", "venue_name": "The Roxy Theatre", "city": "Denver", "state_abbreviation": "CO", "buy_ticket_url": "https://aftontickets.com/event/buyticket/" + id, "hide_start_time": "no", "display_door_start_time": "yes", "entry_type": "door"}
		pages := []any{map[string]any{"page": 1, "total": 1, "per_page": 12, "next": false, "events": []any{e}}}
		details := map[string]string{id: c}
		b, _ := json.Marshal(map[string]any{"from": "2026-09-12", "through": "2027-09-12", "pages": pages, "check_pages": pages, "details": details, "check_details": details})
		if err := os.WriteFile(snapshot, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	save(c, h)
	args := []string{"replay-afton", "--store", dir, "--config", "../../internal/afton/testdata/roxy.yaml", "--snapshot", snapshot, "--now", "2026-09-12T18:00:00Z"}
	var out, errout bytes.Buffer
	replay := func() {
		t.Helper()
		out.Reset()
		errout.Reset()
		if code := run(args, &out, &errout); code != 0 {
			t.Fatal(code, out.String(), errout.String())
		}
	}
	replay()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	_, arts, err := store.Load(root)
	if err != nil || len(arts) != 1 || len(arts[0].Events) != 1 {
		t.Fatal(err, arts)
	}
	event := arts[0].Events[0]
	handler := calendar.NewHandler(calendar.Config{DataDir: dir, Now: func() time.Time { return time.Date(2026, 9, 12, 18, 0, 0, 0, time.UTC) }})
	for _, path := range []string{"/api/calendar", "/api" + event.PublicPath, "/api" + event.PublicPath + ".ics"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 || strings.Contains(w.Body.String(), `"price"`) {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	b, _ = os.ReadFile(args[4])
	cfg, err := artifact.DecodeConfigYAML(b)
	if err != nil {
		t.Fatal(err)
	}
	cfg.State = "established"
	b, _ = json.Marshal(cfg)
	args[4] = filepath.Join(t.TempDir(), "config.yaml")
	if err = os.WriteFile(args[4], b, 0600); err != nil {
		t.Fatal(err)
	}
	save(strings.ReplaceAll(c, "Fixture & Friends", ""), strings.ReplaceAll(h, "Fixture & Friends", ""))
	replay()
	_, arts, err = store.Load(root)
	if err != nil || arts[0].Events[0].Title != event.Title || !arts[0].Events[0].Listed {
		t.Fatal("invalid update lost prior", err)
	}
	before, _ := os.ReadFile(filepath.Join(dir, "catalog.json"))
	save("", "")
	if run(args, &out, &errout) == 0 {
		t.Fatal("empty accepted")
	}
	after, _ := os.ReadFile(filepath.Join(dir, "catalog.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("failed refresh changed catalog")
	}
	save(strings.ReplaceAll(c, "3px8g401j1", "3px8g401j2"), strings.ReplaceAll(h, "3px8g401j1", "3px8g401j2"))
	replay()
	_, arts, err = store.Load(root)
	if err != nil || len(arts[0].Events) != 2 {
		t.Fatal(err, arts)
	}
	for _, e := range arts[0].Events {
		if e.UpstreamID == event.UpstreamID && (e.Listed || e.PublicPath != event.PublicPath) {
			t.Fatal(e)
		}
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/api"+event.PublicPath, nil))
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
}
