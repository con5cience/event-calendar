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

func TestRHPReplayPublicationAndRetention(t *testing.T) {
	for _, key := range []string{"lost-lake", "larimer", "globe-hall", "cervantes"} {
		t.Run(key, func(t *testing.T) {
			dir := t.TempDir()
			var out, errout bytes.Buffer
			args := []string{"replay-rhp", "--store", dir, "--config", "../../internal/rhp/testdata/" + key + ".yaml", "--snapshot", "../../internal/rhp/testdata/" + key + ".json", "--now", "2026-09-10T23:00:00Z"}
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
			e := arts[0].Events[0]
			handler := calendar.NewHandler(calendar.Config{DataDir: dir, Now: func() time.Time { return time.Date(2026, 9, 10, 23, 0, 0, 0, time.UTC) }})
			for _, path := range []string{"/api/calendar", "/api" + e.PublicPath, "/api" + e.PublicPath + ".ics"} {
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
				body := response.Body.String()
				if response.Code != 200 || strings.Contains(body, `"price"`) {
					t.Fatal(path, response.Code, body)
				}
				condition := "Ticketed guardian aged 21+ required"
				if key == "cervantes" {
					condition = "Parent or guardian required"
				}
				if path == "/api/calendar" && !strings.Contains(body, condition) {
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
			os.WriteFile(cfgPath, cfgRaw, 0600)
			args[4] = cfgPath
			raw, _ := os.ReadFile(args[6])
			var snap map[string]any
			json.Unmarshal(raw, &snap)
			snap["details"] = map[string]any{}
			raw, _ = json.Marshal(snap)
			snapPath := filepath.Join(t.TempDir(), "snapshot.json")
			os.WriteFile(snapPath, raw, 0600)
			args[6] = snapPath
			replay()
			_, arts, err = store.Load(root)
			if err != nil || !arts[0].Events[0].Listed || arts[0].Events[0].PublicPath != e.PublicPath || !strings.Contains(out.String(), "rejected") {
				t.Fatalf("invalid detail lost prior event: %v %s", err, &out)
			}
			before, _ := os.ReadFile(filepath.Join(dir, "catalog.json"))
			os.WriteFile(snapPath, []byte(`{"calendar":{"success":false},"details":{}}`), 0600)
			out.Reset()
			if run(args, &out, &errout) == 0 {
				t.Fatal("accepted failed calendar")
			}
			after, _ := os.ReadFile(filepath.Join(dir, "catalog.json"))
			if !bytes.Equal(before, after) {
				t.Fatal("failure changed catalog")
			}
			os.WriteFile(snapPath, []byte(`{"calendar":{"success":true,"data":{"events":[]}},"details":{}}`), 0600)
			replay()
			_, arts, err = store.Load(root)
			if err != nil || arts[0].Events[0].Listed || arts[0].Events[0].PublicPath != e.PublicPath {
				t.Fatal("absence did not unlist while preserving URL", err)
			}
		})
	}
}
