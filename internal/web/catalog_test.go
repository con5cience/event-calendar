package web

import (
	"crypto/sha256"
	"encoding/json"
	"event-calendar/internal/artifact"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func sourceFixture(t *testing.T, source string) artifact.Artifact {
	t.Helper()
	b, err := os.ReadFile("../../tests/contracts/source.json")
	if err != nil {
		t.Fatal(err)
	}
	a, err := artifact.DecodeArtifact(b)
	if err != nil {
		t.Fatal(err)
	}
	a.Source.ID = source
	a.Events[0].ID = source + "-012345abcdef"
	return a
}
func writeTestFile(t *testing.T, dir, name string, b []byte) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0644); err != nil {
		t.Fatal(err)
	}
}
func publishFixture(t *testing.T, dir, generation string, sources ...artifact.Artifact) artifact.Catalog {
	t.Helper()
	c := artifact.Catalog{SchemaVersion: 1, Generation: generation, GeneratedAt: "2026-09-08T12:00:00Z", Sources: []artifact.ArtifactRef{}}
	for _, a := range sources {
		b, err := artifact.EncodeArtifact(a)
		if err != nil {
			t.Fatal(err)
		}
		path := "sources/" + a.Source.ID + "/" + generation + ".json"
		writeTestFile(t, dir, path, b)
		c.Sources = append(c.Sources, artifact.ArtifactRef{SourceID: a.Source.ID, Artifact: path, SHA256: fmt.Sprintf("%x", sha256.Sum256(b))})
	}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, dir, "catalog.next", b)
	if err := os.Rename(filepath.Join(dir, "catalog.next"), filepath.Join(dir, "catalog.json")); err != nil {
		t.Fatal(err)
	}
	return c
}
func calendarResponse(h http.Handler) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/calendar", nil))
	return w
}
func responseEvents(t *testing.T, h http.Handler) []Event {
	t.Helper()
	w := calendarResponse(h)
	if w.Code != 200 {
		t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
	}
	var data struct {
		Events []Event `json:"events"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if data.Events == nil {
		t.Fatal("events must be an array")
	}
	return data.Events
}
func catalogHandler(dir string, now func() time.Time) http.Handler {
	return NewHandler(Config{DataDir: dir, WeekLimit: 10, MonthLimit: 5, Now: now})
}
func fixedNow() time.Time { return time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC) }

func TestCatalogProjectionAndReadTimeExpiry(t *testing.T) {
	dir := t.TempDir()
	a := sourceFixture(t, "gothic")
	b := sourceFixture(t, "mission")
	b.Events[0].PublicPath = strings.Replace(b.Events[0].PublicPath, "test-show", "other-show", 1)
	b.Events[0].AdmissionPolicy = &artifact.AdmissionPolicy{Text: "21+"}
	b.Events[0].Venue = &artifact.Venue{Key: "warehouse", Name: "Warehouse"}
	b.Events[0].OffSite = true
	b.Events[0].DoorsAt = ""
	b.Events[0].ShowAt = ""
	b.Events[0].PublicPath = strings.Replace(b.Events[0].PublicPath, "/gothic/", "/warehouse/", 1)
	publishFixture(t, dir, "one", a, b)
	now := fixedNow()
	h := catalogHandler(dir, func() time.Time { return now })
	events := responseEvents(t, h)
	if len(events) != 2 || events[0].AgePolicy != "16+" || events[0].Artist != "Test artist" || events[1].Venue != "Warehouse" || events[1].Timezone != "" || events[1].AgePolicy != "21+" || !events[1].OffSite {
		t.Fatalf("projection: %#v", events)
	}
	b.Events[0].AdmissionPolicy = nil
	a.Events[0].Listed = false
	publishFixture(t, dir, "two", a, b)
	events = responseEvents(t, h)
	if len(events) != 1 || events[0].AgePolicy != "" {
		t.Fatalf("unlisted/offsite defaults: %#v", events)
	}
	// Expiry still applies when refresh fails. At 06:59 UTC Denver is December 8.
	writeTestFile(t, dir, "catalog.json", []byte(`{`))
	now = time.Date(2026, 12, 9, 6, 59, 0, 0, time.UTC)
	if len(responseEvents(t, h)) != 1 {
		t.Fatal("expired before Denver midnight")
	}
	now = now.Add(time.Minute)
	if len(responseEvents(t, h)) != 0 {
		t.Fatal("stale snapshot bypassed expiry")
	}
}

func TestCatalogFailureRecoveryAndAtomicReplacement(t *testing.T) {
	dir := t.TempDir()
	h := catalogHandler(dir, fixedNow)
	if len(responseEvents(t, h)) != 0 {
		t.Fatal("empty startup")
	}
	a := sourceFixture(t, "gothic")
	c := publishFixture(t, dir, "one", a)
	if len(responseEvents(t, h)) != 1 {
		t.Fatal("initial load")
	}
	writeTestFile(t, dir, c.Sources[0].Artifact, []byte(`{}`))
	if responseEvents(t, h)[0].Title != a.Events[0].Title {
		t.Fatal("bad checksum replaced snapshot")
	}
	a.Events[0].Title = "Changed"
	b := sourceFixture(t, "mission")
	b.Events[0].PublicPath = strings.Replace(b.Events[0].PublicPath, "test-show", "second-show", 1)
	c = publishFixture(t, dir, "two", a, b)
	if err := os.Remove(filepath.Join(dir, c.Sources[1].Artifact)); err != nil {
		t.Fatal(err)
	}
	e := responseEvents(t, h)
	if len(e) != 1 || e[0].Title == "Changed" {
		t.Fatal("partial generation leaked")
	}
	publishFixture(t, dir, "three", a, b)
	if e = responseEvents(t, h); len(e) != 2 || e[0].Title != "Changed" {
		t.Fatal("recovery failed")
	}
	if err := os.Remove(filepath.Join(dir, "catalog.json")); err != nil {
		t.Fatal(err)
	}
	if len(responseEvents(t, h)) != 2 {
		t.Fatal("missing established catalog erased data")
	}
	publishFixture(t, dir, "empty")
	if len(responseEvents(t, h)) != 0 {
		t.Fatal("valid empty catalog not applied")
	}
}

func TestColdCatalogFailures(t *testing.T) {
	for _, name := range []string{"checksum", "missing", "source", "invalid", "oversize", "path", "symlink", "directory", "duplicate-path", "orphan", "missing-root"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			a := sourceFixture(t, "gothic")
			c := publishFixture(t, dir, "one", a)
			ref := c.Sources[0]
			switch name {
			case "checksum":
				writeTestFile(t, dir, ref.Artifact, []byte(`{}`))
			case "missing":
				if err := os.Remove(filepath.Join(dir, ref.Artifact)); err != nil {
					t.Fatal(err)
				}
			case "source":
				b, _ := json.Marshal(sourceFixture(t, "mission"))
				writeTestFile(t, dir, ref.Artifact, b)
				c.Sources[0].SHA256 = fmt.Sprintf("%x", sha256.Sum256(b))
			case "invalid":
				b := []byte(`{}`)
				writeTestFile(t, dir, ref.Artifact, b)
				c.Sources[0].SHA256 = fmt.Sprintf("%x", sha256.Sum256(b))
			case "oversize":
				writeTestFile(t, dir, ref.Artifact, []byte(strings.Repeat(" ", artifact.MaxDocumentBytes+1)))
			case "path":
				c.Sources[0].Artifact = "../source.json"
			case "symlink":
				outside := t.TempDir()
				writeTestFile(t, outside, "event.json", []byte(`{}`))
				if err := os.Remove(filepath.Join(dir, ref.Artifact)); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(outside, "event.json"), filepath.Join(dir, ref.Artifact)); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Remove(filepath.Join(dir, ref.Artifact)); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(filepath.Join(dir, ref.Artifact), 0755); err != nil {
					t.Fatal(err)
				}
			case "duplicate-path":
				b := sourceFixture(t, "mission")
				c = publishFixture(t, dir, "two", a, b)
			case "orphan":
				if err := os.Remove(filepath.Join(dir, "catalog.json")); err != nil {
					t.Fatal(err)
				}
			case "missing-root":
				dir = filepath.Join(dir, "missing")
			}
			if name != "orphan" && name != "missing-root" {
				b, _ := json.Marshal(c)
				writeTestFile(t, dir, "catalog.json", b)
			}
			if w := calendarResponse(catalogHandler(dir, fixedNow)); w.Code != 503 {
				t.Fatalf("got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestConcurrentCatalogRefresh(t *testing.T) {
	dir := t.TempDir()
	a := sourceFixture(t, "gothic")
	b := sourceFixture(t, "mission")
	b.Events[0].PublicPath = strings.Replace(b.Events[0].PublicPath, "test-show", "second-show", 1)
	publishFixture(t, dir, "initial", a, b)
	h := catalogHandler(dir, fixedNow)
	responseEvents(t, h)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				w := calendarResponse(h)
				var p struct {
					Events []Event `json:"events"`
				}
				err := json.Unmarshal(w.Body.Bytes(), &p)
				if w.Code != 200 || err != nil || len(p.Events) != 2 || p.Events[0].Title != p.Events[1].Title {
					t.Errorf("mixed snapshot: %s", w.Body.String())
					return
				}
			}
		}()
	}
	for i := 0; i < 10; i++ {
		a.Events[0].Title = fmt.Sprint(i)
		b.Events[0].Title = a.Events[0].Title
		publishFixture(t, dir, fmt.Sprintf("gen-%d", i), a, b)
	}
	wg.Wait()
}
