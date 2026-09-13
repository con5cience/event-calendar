package web

import (
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
)

func eventCalendar(e Event, now time.Time) (string, error) {
	cal := ics.NewCalendar()
	entry := cal.AddEvent(e.ID + "@event-calendar")
	entry.SetDtStampTime(now.UTC())
	entry.SetSummary(e.Title)
	entry.SetLocation(e.Venue)
	entry.SetStatus(ics.ObjectStatus("CONFIRMED"))
	if status := strings.ToLower(strings.TrimSpace(e.Status)); status == "cancelled" || status == "canceled" {
		entry.SetStatus(ics.ObjectStatus("CANCELLED"))
	}
	description := []string{}
	start := e.DoorsAt
	if start == "" {
		start = e.ShowAt
	}
	if start == "" {
		date, err := time.Parse("2006-01-02", e.Date)
		if err != nil {
			return "", err
		}
		entry.SetAllDayStartAt(date)
		description = append(description, "Time not provided")
	} else {
		date, err := time.Parse(time.RFC3339, start)
		if err != nil {
			return "", err
		}
		entry.SetStartAt(date.UTC())
	}
	for _, field := range [][2]string{{"Artist", e.Artist}, {"Doors", e.DoorsAt}, {"Show", e.ShowAt}, {"Age policy", e.AgePolicy}, {"Age policy link", e.AgePolicyURL}, {"Link", e.EventURL}, {"Tickets", e.TicketURL}} {
		if field[1] != "" {
			description = append(description, field[0]+": "+field[1])
		}
	}
	if e.OffSite {
		description = append(description, "Off-site")
	}
	if !e.Listed {
		description = append(description, "No longer listed")
	}
	entry.SetDescription(strings.Join(description, "\n"))
	if e.EventURL != "" {
		entry.SetURL(e.EventURL)
	}
	return cal.Serialize(ics.WithNewLineWindows), nil
}
