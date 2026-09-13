package web

import (
	"event-calendar/internal/artifact"
	"strings"
	"testing"
)

func TestAgeMetadataThroughPublishedArtifactsAndHTTP(t *testing.T) {
	dir := t.TempDir()
	a := sourceFixture(t, "gothic")
	publishFixture(t, dir, "inherited", a)
	h := catalogHandler(dir, fixedNow)
	if got := responseEvents(t, h)[0]; got.AgeCategory != "16+" {
		t.Fatalf("inherited metadata: %#v", got)
	}
	a.Events[0].AdmissionPolicy = &artifact.AdmissionPolicy{Text: "Adults only", Category: "21+"}
	publishFixture(t, dir, "override", a)
	if got := responseEvents(t, h)[0]; got.AgeCategory != "21+" || got.AgePolicy != "Adults only" {
		t.Fatalf("override: %#v", got)
	}
	a.Events[0].AdmissionPolicy = &artifact.AdmissionPolicy{Text: "Contact venue"}
	publishFixture(t, dir, "unknown", a)
	w := calendarResponse(h)
	if strings.Contains(w.Body.String(), "age_category") {
		t.Fatalf("unknown event override must not inherit structured age: %s", w.Body.String())
	}
}
