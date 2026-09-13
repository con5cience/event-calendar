package afton

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestLegacyAdmissionAndFreeEvents(t *testing.T) {
	b, err := os.ReadFile("testdata/detail.html")
	if err != nil {
		t.Fatal(err)
	}
	h := string(b)
	loc, _ := time.LoadLocation("America/Denver")
	e := record{Type: "real_world", Name: "Fixture & Friends", Start: "2026-09-26 19:00:00", Doors: "2026-09-26 18:00:00", Venue: "The Roxy Theatre", City: "Denver", State: "CO", Link: "https://aftontickets.com/event/buyticket/3px8g401j1", Hide: "no", DisplayDoors: "yes"}
	legacy := `<li class="age-restriction">Age Restriction:&nbsp; <b>All Ages &amp; Bar w/ID</b></li><div class="age-restriction"><b>Age Restriction:</b> All Ages &amp; Bar w/ID</div>`
	modern := `<div><span class="modal-event-info-label">Age Restriction:</span><span class="modal-event-info-value">All Ages &amp; Bar w/ID</span></div>`
	for _, kind := range []string{"legacy", "conflict", "free", "restricted"} {
		t.Run(kind, func(t *testing.T) {
			s := h
			switch kind {
			case "legacy":
				s = strings.Replace(s, modern, legacy, 1)
			case "conflict":
				s = strings.Replace(s, modern, modern+strings.ReplaceAll(legacy, "All Ages &amp; Bar w/ID", "21+ Only"), 1)
			case "free":
				s = strings.Replace(s, `"offers":[{"url":"https://aftontickets.com/event/buyticket/3px8g401j1","price":"20.00"}]`, `"organizer":{"url":"https://aftontickets.com/event/buyticket/3px8g401j1"}`, 1)
			case "restricted":
				s = strings.ReplaceAll(s, "All Ages &amp; Bar w/ID", "21+ Only")
			}
			v, err := normalize(e, s, loc)
			if kind == "conflict" {
				if err == nil {
					t.Fatal("conflicting admission accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if kind == "restricted" {
				if v.AdmissionPolicy.Category != "21+" || v.AdmissionPolicy.WithAdult != nil {
					t.Fatal(v)
				}
			} else if v.AdmissionPolicy.WithAdult == nil {
				t.Fatal(v)
			}
		})
	}
}
