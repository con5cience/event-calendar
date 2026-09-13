package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestLocaleBundleRequiresValidSite(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{"--site", "missing-locale", "missing-input", t.TempDir() + "/output"}, &out)
	if code != 1 || !strings.Contains(out.String(), "missing-locale") {
		t.Fatalf("locale validation must happen before packaging: %d %s", code, out.String())
	}
}
