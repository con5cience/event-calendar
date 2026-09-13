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

func TestReplayPublishesThroughHTTP(t *testing.T) {
	dir := t.TempDir()
	var out, errout bytes.Buffer
	args := []string{"replay-aeg", "--store", dir, "--config", "../../internal/aeg/testdata/gothic.yaml", "--snapshot", "../../internal/aeg/testdata/gothic.json", "--now", "2026-09-09T12:00:00Z"}
	if code := run(args, &out, &errout); code != 0 {
		t.Fatalf("exit %d: %s %s", code, out.String(), errout.String())
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	cat, arts, err := store.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 1 || len(arts[0].Events) != 1 {
		t.Fatal(arts)
	}
	handler := calendar.NewHandler(calendar.Config{DataDir: dir, Now: func() time.Time { return time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC) }})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/api/calendar", nil))
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"status":"Cancelled"`) || !strings.Contains(response.Body.String(), `"doors_at":"2026-09-12T19:00:00-06:00"`) {
		t.Fatal(response.Code, response.Body.String())
	}
	// New cannot overwrite established state, and no failed replay changes catalog.
	before, _ := os.ReadFile(filepath.Join(dir, "catalog.json"))
	out.Reset()
	if run(args, &out, &errout) == 0 {
		t.Fatal("new source overwrote existing")
	}
	after, _ := os.ReadFile(filepath.Join(dir, "catalog.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("failed replay published")
	}
	// Use established configuration; a malformed update retains last valid content.
	cfgBytes, _ := os.ReadFile(args[4])
	cfg, err := artifact.DecodeConfigYAML(cfgBytes)
	if err != nil {
		t.Fatal(err)
	}
	cfg.State = "established"
	cfgBytes, _ = json.Marshal(cfg)
	cfgPath := filepath.Join(t.TempDir(), "established.yaml")
	if err := os.WriteFile(cfgPath, cfgBytes, 0600); err != nil {
		t.Fatal(err)
	}
	args[4] = cfgPath
	snapshot, _ := os.ReadFile(args[6])
	snapshot = bytes.ReplaceAll(snapshot, []byte(`"eventTitleText": "Fixture Ensemble"`), []byte(`"eventTitleText": ""`))
	snapshot = bytes.ReplaceAll(snapshot, []byte(`"headlinersText": "Fixture Ensemble"`), []byte(`"headlinersText": ""`))
	snapshotPath := filepath.Join(t.TempDir(), "invalid.json")
	os.WriteFile(snapshotPath, snapshot, 0600)
	args[6] = snapshotPath
	out.Reset()
	if code := run(args, &out, &errout); code != 0 {
		t.Fatal(code, out.String(), errout.String())
	}
	var report struct{ Rejected []artifact.Rejection }
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Rejected) != 1 {
		t.Fatal(out.String())
	}
	next, nextArts, err := store.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if next.Generation == cat.Generation || nextArts[0].Events[0].PublicPath != arts[0].Events[0].PublicPath {
		t.Fatal("identity or publication incorrect")
	}
	if err := os.WriteFile(snapshotPath, []byte(`{"meta":{"total":0,"page":1,"rows":100},"events":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := run(args, &out, &errout); code != 0 {
		t.Fatal(code, out.String(), errout.String())
	}
	_, emptyArts, err := store.Load(root)
	if err != nil || emptyArts[0].Events[0].Listed {
		t.Fatal("successful empty snapshot did not remove listing", err)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/api/calendar", nil))
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"events":[]`) {
		t.Fatal(response.Code, response.Body.String())
	}
}

func TestReplayCommittedButOutputLost(t *testing.T) {
	dir := t.TempDir()
	var errout bytes.Buffer
	code := run([]string{"replay-aeg", "--store", dir, "--config", "../../internal/aeg/testdata/gothic.yaml", "--snapshot", "../../internal/aeg/testdata/gothic.json", "--now", "2026-09-09T12:00:00Z"}, brokenOutput{}, &errout)
	if code != 3 {
		t.Fatalf("published replay with lost output returned %d", code)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, _, err := store.Load(root); err != nil {
		t.Fatal(err)
	}
}
func TestReplayFailureNeverPublishes(t *testing.T) {
	for _, snapshot := range []string{"missing.json", "../../internal/aeg/testdata/gothic.yaml"} {
		dir := t.TempDir()
		var out, errout bytes.Buffer
		if run([]string{"replay-aeg", "--store", dir, "--config", "../../internal/aeg/testdata/gothic.yaml", "--snapshot", snapshot, "--now", "2026-09-09T12:00:00Z"}, &out, &errout) == 0 {
			t.Fatal("accepted failed input")
		}
		if _, err := os.Stat(filepath.Join(dir, "catalog.json")); !os.IsNotExist(err) {
			t.Fatal("failure created catalog", err)
		}
	}
}
