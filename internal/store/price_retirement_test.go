package store

import (
	"bytes"
	"os"
	"testing"
)

func TestPriceRetiredFromNewPublication(t *testing.T) {
	dir := t.TempDir()
	input := candidate(t, "gothic")
	before := append([]byte(nil), input...)
	if !bytes.Contains(input, []byte(`"price"`)) {
		t.Fatal("legacy candidate must contain price")
	}
	publish(t, dir, "none", input)
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	catalog, artifacts, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if artifacts[0].Events[0].Price != nil {
		t.Fatal("price published")
	}
	written, err := ReadDocument(root, catalog.Sources[0].Artifact)
	if err != nil || bytes.Contains(written, []byte(`"price"`)) {
		t.Fatalf("%v %s", err, written)
	}
	if !bytes.Equal(input, before) {
		t.Fatal("input bytes mutated")
	}
}
