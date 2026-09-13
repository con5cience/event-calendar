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

func TestCliqueReplayPublicationAndRetention(t *testing.T) {
	dir := t.TempDir()
	var out, errout bytes.Buffer
	args := []string{"replay-clique", "--store", dir, "--config", "../../internal/clique/testdata/red-rocks.yaml", "--snapshot", "../../internal/clique/testdata/red-rocks.json", "--now", "2026-09-10T23:00:00Z"}
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
		t.Fatalf("%v %+v", err, arts)
	}
	event := arts[0].Events[0]
	handler := calendar.NewHandler(calendar.Config{DataDir: dir, Now: func() time.Time { return time.Date(2026, 9, 10, 23, 0, 0, 0, time.UTC) }})
	for _, path := range []string{"/api/calendar", "/api" + event.PublicPath, "/api" + event.PublicPath + ".ics"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		body := w.Body.String()
		if w.Code != 200 || strings.Contains(body, `"price"`) {
			t.Fatal(path, w.Code, body)
		}
		if path == "/api/calendar" && !strings.Contains(body, "All ages event; ticket required") {
			t.Fatal(body)
		}
		if strings.HasSuffix(path, ".ics") && !strings.Contains(body, "BEGIN:VEVENT") {
			t.Fatal(body)
		}
	}
	cfgRaw, _ := os.ReadFile(args[4])
	cfg, err := artifact.DecodeConfigYAML(cfgRaw)
	if err != nil {
		t.Fatal(err)
	}
	cfg.State = "established"
	cfgRaw, _ = json.Marshal(cfg)
	cfgPath := filepath.Join(t.TempDir(), "source.yaml")
	if err := os.WriteFile(cfgPath, cfgRaw, 0600); err != nil {
		t.Fatal(err)
	}
	args[4] = cfgPath
	raw, _ := os.ReadFile(args[6])
	var snap map[string]any
	json.Unmarshal(raw, &snap)
	snap["upcoming"].([]any)[0].(map[string]any)["acf"].(map[string]any)["event_doors_open"] = []string{"bad time"}
	snapPath := filepath.Join(t.TempDir(), "snapshot.json")
	save := func() {
		t.Helper()
		b, _ := json.Marshal(snap)
		if err := os.WriteFile(snapPath, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	save()
	args[6] = snapPath
	replay()
	_, arts, err = store.Load(root)
	if err != nil || !arts[0].Events[0].Listed || arts[0].Events[0].PublicPath != event.PublicPath || !strings.Contains(out.String(), "rejected") {
		t.Fatal("invalid update lost prior", err, &out)
	}
	before, _ := os.ReadFile(filepath.Join(dir, "catalog.json"))
	delete(snap, "windows")
	save()
	if run(args, &out, &errout) == 0 {
		t.Fatal("accepted failed enumeration")
	}
	after, _ := os.ReadFile(filepath.Join(dir, "catalog.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("failed job changed catalog")
	}
	snap["upcoming"] = []any{}
	snap["range"] = []any{}
	snap["windows"] = []any{map[string]any{"start": "2026-09-10", "end": "2027-09-11", "events": []any{}}}
	save()
	replay()
	_, arts, err = store.Load(root)
	if err != nil || arts[0].Events[0].Listed || arts[0].Events[0].PublicPath != event.PublicPath {
		t.Fatal("absence did not unlist and retain URL", err)
	}
}
