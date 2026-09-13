package main

import (
	"bytes"
	calendar "event-calendar/internal/web"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHMTReplayHTTP(t *testing.T) {
	for _, venue := range []string{"hq", "oriental"} {
		t.Run(venue, func(t *testing.T) {
			dir := t.TempDir()
			var out, errout bytes.Buffer
			args := []string{"replay-hmt", "--store", dir, "--config", "../../internal/hmt/testdata/" + venue + ".yaml", "--snapshot", "../../internal/hmt/testdata/" + venue + ".json", "--now", "2026-09-10T12:00:00Z"}
			if code := run(args, &out, &errout); code != 0 {
				t.Fatalf("%d %s %s", code, &out, &errout)
			}
			h := calendar.NewHandler(calendar.Config{DataDir: dir, Now: func() time.Time { return time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC) }})
			res := httptest.NewRecorder()
			h.ServeHTTP(res, httptest.NewRequest("GET", "/api/calendar", nil))
			if res.Code != 200 || !strings.Contains(res.Body.String(), "Fixture Ensemble") || !strings.Contains(res.Body.String(), "19:00:00-06:00") {
				t.Fatal(res.Code, res.Body.String())
			}
			if run(args, &out, &errout) == 0 {
				t.Fatal("new source replaced established source")
			}
		})
	}
}
