package anteup

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"strings"
	"testing"
	"time"
)

const policyHTML = `<main>Ante Up is a nonprofit sober community space for people of all ages. 2130 S. Platte River Drive, Denver.</main>`

func config(t *testing.T) artifact.SourceConfig {
	t.Helper()
	b := []byte(`{"schema_version":1,"source":{"id":"ante-up","adapter":"html-anteup"},"state":"new","venue":{"key":"ante-up","name":"Ante Up!","timezone":"America/Denver","website":"https://www.anteupdenver.com/events-1","address":"2130 S Platte River Dr, Denver, CO 80223","admission_policy":{"text":"All ages; sober community space","category":"All ages","url":"https://www.anteupdenver.com/venue"}},"admission_rules":{"All ages":{"url":"https://www.anteupdenver.com/venue","reviewed_on":"2026-09-17","ranges":[{"min_age":0,"max_age":17,"condition":"Permitted at this age; sober space"}]}}}`)
	cfg, err := artifact.DecodeConfigJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func eventJSON(id, title, slug, desc, name, status string) string {
	if status == "" {
		status = "0"
	}
	return `{
		"id":"` + id + `","title":"` + title + `","slug":"` + slug + `","description":"` + desc + `","status":` + status + `,
		"location":{"name":"` + name + `","address":"2130 S Platte River Dr, Denver, CO 80223, USA"},
		"scheduling":{"config":{"scheduleTbd":false,"startDate":"2026-09-18T01:00:57.686Z","timeZoneId":"America/Denver"},"startDateFormatted":"September 17, 2026","startTimeFormatted":"7:00 PM"},
		"registration":{"ticketing":{"config":{}}}
	}`
}

func warmup(events, hasMore string) string {
	if hasMore == "" {
		hasMore = "false"
	}
	return `{"appsWarmupData":{"140603ad-af8d-84a5-2c80-a0f60cb47351":{"widget":{"events":{"events":[` + events + `],"hasMore":` + hasMore + `,"moreLoading":false},"dates":{"events":{"eyes":{"startDateISOFormatNotUTC":"2026-09-17T19:00:57-06:00","startTime":"7:00 PM","utcOffset":-360}}}}}}}`
}

func snapshot(events, hasMore, policy, checkEvents string) []byte {
	if checkEvents == "" {
		checkEvents = events
	}
	if policy == "" {
		policy = policyHTML
	}
	raw, _ := json.Marshal(map[string]any{
		"from": "2026-09-17", "through": "2027-09-17",
		"pages": pages{Warmup: warmup(events, hasMore), Policy: policy},
		"check": pages{Warmup: warmup(checkEvents, hasMore), Policy: policy},
	})
	return raw
}

func TestMapping(t *testing.T) {
	now := time.Date(2026, 9, 17, 18, 0, 0, 0, time.UTC)
	cfg := config(t)
	eyes := eventJSON("eyes", "EYES OF SALT", "eyes-of-salt", `ALL AGES \nDOORS at 7:00 PM\nSHOW at 7:30 PM`, "ANTE UP", "")
	run, err := Decode(cfg, snapshot(eyes, "", "", ""), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Observations) != 1 || run.Observations[0].Failure != "" {
		t.Fatalf("%+v", run.Observations)
	}
	var d artifact.EventData
	if json.Unmarshal(run.Observations[0].Data, &d) != nil {
		t.Fatal("data")
	}
	if d.Title != "EYES OF SALT" || d.Date != "2026-09-17" || d.DoorsAt != "2026-09-17T19:00:00-06:00" || d.ShowAt != "2026-09-17T19:30:00-06:00" || d.EventURL != "https://www.anteupdenver.com/events/eyes-of-salt" || d.TicketURL != d.EventURL || d.AdmissionPolicy != nil {
		t.Fatalf("%+v", d)
	}
	out, err := (&artifact.Reconciler{}).Reconcile(cfg, nil, run, "2026-09-17")
	if err != nil || len(out.Artifact.Events) != 1 || out.Artifact.Events[0].AdmissionPolicy == nil || out.Artifact.Events[0].AdmissionPolicy.WithAdult == nil {
		t.Fatalf("%v %+v", err, out)
	}
	if out.Artifact.Events[0].AdmissionPolicy.WithAdult.Ranges[0].Condition != "Permitted at this age; sober space" {
		t.Fatal(out.Artifact.Events[0].AdmissionPolicy.WithAdult)
	}
}

func TestRecordFailures(t *testing.T) {
	now := time.Date(2026, 9, 17, 18, 0, 0, 0, time.UTC)
	cfg := config(t)
	for _, tc := range []struct {
		name, events, want string
	}{
		{"offsite", eventJSON("eyes", "Elsewhere", "elsewhere", "ALL AGES", "Other Room", ""), "unreviewed venue"},
		{"restricted", eventJSON("eyes", "Late Show", "late-show", "21+ only", "ANTE UP", ""), ""},
		{"cancelled", eventJSON("eyes", "CANCELLED: Eyes", "eyes-of-salt", "ALL AGES", "ANTE UP", ""), ""},
		{"status", eventJSON("eyes", "Eyes", "eyes-of-salt", "ALL AGES", "ANTE UP", "2"), "unreviewed event status"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run, err := Decode(cfg, snapshot(tc.events, "", "", ""), now)
			if err != nil {
				t.Fatal(err)
			}
			if tc.want != "" && run.Observations[0].Failure != tc.want {
				t.Fatal(run.Observations[0].Failure)
			}
			var d artifact.EventData
			json.Unmarshal(run.Observations[0].Data, &d)
			if tc.name == "restricted" && (d.AdmissionPolicy == nil || d.AdmissionPolicy.Category != "" || d.AdmissionPolicy.Text != "21+") {
				t.Fatalf("%+v", d.AdmissionPolicy)
			}
			if tc.name == "cancelled" && (d.Status != "Cancelled" || d.TicketURL != "" || d.AdmissionPolicy == nil) {
				t.Fatalf("%+v", d)
			}
		})
	}
}

func TestRefreshFailures(t *testing.T) {
	now := time.Date(2026, 9, 17, 18, 0, 0, 0, time.UTC)
	cfg := config(t)
	eyes := eventJSON("eyes", "EYES OF SALT", "eyes-of-salt", "ALL AGES", "ANTE UP", "")
	for _, tc := range []struct {
		name, events, more, policy, check string
	}{
		{"more", eyes, "true", "", ""},
		{"empty", "", "", "", ""},
		{"policy", eyes, "", "<main>21+ bar</main>", ""},
		{"changed", eyes, "", "", eventJSON("eyes", "Other", "other", "ALL AGES", "ANTE UP", "")},
		{"duplicate", eyes + "," + eyes, "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decode(cfg, snapshot(tc.events, tc.more, tc.policy, tc.check), now)
			if err == nil {
				t.Fatal("accepted")
			}
		})
	}
}

func TestUnlabeledClockStaysUnpublished(t *testing.T) {
	now := time.Date(2026, 9, 17, 18, 0, 0, 0, time.UTC)
	ev := eventJSON("eyes", "Swap", "musical-equipment-swap", `ALL AGES \n10:00 AM - 3:00 PM`, "ANTE UP", "")
	ev = strings.ReplaceAll(ev, "2026-09-18T01:00:57.686Z", "2026-10-04T16:00:57.686Z")
	ev = strings.ReplaceAll(ev, "September 17, 2026", "October 4, 2026")
	ev = strings.ReplaceAll(ev, "7:00 PM", "10:00 AM")
	raw := bytesReplace(snapshot(ev, "", "", ""), "2026-09-17T19:00:57-06:00", "2026-10-04T10:00:57-06:00")
	raw = bytesReplace(raw, `\"startTime\":\"7:00 PM\"`, `\"startTime\":\"10:00 AM\"`)
	run, err := Decode(config(t), raw, now)
	if err != nil {
		t.Fatal(err)
	}
	var d artifact.EventData
	json.Unmarshal(run.Observations[0].Data, &d)
	if d.Date != "2026-10-04" || d.DoorsAt != "" || d.ShowAt != "" || run.Observations[0].Failure != "" {
		t.Fatalf("%s %+v", run.Observations[0].Failure, d)
	}
}

func bytesReplace(b []byte, old, new string) []byte {
	return []byte(strings.ReplaceAll(string(b), old, new))
}
