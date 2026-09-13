package rhp

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Optional operator check: compare every retained raw page with its compacted
// replay input, using the exact production normalizer on both representations.
func TestCapturedHTMLMatchesRaw(t *testing.T) {
	dir, key := os.Getenv("RHP_CAPTURE_DIR"), os.Getenv("RHP_SOURCE")
	if dir == "" || key == "" {
		t.Skip("requires explicit retained live capture")
	}
	config, err := os.ReadFile(filepath.Join("testdata", key+".yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := artifact.DecodeConfigYAML(config)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "raw-pages.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pages map[string]string
	if err = json.Unmarshal(raw, &pages); err != nil {
		t.Fatal(err)
	}
	name := os.Getenv("RHP_SNAPSHOT")
	if name == "" {
		name = "snapshot.json"
	}
	raw, err = os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	var s snapshot
	if err = json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("America/Denver")
	for _, row := range s.Calendar.Data.Events {
		original, originalErr := normalize(row, pages[row.URL], cfg, loc)
		compact, compactErr := normalize(row, s.Details[row.URL], cfg, loc)
		if originalErr != nil {
			if !strings.HasPrefix(originalErr.Error(), "rhp: deferred external venue:") || compactErr == nil || originalErr.Error() != compactErr.Error() {
				t.Fatal(row.URL, originalErr, compactErr)
			}
		} else if compactErr != nil {
			t.Fatal(row.URL, compactErr)
		}
		if !reflect.DeepEqual(original, compact) {
			t.Fatalf("compaction changed %s", row.URL)
		}
	}
}
