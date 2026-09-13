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

func TestKSEReplayRetentionAndAPI(t *testing.T) {
	testReplayRetentionAndAPI(t, "kse", "paramount", false)
}

func TestBallReplayRetentionAndAPI(t *testing.T) {
	testReplayRetentionAndAPI(t, "kse", "ball-arena", false)
}

func TestMarquisReplayRetentionAndAPI(t *testing.T) {
	testReplayRetentionAndAPI(t, "livenation", "marquis", true)
}

func TestSummitReplayRetentionAndAPI(t *testing.T) {
	testReplayRetentionAndAPI(t, "livenation", "summit", true)
}

func TestFillmoreReplayRetentionAndAPI(t *testing.T) {
	testReplayRetentionAndAPI(t, "livenation", "fillmore", true)
}

func TestBlackBoxReplayRetentionAndAPI(t *testing.T) {
	testReplayRetentionAndAPI(t, "blackbox", "black-box", false)
}

func TestLevittReplayRetentionAndAPI(t *testing.T) {
	testReplayRetentionAndAPI(t, "venuepilot", "levitt", false)
}

func TestMeowWolfReplayRetentionAndAPI(t *testing.T) {
	testReplayRetentionAndAPI(t, "meowwolf", "meow-wolf-denver", false)
}

func testReplayRetentionAndAPI(t *testing.T, adapter, source string, paged bool) {
	t.Helper()
	dir := t.TempDir()
	base := "../../internal/" + adapter + "/testdata/" + source
	args := []string{"replay-" + adapter, "--store", dir, "--config", base + ".yaml", "--snapshot", base + ".json", "--now", "2026-09-11T18:00:00Z"}
	if source == "ball-arena" {
		args[0] = "replay-kse-calendar"
	}
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
	e := arts[0].Events[0]
	h := calendar.NewHandler(calendar.Config{DataDir: dir, Now: func() time.Time { return time.Date(2026, 9, 11, 18, 0, 0, 0, time.UTC) }})
	for _, path := range []string{"/api/calendar", "/api" + e.PublicPath, "/api" + e.PublicPath + ".ics"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 || strings.Contains(w.Body.String(), `"price"`) {
			t.Fatal(w.Code, w.Body.String())
		}
		if path == "/api/calendar" && !strings.Contains(w.Body.String(), `"with_adult"`) {
			t.Fatal(w.Body.String())
		}
	}
	b, _ := os.ReadFile(args[4])
	c, err := artifact.DecodeConfigYAML(b)
	if err != nil {
		t.Fatal(err)
	}
	c.State = "established"
	b, _ = json.Marshal(c)
	args[4] = filepath.Join(t.TempDir(), "config.yaml")
	if err = os.WriteFile(args[4], b, 0600); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(args[6])
	var snap map[string]any
	if err = json.Unmarshal(b, &snap); err != nil {
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
	key := "events"
	if paged {
		key = "pages"
	}
	rows := snap[key].([]any)
	if paged {
		rows = rows[0].([]any)
	}
	row := rows[0].(map[string]any)
	row["name"] = ""
	if source == "ball-arena" || source == "black-box" || source == "meow-wolf-denver" {
		row["title"] = ""
	}
	snap["check"] = snap[key]
	save()
	replay()
	_, arts, err = store.Load(root)
	if err != nil || !arts[0].Events[0].Listed || arts[0].Events[0].PublicPath != e.PublicPath {
		t.Fatal("invalid update lost prior", err)
	}
	before, _ := os.ReadFile(filepath.Join(dir, "catalog.json"))
	delete(snap, "check")
	save()
	if run(args, &out, &errout) == 0 {
		t.Fatal("incomplete snapshot accepted")
	}
	after, _ := os.ReadFile(filepath.Join(dir, "catalog.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("failed job changed catalog")
	}
	snap[key] = []any{}
	if source == "black-box" || source == "levitt" || source == "meow-wolf-denver" {
		snap["total"] = 0
	}
	if paged {
		snap[key] = []any{[]any{}}
	}
	if source == "ball-arena" {
		listing := strings.ReplaceAll(snap["listing"].(string), "1E006434E7FB9F65", "1E006434E7FB9F66")
		listing = strings.ReplaceAll(listing, "Sat • Sep 12 2026 • 7:00 PM", "TBA")
		snap["listing"], snap["listing_check"] = listing, listing
	}
	snap["check"] = snap[key]
	save()
	replay()
	_, arts, err = store.Load(root)
	if err != nil || arts[0].Events[0].Listed || arts[0].Events[0].PublicPath != e.PublicPath {
		t.Fatal("missing event not retained correctly", err)
	}
}
