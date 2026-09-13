package web

import (
	"strings"
	"testing"
)

func TestPriceRetiredFromLegacyArtifactResponses(t *testing.T) {
	dir := t.TempDir()
	a := sourceFixture(t, "gothic")
	if a.Events[0].Price == nil {
		t.Fatal("fixture must contain price")
	}
	price := a.Events[0].Price.Text
	publishFixture(t, dir, "priced", a)
	h := catalogHandler(dir, fixedNow)
	path := a.Events[0].PublicPath
	for _, route := range []string{"/api/calendar", "/api" + path, "/api" + path + ".ics"} {
		w := getResponse(h, route)
		if w.Code != 200 {
			t.Fatalf("%s: %d", route, w.Code)
		}
		for _, forbidden := range []string{`"price"`, `"cost_category"`, "Price:", price} {
			if strings.Contains(w.Body.String(), forbidden) {
				t.Fatalf("%s leaked %q", route, forbidden)
			}
		}
	}
	a.Events[0].Listed = false
	publishFixture(t, dir, "retained", a)
	if body := getResponse(h, "/api"+path).Body.String(); strings.Contains(body, `"price"`) {
		t.Fatal(body)
	}
}

func TestPriceRetiredConfigurationIgnored(t *testing.T) {
	t.Setenv("SITE_DIR", "../../tests/contracts")
	t.Setenv("COST_BRACKET_LIMITS", "obsolete value")
	if _, err := ConfigFromEnv(); err != nil {
		t.Fatal(err)
	}
}
