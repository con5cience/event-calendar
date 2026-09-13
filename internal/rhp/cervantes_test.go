package rhp

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"os"
	"strings"
	"testing"
)

func cervantesFixture(t *testing.T, room, name string) (artifact.SourceConfig, map[string]any) {
	t.Helper()
	raw, err := os.ReadFile("testdata/cervantes.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := artifact.DecodeConfigYAML(raw)
	if err != nil {
		t.Fatal(err)
	}
	_, s := fixture(t, "lost-lake")
	row := s["calendar"].(map[string]any)["data"].(map[string]any)["events"].([]any)[0].(map[string]any)
	old := row["url"].(string)
	u := "https://cervantesmasterpiece.com/event/fixture/" + room + "/denver-colorado/"
	page := s["details"].(map[string]any)[old].(string)
	page = strings.ReplaceAll(page, old, u)
	page = strings.ReplaceAll(page, "Lost Lake", name)
	page = strings.ReplaceAll(page, "Doors: 7 pm Show: 8 pm", "Doors: 7 pm | Show: 8 pm")
	row["url"] = u
	s["details"] = map[string]any{u: page}
	return cfg, s
}

func TestCervantesRoomsAndAdmission(t *testing.T) {
	for _, room := range []struct{ slug, name string }{
		{"cervantes-other-side", "Cervantes&#8217; Other Side"},
		{"cervantes-masterpiece-ballroom", "Cervantes&#8217; Masterpiece Ballroom"},
		{"cervantes-and-other-side-dual-venue", "Cervantes&#8217; and Other Side &#8211; Dual Venue"},
	} {
		t.Run(room.slug, func(t *testing.T) {
			c, s := cervantesFixture(t, room.slug, room.name)
			r := decodeFixture(t, c, s)
			out, err := (&artifact.Reconciler{}).Reconcile(c, nil, r, r.Coverage.From)
			if err != nil || len(out.Rejected) != 0 || len(out.Artifact.Events) != 1 {
				t.Fatalf("%v %+v", err, out)
			}
			e := out.Artifact.Events[0]
			if e.OffSite || e.Venue != nil || e.DoorsAt != "2026-09-12T19:00:00-06:00" || e.Price != nil {
				t.Fatalf("%+v", e)
			}
			p := e.AdmissionPolicy
			if p == nil || p.WithAdult == nil || p.WithAdult.URL != "https://cervantesmasterpiece.com/faq/" || p.WithAdult.Ranges[0].MinAge != 14 || p.WithAdult.Ranges[0].MaxAge != 15 || p.WithAdult.Ranges[1].MinAge != 16 {
				t.Fatalf("%+v", p)
			}
		})
	}
}

func TestCervantesPolicyAndExternalBoundaries(t *testing.T) {
	for _, variant := range []string{"18+", "21+", "unknown", "all ages", "external", "mismatched venue", "override"} {
		t.Run(variant, func(t *testing.T) {
			c, s := cervantesFixture(t, "cervantes-other-side", "Cervantes&#8217; Other Side")
			ds := s["details"].(map[string]any)
			var u, page string
			for key, v := range ds {
				u = key
				page = v.(string)
			}
			switch variant {
			case "18+":
				page = strings.ReplaceAll(page, "Ages 16 and up", "Ages 18 and up")
			case "21+":
				page = strings.ReplaceAll(page, "Ages 16 and up", "Ages 21 and up")
			case "unknown":
				page = strings.ReplaceAll(page, "Ages 16 and up", "16+ No exceptions")
			case "all ages":
				page = strings.ReplaceAll(page, "Ages 16 and up", "All Ages")
			case "external":
				row := s["calendar"].(map[string]any)["data"].(map[string]any)["events"].([]any)[0].(map[string]any)
				v := strings.ReplaceAll(u, "cervantes-other-side", "mission-ballroom")
				row["url"] = v
				delete(ds, u)
				page = strings.ReplaceAll(page, u, v)
				u = v
				page = strings.ReplaceAll(page, "Cervantes&#8217; Other Side", "Mission Ballroom")
			case "mismatched venue":
				page = strings.ReplaceAll(page, "Cervantes&#8217; Other Side", "Mission Ballroom")
			case "override":
				c.Overrides = []artifact.Override{{Match: artifact.Match{UpstreamID: u, VenueKey: "cervantes", Date: "2026-09-12"}, Set: map[string]json.RawMessage{"admission_policy": json.RawMessage(`{"text":"18+ only","category":"18+"}`)}}}
			}
			ds[u] = page
			r := decodeFixture(t, c, s)
			out, err := (&artifact.Reconciler{}).Reconcile(c, nil, r, r.Coverage.From)
			if err != nil {
				t.Fatal(err)
			}
			if variant == "external" || variant == "mismatched venue" {
				if len(out.Artifact.Events) != 0 || len(out.Rejected) != 1 {
					t.Fatalf("external event published: %+v", out)
				}
				return
			}
			if len(out.Rejected) != 0 || len(out.Artifact.Events) != 1 {
				t.Fatalf("%+v", out)
			}
			p := out.Artifact.Events[0].AdmissionPolicy
			if variant == "all ages" {
				if p.WithAdult == nil || p.WithAdult.Ranges[0].MinAge != 0 || p.WithAdult.Ranges[0].Condition != "Parent or guardian encouraged" {
					t.Fatalf("%+v", p)
				}
			} else if p.WithAdult != nil {
				t.Fatalf("unreviewed exception: %+v", p)
			}
		})
	}
}

func TestCervantesSeparatePerformances(t *testing.T) {
	c, s := cervantesFixture(t, "cervantes-masterpiece-ballroom", "Cervantes&#8217; Masterpiece Ballroom")
	data := s["calendar"].(map[string]any)["data"].(map[string]any)
	rows := data["events"].([]any)
	row := rows[0].(map[string]any)
	later := map[string]any{}
	for k, v := range row {
		later[k] = v
	}
	old := row["url"].(string)
	u := strings.ReplaceAll(old, "/fixture/", "/fixture-late/")
	later["url"] = u
	later["start"] = "2026-09-12T22:00:00"
	later["strctaHtml"] = strings.ReplaceAll(later["strctaHtml"].(string), "/123/", "/124/")
	ds := s["details"].(map[string]any)
	h := ds[old].(string)
	h = strings.ReplaceAll(h, old, u)
	h = strings.ReplaceAll(h, "20:00:00", "22:00:00")
	h = strings.ReplaceAll(h, "7 pm | Show: 8 pm", "9 pm | Show: 10 pm")
	h = strings.ReplaceAll(h, "/123/", "/124/")
	ds[u] = h
	data["events"] = append(rows, later)
	r := decodeFixture(t, c, s)
	out, err := (&artifact.Reconciler{}).Reconcile(c, nil, r, r.Coverage.From)
	if err != nil || len(out.Rejected) != 0 || len(out.Artifact.Events) != 2 {
		t.Fatalf("separate performances lost: %v %+v", err, out)
	}
	a, b := out.Artifact.Events[0], out.Artifact.Events[1]
	if a.PublicPath == b.PublicPath || a.ID == b.ID || a.ShowAt == b.ShowAt {
		t.Fatalf("performances merged: %+v", out)
	}
}
