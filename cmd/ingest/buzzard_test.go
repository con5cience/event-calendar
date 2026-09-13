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

func TestBuzzardReplayRetentionAndAPI(t *testing.T) {
	dir := t.TempDir()
	b, _ := os.ReadFile("../../internal/buzzard/testdata/calendar.html")
	c := string(b)
	b, _ = os.ReadFile("../../internal/buzzard/testdata/home.html")
	h := string(b)
	snapshot := filepath.Join(t.TempDir(), "snapshot.json")
	save := func(c, h string) {
		t.Helper()
		p := map[string]string{"calendar": c, "home": h}
		b, _ := json.Marshal(map[string]any{"from": "2026-09-12", "through": "2027-09-12", "pages": p, "check": p})
		if err := os.WriteFile(snapshot, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	save(c, h)
	args := []string{"replay-buzzard", "--store", dir, "--config", "../../internal/buzzard/testdata/black-buzzard.yaml", "--snapshot", snapshot, "--now", "2026-09-12T18:00:00Z"}
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
		if w.Code != 200 || strings.Contains(w.Body.String(), `"price"`) || strings.Contains(w.Body.String(), `"with_adult"`) {
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
	save(strings.ReplaceAll(c, "Fixture &amp; Friends", ""), strings.ReplaceAll(h, "Fixture &amp; Friends", ""))
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
	save(strings.ReplaceAll(c, "12345", "12346"), strings.ReplaceAll(h, "12345", "12346"))
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
