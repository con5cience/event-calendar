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

func TestPlotReplayAndRetention(t *testing.T) {
	dir := t.TempDir()
	args := []string{"replay-plot", "--store", dir, "--config", "../../internal/plot/testdata/hi-dive.yaml", "--snapshot", "../../internal/plot/testdata/hi-dive.json", "--now", "2026-09-11T18:00:00Z"}
	var out, errout bytes.Buffer
	replay := func() {
		t.Helper()
		out.Reset()
		if code := run(args, &out, &errout); code != 0 {
			t.Fatalf("%d %s %s", code, &out, &errout)
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
	checkAPI := func() {
		t.Helper()
		for _, path := range []string{"/api/calendar", "/api" + event.PublicPath, "/api" + event.PublicPath + ".ics"} {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
			body := w.Body.String()
			if w.Code != 200 || strings.Contains(body, `"price"`) {
				t.Fatal(path, w.Code, body)
			}
			if path == "/api/calendar" && !strings.Contains(body, "Parent or legal guardian required") {
				t.Fatal(body)
			}
			if strings.HasSuffix(path, ".ics") && !strings.Contains(body, "BEGIN:VEVENT") {
				t.Fatal(body)
			}
		}
	}
	checkAPI()
	cfgRaw, _ := os.ReadFile(args[4])
	cfg, err := artifact.DecodeConfigYAML(cfgRaw)
	if err != nil {
		t.Fatal(err)
	}
	cfg.State = "established"
	cfgRaw, _ = json.Marshal(cfg)
	args[4] = filepath.Join(t.TempDir(), "config.yaml")
	if err = os.WriteFile(args[4], cfgRaw, 0600); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(args[6])
	var snap map[string]any
	if err = json.Unmarshal(raw, &snap); err != nil {
		t.Fatal(err)
	}
	args[6] = filepath.Join(t.TempDir(), "snapshot.json")
	save := func() {
		t.Helper()
		b, _ := json.Marshal(snap)
		if err := os.WriteFile(args[6], b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	row := snap["pages"].([]any)[0].([]any)[0].(map[string]any)
	row["doors"] = "invalid"
	snap["first_check"] = []any{row}
	save()
	replay()
	_, arts, err = store.Load(root)
	if err != nil || !arts[0].Events[0].Listed || arts[0].Events[0].PublicPath != event.PublicPath {
		t.Fatal("invalid update lost prior", err)
	}
	before, _ := os.ReadFile(filepath.Join(dir, "catalog.json"))
	snap["terminal"] = nil
	save()
	if run(args, &out, &errout) == 0 {
		t.Fatal("accepted incomplete capture")
	}
	after, _ := os.ReadFile(filepath.Join(dir, "catalog.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("failed capture changed catalog")
	}
	// Unknown event restriction must expose the reviewed venue fallback in the real API.
	snap["terminal"] = []any{}
	row["doors"] = "Doors: 8pm"
	delete(row, "description")
	save()
	replay()
	checkAPI()
	snap["pages"] = []any{[]any{}}
	snap["first_check"] = []any{}
	save()
	replay()
	_, arts, err = store.Load(root)
	if err != nil || arts[0].Events[0].Listed || arts[0].Events[0].PublicPath != event.PublicPath {
		t.Fatal("absence must unlist and retain link", err)
	}
}
