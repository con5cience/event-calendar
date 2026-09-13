package artifact

import (
	"os"
	"testing"
)

func TestPublishedVenueAdmissionRules(t *testing.T) {
	for _, venue := range []string{"gothic", "mission", "ogden", "fiddlers-green"} {
		t.Run(venue, func(t *testing.T) {
			b, err := os.ReadFile("../aeg/testdata/" + venue + ".yaml")
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := DecodeConfigYAML(b)
			if err != nil {
				t.Fatal(err)
			}
			for _, category := range []string{"All ages", "13+", "16+", "18+", "21+", ""} {
				d := EventData{AdmissionPolicy: &AdmissionPolicy{Text: "Original restriction", Category: category}}
				got := EnrichAdmission(cfg, "test", d).AdmissionPolicy
				known := category == "All ages" || (venue != "fiddlers-green" && category != "13+" && category != "")
				if (got.WithAdult != nil) != known {
					t.Fatalf("%s: reviewed metadata = %+v", category, got.WithAdult)
				}
				if got.Text != d.AdmissionPolicy.Text {
					t.Fatal("restriction changed")
				}
				if !known {
					continue
				}
				if got.WithAdult.URL == "" || got.WithAdult.ReviewedOn == "" {
					t.Fatal("missing provenance")
				}
				for _, age := range []int{0, 1, 2, 10, 11, 14, 15, 16, 17} {
					condition := ""
					for _, r := range got.WithAdult.Ranges {
						if age >= r.MinAge && age <= r.MaxAge {
							condition = r.Condition
						}
					}
					allowed := category == "All ages" || category == "16+"
					if (condition != "") != allowed {
						t.Fatalf("%s age %d: %q", category, age, condition)
					}
					if venue != "fiddlers-green" && allowed {
						want := "Permitted at this age"
						if age <= 10 {
							want = "Parent or legal guardian required"
						} else if category == "16+" && age <= 15 {
							want = "Ticketed adult required"
						}
						if condition != want {
							t.Fatalf("age %d: %q, want %q", age, condition, want)
						}
					}
				}
			}
			if EnrichAdmission(cfg, "test", EventData{}).AdmissionPolicy != nil {
				t.Fatal("missing policy inferred")
			}
		})
	}
}
