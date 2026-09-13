package main

import (
	"bytes"
	"event-calendar/internal/store"
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	b, err := os.ReadFile("../../tests/contracts/source.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (&store.Publisher{}).Publish(dir, "none", [][]byte{b}); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestPackageSnapshot(t *testing.T) {
	in := fixture(t)
	if err := os.WriteFile(filepath.Join(in, "unused.json"), []byte("private capture"), 0600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "snapshot")
	var stderr bytes.Buffer
	if code := run([]string{in, out}, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, &stderr)
	}
	root, err := os.OpenRoot(out)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	c, _, err := store.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"catalog.json"}
	for _, ref := range c.Sources {
		names = append(names, ref.Artifact)
	}
	for _, name := range names {
		original, err := os.ReadFile(filepath.Join(in, name))
		if err != nil {
			t.Fatal(err)
		}
		copied, err := os.ReadFile(filepath.Join(out, name))
		if err != nil || !bytes.Equal(original, copied) {
			t.Fatalf("changed bytes: %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "unused.json")); !os.IsNotExist(err) {
		t.Fatal("copied unreferenced file")
	}
}

func TestRejectInvalidSnapshot(t *testing.T) {
	for _, kind := range []string{"missing-catalog", "corrupt-catalog", "missing-source", "checksum"} {
		t.Run(kind, func(t *testing.T) {
			in := fixture(t)
			name := filepath.Join(in, "catalog.json")
			if kind == "missing-source" || kind == "checksum" {
				root, err := os.OpenRoot(in)
				if err != nil {
					t.Fatal(err)
				}
				c, _, err := store.Load(root)
				root.Close()
				if err != nil {
					t.Fatal(err)
				}
				name = filepath.Join(in, c.Sources[0].Artifact)
			}
			var err error
			if kind == "missing-catalog" || kind == "missing-source" {
				err = os.Remove(name)
			} else {
				err = os.WriteFile(name, []byte("{}"), 0644)
			}
			if err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(t.TempDir(), "snapshot")
			var stderr bytes.Buffer
			if run([]string{in, out}, &stderr) != 1 {
				t.Fatalf("accepted invalid snapshot: %s", &stderr)
			}
			if _, err := os.Stat(out); !os.IsNotExist(err) {
				t.Fatal("created output before validating")
			}
		})
	}
}

func TestRejectExistingOutputAndBadArguments(t *testing.T) {
	var stderr bytes.Buffer
	if run(nil, &stderr) != 2 {
		t.Fatal("expected usage error")
	}
	out := t.TempDir()
	if run([]string{fixture(t), out}, &stderr) != 1 {
		t.Fatal("accepted existing output")
	}
}
