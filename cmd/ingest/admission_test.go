package main

import (
	"bytes"
	"encoding/json"
	"event-calendar/internal/artifact"
	"event-calendar/internal/store"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestEnrichAdmissionPreservesRecords(t *testing.T) {
	dir := t.TempDir()
	var out, errout bytes.Buffer
	if run([]string{"publish", "--store", dir, "--expect", "none", "../../tests/contracts/source.json"}, &out, &errout) != 0 {
		t.Fatal(out.String(), errout.String())
	}
	b, _ := os.ReadFile("../../tests/contracts/config.yaml")
	cfg, err := artifact.DecodeConfigYAML(b)
	if err != nil {
		t.Fatal(err)
	}
	cfg.State = "established"
	cfg.Overrides = nil
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	_, before, err := store.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Source = before[0].Source
	cfg.Venue = before[0].Venue
	encoded, _ := json.Marshal(cfg)
	var raw map[string]any
	json.Unmarshal(encoded, &raw)
	raw["admission_rules"] = map[string]any{"16+": map[string]any{"url": "https://example.com/policy", "reviewed_on": "2026-09-09", "ranges": []any{map[string]any{"min_age": 11, "max_age": 15, "condition": "Ticketed adult required"}}}}
	encoded, _ = json.Marshal(raw)
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(configFile, encoded, 0600)
	out.Reset()
	if code := run([]string{"enrich-admission", "--store", dir, "--config", configFile}, &out, &errout); code != 0 {
		t.Fatalf("%d %s %s", code, out.String(), errout.String())
	}
	_, after, err := store.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(after[0].Events) != len(before[0].Events) {
		t.Fatal("record count changed")
	}
	for i, event := range after[0].Events {
		if event.AdmissionPolicy == nil || event.AdmissionPolicy.WithAdult == nil {
			t.Fatal("missing admission information")
		}
		event.AdmissionPolicy = before[0].Events[i].AdmissionPolicy
		if !reflect.DeepEqual(event, before[0].Events[i]) {
			t.Fatal("non-admission data changed")
		}
	}
}
