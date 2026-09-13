package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type brokenOutput struct{}

func (brokenOutput) Write([]byte) (int, error) { return 0, errors.New("output disconnected") }
func TestCommittedPublicationWithBrokenOutput(t *testing.T) {
	var errout bytes.Buffer
	dir := t.TempDir()
	code := run([]string{"publish", "--store", dir, "--expect", "none", "../../tests/contracts/source.json"}, brokenOutput{}, &errout)
	if code != 3 {
		t.Fatalf("published result lost: exit %d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "catalog.json")); err != nil {
		t.Fatal(err)
	}
}

func TestPublishCLI(t *testing.T) {
	dir := t.TempDir()
	var out, errout bytes.Buffer
	args := []string{"publish", "--store", dir, "--expect", "none", "../../tests/contracts/source.json"}
	if code := run(args, &out, &errout); code != 0 {
		t.Fatalf("exit %d: %s", code, errout.String())
	}
	var report struct {
		Generation         string
		Published, Durable bool
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if !report.Published || !report.Durable || report.Generation == "" {
		t.Fatal(out.String())
	}
	before, err := os.ReadFile(filepath.Join(dir, "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errout.Reset()
	if code := run(args, &out, &errout); code == 0 {
		t.Fatal("stale expectation succeeded")
	}
	after, _ := os.ReadFile(filepath.Join(dir, "catalog.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("failed CLI changed store")
	}
}
func TestCLIUsageAndInputErrors(t *testing.T) {
	for _, args := range [][]string{nil, {"run-all"}, {"publish"}, {"publish", "--store", t.TempDir(), "--expect", "none", "missing.json"}} {
		var out, errout bytes.Buffer
		if code := run(args, &out, &errout); code == 0 {
			t.Fatalf("accepted %v", args)
		}
	}
}
