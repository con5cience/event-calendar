package store_test

import (
	"encoding/json"
	"event-calendar/internal/store"
	"event-calendar/internal/web"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestPublishedBytesReachCalendarHTTP(t *testing.T) {
	b, err := os.ReadFile("../../tests/contracts/source.json")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	h := web.NewHandler(web.Config{DataDir: dir, WeekLimit: 10, MonthLimit: 5, Now: func() time.Time { return time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC) }})
	check := func(count int) {
		t.Helper()
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/api/calendar", nil))
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var data struct {
			Events []web.Event `json:"events"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		if len(data.Events) != count {
			t.Fatal(w.Body.String())
		}
		if count > 0 && data.Events[0].AgePolicy != "16+" {
			t.Fatal(w.Body.String())
		}
	}
	check(0)
	r, err := (&store.Publisher{}).Publish(dir, "none", [][]byte{b})
	if err != nil || !r.Published {
		t.Fatal(err)
	}
	check(1)
	if _, err := (&store.Publisher{}).Publish(dir, r.Generation, [][]byte{[]byte(`{}`)}); err == nil {
		t.Fatal("accepted bad candidate")
	}
	check(1)
}
