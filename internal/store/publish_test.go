package store

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"event-calendar/internal/artifact"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func candidate(t *testing.T, source string) []byte {
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
	a.Events[0].PublicPath = strings.Replace(a.Events[0].PublicPath, "test-show", source+"-show", 1)
	b, err = artifact.EncodeArtifact(a)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func catalogBytes(t *testing.T, dir string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func publish(t *testing.T, dir, expected string, input ...[]byte) Report {
	t.Helper()
	r, err := (&Publisher{}).Publish(dir, expected, input)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Published || !r.Durable || r.Generation == "" {
		t.Fatalf("bad report: %#v", r)
	}
	return r
}
func TestPublishAndPreserveUntouchedSources(t *testing.T) {
	dir := t.TempDir()
	first := publish(t, dir, "none", candidate(t, "gothic"), candidate(t, "mission"))
	var prior artifact.Catalog
	json.Unmarshal(catalogBytes(t, dir), &prior)
	input := bytes.Replace(candidate(t, "gothic"), []byte("Test show"), []byte("Updated show"), 1)
	second := publish(t, dir, first.Generation, input)
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	c, events, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if c.Generation != second.Generation || len(events) != 2 || events[0].Events[0].Title != "Updated show" || c.Sources[1] != prior.Sources[1] {
		t.Fatalf("bad generation: %#v", c)
	}
	if _, err := ReadDocument(root, prior.Sources[0].Artifact); err != nil {
		t.Fatal("old immutable file removed", err)
	}
}
func TestRejectWithoutChangingCatalog(t *testing.T) {
	for _, kind := range []string{"stale", "missing-expected", "invalid", "duplicate", "path-collision", "corrupt-prior", "missing-prior", "empty-input", "changed-source"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			first := publish(t, dir, "none", candidate(t, "gothic"))
			expected := first.Generation
			input := [][]byte{candidate(t, "mission")}
			switch kind {
			case "stale":
				expected = "old"
			case "missing-expected":
				expected = ""
			case "invalid":
				input = append(input, []byte(`{}`))
			case "duplicate":
				input = append(input, input[0])
			case "path-collision":
				input[0] = bytes.ReplaceAll(input[0], []byte("mission-show"), []byte("gothic-show"))
			case "corrupt-prior":
				if err := os.WriteFile(filepath.Join(dir, "catalog.json"), []byte(`{}`), 0644); err != nil {
					t.Fatal(err)
				}
			case "missing-prior":
				if err := os.Remove(filepath.Join(dir, "catalog.json")); err != nil {
					t.Fatal(err)
				}
			case "empty-input":
				input = nil
			case "changed-source":
				input = [][]byte{bytes.Replace(candidate(t, "gothic"), []byte("aeg-json"), []byte("another-adapter"), 1)}
			}
			before, _ := os.ReadFile(filepath.Join(dir, "catalog.json"))
			r, err := (&Publisher{}).Publish(dir, expected, input)
			if err == nil || r.Published {
				t.Fatalf("accepted %s: %#v %v", kind, r, err)
			}
			after, _ := os.ReadFile(filepath.Join(dir, "catalog.json"))
			if !bytes.Equal(before, after) {
				t.Fatal("failed publication changed catalog")
			}
		})
	}
}
func TestPublicationFaultBoundaries(t *testing.T) {
	for _, stage := range []string{"source-synced", "catalog-synced", "before-rename", "after-rename"} {
		t.Run(stage, func(t *testing.T) {
			dir := t.TempDir()
			first := publish(t, dir, "none", candidate(t, "gothic"))
			before := catalogBytes(t, dir)
			p := Publisher{checkpoint: func(s string) error {
				if s == stage {
					return errors.New("injected failure")
				}
				return nil
			}}
			r, err := p.Publish(dir, first.Generation, [][]byte{candidate(t, "mission")})
			if err == nil {
				t.Fatal("fault not reached")
			}
			after := catalogBytes(t, dir)
			if stage == "after-rename" {
				if !r.Published || r.Durable || bytes.Equal(before, after) {
					t.Fatal("post-commit report is misleading")
				}
				first.Generation = r.Generation
			} else if r.Published || !bytes.Equal(before, after) {
				t.Fatal("pre-commit failure changed catalog")
			}
			publish(t, dir, first.Generation, candidate(t, "mission"))
		})
	}
}
func TestImmutableCollisionAndRootConfinement(t *testing.T) {
	dir := t.TempDir()
	p := Publisher{generation: func() (string, error) { return "fixed", nil }}
	first, err := p.Publish(dir, "none", [][]byte{candidate(t, "gothic")})
	if err != nil {
		t.Fatal(err)
	}
	before := catalogBytes(t, dir)
	if r, err := p.Publish(dir, first.Generation, [][]byte{candidate(t, "gothic")}); err == nil || r.Published {
		t.Fatal("overwrote immutable file")
	}
	if !bytes.Equal(before, catalogBytes(t, dir)) {
		t.Fatal("collision changed catalog")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "sources", "mission")); err != nil {
		t.Fatal(err)
	}
	if _, err := (&Publisher{}).Publish(dir, first.Generation, [][]byte{candidate(t, "mission")}); err == nil {
		t.Fatal("escaped store")
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatal("wrote outside root")
	}
}

// A separate test process holds the real OS lock, then is killed at a write boundary.
func TestPublisherChild(t *testing.T) {
	if os.Getenv("CALENDAR_PUBLISH_CHILD") == "" {
		return
	}
	p := Publisher{checkpoint: func(stage string) error {
		if stage == "source-synced" {
			fmt.Println("locked")
			select {}
		}
		return nil
	}}
	_, err := p.Publish(os.Getenv("CALENDAR_PUBLISH_STORE"), os.Getenv("CALENDAR_PUBLISH_EXPECT"), [][]byte{candidate(t, "mission")})
	if err != nil {
		t.Fatal(err)
	}
}
func TestCompetingProcessAndCrashRecovery(t *testing.T) {
	dir := t.TempDir()
	first := publish(t, dir, "none", candidate(t, "gothic"))
	before := catalogBytes(t, dir)
	cmd := exec.Command(os.Args[0], "-test.run=^TestPublisherChild$")
	cmd.Env = append(os.Environ(), "CALENDAR_PUBLISH_CHILD=1", "CALENDAR_PUBLISH_STORE="+dir, "CALENDAR_PUBLISH_EXPECT="+first.Generation)
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	ready := make(chan bool, 1)
	go func() { s := bufio.NewScanner(pipe); ready <- s.Scan() && s.Text() == "locked" }()
	select {
	case ok := <-ready:
		if !ok {
			t.Fatal("child did not acquire lock")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("child timeout")
	}
	r, err := (&Publisher{}).Publish(dir, first.Generation, [][]byte{candidate(t, "mission")})
	if err == nil || r.Published || !strings.Contains(err.Error(), "writer") {
		t.Fatalf("competing writer not rejected: %v", err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	if !bytes.Equal(before, catalogBytes(t, dir)) {
		t.Fatal("crash changed catalog")
	}
	publish(t, dir, first.Generation, candidate(t, "mission"))
}
