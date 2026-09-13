package web

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Never derive public metadata URLs from the request Host or forwarded headers.
const publicOrigin = "https://denver.withadult.com"
const siteTitle = "withAdult(denver): Bring your people."
const siteDescription = "Find Denver-area events and explore reviewed venue admission policies with the With Adult age filter. Check event and venue details before buying tickets."

type pageMetadata struct {
	Title, Description, Canonical string
	Event                         *Event
	Events                        []Event
	Unavailable                   bool
}

var metadataTemplate = template.Must(template.New("metadata").Parse(`
<title>{{.Title}}</title>
<meta name="description" content="{{.Description}}">
<link rel="canonical" href="{{.Canonical}}">
<meta property="og:site_name" content="withAdult(denver)">
<meta property="og:type" content="website">
<meta property="og:title" content="{{.Title}}">
<meta property="og:description" content="{{.Description}}">
<meta property="og:url" content="{{.Canonical}}">
<meta property="og:image" content="https://denver.withadult.com/assets/favicon.png">
<meta property="og:image:type" content="image/png">
<meta property="og:image:width" content="554">
<meta property="og:image:height" content="554">
<meta property="og:image:alt" content="Purple Denver emblem for withAdult(denver)">
<meta name="twitter:card" content="summary">
<meta name="twitter:title" content="{{.Title}}">
<meta name="twitter:description" content="{{.Description}}">
<meta name="twitter:image" content="https://denver.withadult.com/assets/favicon.png">
<meta name="twitter:image:alt" content="Purple Denver emblem for withAdult(denver)">
{{if .Event}}{{if not .Event.Listed}}<meta name="robots" content="noindex">{{end}}{{end}}
`))

// This is visible content for browsers without JavaScript and for crawlers.
// React replaces the root contents on startup; no crawler-specific response.
var fallbackTemplate = template.Must(template.New("fallback").Parse(`<main>
{{if .Event}}{{with .Event}}
<h1>{{.Title}}</h1><p><time datetime="{{.Date}}">{{.Date}}</time> · {{.Venue}}</p>
{{if .Status}}<p>Status: {{.Status}}</p>{{end}}
{{if .AgePolicy}}<p>Age Policy: {{.AgePolicy}}</p>{{end}}
<p>Check event details and venue admission policy before buying tickets.</p>
{{if .EventURL}}<p><a href="{{.EventURL}}" target="_blank" rel="noopener noreferrer">View Event</a></p>{{end}}
{{if .TicketURL}}<p><a href="{{.TicketURL}}" target="_blank" rel="noopener noreferrer">Buy Tickets</a></p>{{end}}
{{if .AgePolicyURL}}<p><a href="{{.AgePolicyURL}}" target="_blank" rel="noopener noreferrer">Venue admission policy</a></p>{{end}}
<p><a href="/">Browse the calendar</a></p>
{{end}}{{else}}
<h1>withAdult(denver): Bring your people.</h1><p>` + siteDescription + `</p>
{{if .Unavailable}}<p>Calendar data is unavailable</p>{{else if .Events}}
<ul>{{range .Events}}<li><a href="{{.PublicPath}}">{{.Date}}: {{.Title}} @ {{.Venue}}</a></li>{{end}}</ul>
{{else}}<p>No events available</p>{{end}}
{{end}}
<noscript>Enable JavaScript to use the interactive calendar and filters.</noscript>
</main>`))

var shellTitle = regexp.MustCompile(`<title>[^<]*</title>`)
var shellRoot = regexp.MustCompile(`<div id="root">\s*</div>`)

func servePage(w http.ResponseWriter, r *http.Request, assetsDir string, data pageMetadata) {
	shell, err := os.ReadFile(filepath.Join(assetsDir, "index.html"))
	if err != nil || len(shellTitle.FindAllIndex(shell, -1)) != 1 || len(shellRoot.FindAllIndex(shell, -1)) != 1 {
		log.Printf("page shell unavailable or invalid: %v", err)
		http.Error(w, "Page is unavailable", http.StatusServiceUnavailable)
		return
	}
	var head, content bytes.Buffer
	if err := metadataTemplate.Execute(&head, data); err != nil {
		http.Error(w, "Page is unavailable", http.StatusServiceUnavailable)
		return
	}
	if err := fallbackTemplate.Execute(&content, data); err != nil {
		http.Error(w, "Page is unavailable", http.StatusServiceUnavailable)
		return
	}
	// ReplaceAllFunc keeps dollar signs in escaped upstream text literal.
	shell = shellTitle.ReplaceAllFunc(shell, func([]byte) []byte { return head.Bytes() })
	shell = shellRoot.ReplaceAllFunc(shell, func([]byte) []byte { return []byte(`<div id="root">` + content.String() + `</div>`) })
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodHead {
		_, _ = w.Write(shell)
	}
}

func eventPage(e Event) pageMetadata {
	description := fmt.Sprintf("%s at %s on %s.", e.Title, e.Venue, e.Date)
	if e.AgePolicy != "" {
		description += " Age Policy: " + e.AgePolicy + "."
	}
	if strings.EqualFold(e.Status, "cancelled") || strings.EqualFold(e.Status, "canceled") {
		description = "Cancelled. " + description
	}
	return pageMetadata{Title: e.Title + " @ " + e.Venue + " · " + e.Date + " | withAdult(denver)", Description: description, Canonical: publicOrigin + e.PublicPath, Event: &e}
}

type sitemapURL struct {
	Location string `xml:"loc"`
}
type sitemapDocument struct {
	XMLName xml.Name     `xml:"urlset"`
	XMLNS   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

func serveSitemap(w http.ResponseWriter, r *http.Request, events []Event) {
	document := sitemapDocument{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9", URLs: []sitemapURL{{publicOrigin + "/"}}}
	sort.Slice(events, func(i, j int) bool { return events[i].PublicPath < events[j].PublicPath })
	for _, e := range events {
		document.URLs = append(document.URLs, sitemapURL{publicOrigin + e.PublicPath})
	}
	body, err := xml.Marshal(document)
	if err != nil {
		http.Error(w, "Sitemap is unavailable", 503)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodHead {
		fmt.Fprint(w, xml.Header+string(body))
	}
}
