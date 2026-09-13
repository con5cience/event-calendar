package web

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"event-calendar/internal/locale"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

type Config struct {
	Site                            locale.Config
	SiteDir                         string
	DataDir, AssetsDir, InitialDate string
	WeekLimit, MonthLimit, DayLimit int
	Now                             func() time.Time
}

func ConfigFromEnv() (Config, error) {
	c := Config{DataDir: env("DATA_DIR", ".artifacts"), AssetsDir: env("ASSETS_DIR", "dist"), InitialDate: os.Getenv("CALENDAR_INITIAL_DATE")}
	c.SiteDir = os.Getenv("SITE_DIR")
	var err error
	c.Site, err = locale.Load(c.SiteDir)
	if err != nil {
		return c, err
	}
	c.DataDir = env("DATA_DIR", filepath.Join(".artifacts", c.Site.ID))
	for _, item := range []struct {
		name              string
		fallback, minimum int
		dst               *int
	}{
		{"WEEK_EVENT_LIMIT", c.Site.Limits.Week, 1, &c.WeekLimit}, {"MONTH_EVENT_LIMIT", c.Site.Limits.Month, 1, &c.MonthLimit}, {"DAY_EVENT_LIMIT", c.Site.Limits.Day, 0, &c.DayLimit},
	} {
		n, err := strconv.Atoi(env(item.name, strconv.Itoa(item.fallback)))
		if err != nil || n < item.minimum {
			return c, fmt.Errorf("%s must be an integer >= %d", item.name, item.minimum)
		}
		*item.dst = n
	}
	if c.InitialDate != "" {
		if _, err := time.Parse("2006-01-02", c.InitialDate); err != nil {
			return c, fmt.Errorf("CALENDAR_INITIAL_DATE: %w", err)
		}
	}
	return c, nil
}
func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

type Event struct {
	PublicPath   string                   `json:"public_path"`
	Listed       bool                     `json:"listed"`
	Status       string                   `json:"status,omitempty"`
	ID           string                   `json:"id"`
	Title        string                   `json:"title"`
	Venue        string                   `json:"venue"`
	Date         string                   `json:"date"`
	Timezone     string                   `json:"timezone"`
	Artist       string                   `json:"artist,omitempty"`
	DoorsAt      string                   `json:"doors_at,omitempty"`
	ShowAt       string                   `json:"show_at,omitempty"`
	AgePolicy    string                   `json:"age_policy,omitempty"`
	EventURL     string                   `json:"event_url,omitempty"`
	TicketURL    string                   `json:"ticket_url,omitempty"`
	OffSite      bool                     `json:"off_site,omitempty"`
	AgePolicyURL string                   `json:"age_policy_url,omitempty"`
	AgeCategory  string                   `json:"age_category,omitempty"`
	WithAdult    *artifact.AdultAdmission `json:"with_adult,omitempty"`
}

func NewHandler(c Config) http.Handler {
	mux := http.NewServeMux()
	reader := &catalogReader{dir: c.DataDir, site: c.Site}
	mux.HandleFunc("GET /api/site", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodHead {
			_ = json.NewEncoder(w).Encode(c.Site.Presentation)
		}
	})
	if c.SiteDir != "" {
		mux.HandleFunc("GET /assets/favicon.png", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, filepath.Join(c.SiteDir, "assets", "favicon.png"))
		})
	}
	now := c.Now
	if now == nil {
		now = time.Now
	}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("GET /api/calendar", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		events, err := reader.events(now())
		if err != nil {
			log.Printf("calendar artifact: %v", err)
			if events == nil {
				http.Error(w, "Calendar data is unavailable", http.StatusServiceUnavailable)
				return
			}
		}
		var day *int
		if c.DayLimit > 0 {
			day = &c.DayLimit
		}
		payload := struct {
			Events      []Event        `json:"events"`
			Limits      map[string]any `json:"limits"`
			InitialDate string         `json:"initial_date,omitempty"`
		}{events, map[string]any{"week": c.WeekLimit, "month": c.MonthLimit, "day": day}, c.InitialDate}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			log.Printf("calendar response: %v", err)
		}
	})
	assets := http.FileServer(http.Dir(c.AssetsDir))
	mux.HandleFunc("GET /robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if r.Method != http.MethodHead {
			fmt.Fprint(w, "User-agent: *\nAllow: /\nSitemap: "+c.Site.Origin+"/sitemap.xml\n")
		}
	})
	mux.HandleFunc("GET /sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		events, err := reader.events(now())
		if err != nil {
			log.Printf("sitemap artifact: %v", err)
			if events == nil {
				http.Error(w, "Sitemap is unavailable", http.StatusServiceUnavailable)
				return
			}
		}
		serveSitemap(w, r, events, c.Site.Origin)
	})
	eventHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		all, err := reader.retained(now())
		if err != nil {
			log.Printf("event artifact: %v", err)
			if all == nil {
				http.Error(w, "Event data is unavailable", http.StatusServiceUnavailable)
				return
			}
		}
		path := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api"), ".ics")
		for _, e := range all {
			if e.PublicPath != path {
				continue
			}
			if strings.HasSuffix(r.URL.Path, ".ics") {
				body, err := eventCalendar(e, now(), c.Site.ICSNamespace)
				if err != nil {
					http.Error(w, "Event export is unavailable", 503)
					return
				}
				w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
				w.Header().Set("Content-Disposition", "attachment; filename=\""+e.ID+".ics\"")
				fmt.Fprint(w, body)
			} else if strings.HasPrefix(r.URL.Path, "/api/") {
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(e); err != nil {
					log.Printf("event response: %v", err)
				}
			} else {
				servePage(w, r, c.AssetsDir, eventPage(e, c.Site))
			}
			return
		}
		http.NotFound(w, r)
	}
	mux.HandleFunc("GET /events/", eventHandler)
	mux.HandleFunc("GET /api/events/", eventHandler)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			events, err := reader.events(now())
			if err != nil {
				log.Printf("homepage artifact: %v", err)
			}
			servePage(w, r, c.AssetsDir, pageMetadata{Site: c.Site, Title: c.Site.Title(), Description: c.Site.Description, Canonical: c.Site.Origin + "/", Events: events, Unavailable: events == nil && err != nil})
			return
		}
		if r.URL.Path != "/" && !strings.HasPrefix(r.URL.Path, "/assets/") {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path != "/" {
			info, err := os.Stat(filepath.Join(c.AssetsDir, filepath.Clean(r.URL.Path)))
			if err != nil || info.IsDir() {
				http.NotFound(w, r)
				return
			}
		}
		assets.ServeHTTP(w, r)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'")
		mux.ServeHTTP(w, r)
	})
}
