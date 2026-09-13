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

func TestOpheliasReplayRetentionAndAPI(t *testing.T) {
	dir := t.TempDir()
	b, _ := os.ReadFile("../../internal/ophelias/testdata/listing.html")
	h := string(b)
	snap := filepath.Join(t.TempDir(), "snapshot.json")
	save := func(page, check string) {
		t.Helper()
		b, _ := json.Marshal(map[string]string{"from": "2026-09-11", "through": "2027-09-11", "html": page, "check_html": check})
		if err := os.WriteFile(snap, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	save(h, h)
	args := []string{"replay-ophelias", "--store", dir, "--config", "../../internal/ophelias/testdata/ophelias.yaml", "--snapshot", snap, "--now", "2026-09-11T18:00:00Z"}
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
	handler := calendar.NewHandler(calendar.Config{DataDir: dir, Now: func() time.Time { return time.Date(2026, 9, 11, 18, 0, 0, 0, time.UTC) }})
	for _, path := range []string{"/api/calendar", "/api" + event.PublicPath, "/api" + event.PublicPath + ".ics"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 || strings.Contains(w.Body.String(), `"price"`) {
			t.Fatal(w.Code, w.Body.String())
		}
		if path == "/api/calendar" && !strings.Contains(w.Body.String(), `"with_adult"`) {
			t.Fatal(w.Body.String())
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
	invalid := strings.ReplaceAll(h, "Fixture &amp; Friends", "")
	save(invalid, invalid)
	replay()
	_, arts, err = store.Load(root)
	if err != nil || !arts[0].Events[0].Listed || arts[0].Events[0].PublicPath != event.PublicPath {
		t.Fatal("last valid lost", err)
	}
	before, _ := os.ReadFile(filepath.Join(dir, "catalog.json"))
	for _, bad := range []string{"", strings.Replace(h, "Upcoming Events", "No events", 1)} {
		save(bad, bad)
		if run(args, &out, &errout) == 0 {
			t.Fatal("unverified empty accepted")
		}
		after, _ := os.ReadFile(filepath.Join(dir, "catalog.json"))
		if !bytes.Equal(before, after) {
			t.Fatal("failure changed catalog")
		}
	}
	newer := strings.ReplaceAll(h, "1E0064D0BB2EA1FC", "1E0064D0BB2EA1FD")
	save(newer, newer)
	replay()
	_, arts, err = store.Load(root)
	if err != nil || len(arts[0].Events) != 2 {
		t.Fatal(err, arts)
	}
	for _, e := range arts[0].Events {
		if e.UpstreamID == event.UpstreamID && (e.Listed || e.PublicPath != event.PublicPath) {
			t.Fatal("missing record retention", e)
		}
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/api"+event.PublicPath, nil))
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
}
