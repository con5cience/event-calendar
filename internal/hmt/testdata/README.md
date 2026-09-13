# Synthetic HoldMyTicket snapshots

The JSON files contain invented events, not captured public listings. Do not
publish them into the normal calendar store. YAML files identify HQ (feed 6457)
and The Oriental Theater (feed 801), plus The Federal Theatre (user feed 8693);
`state: new` permits only first publication. Federal's live subscription route is
`https://holdmyticket.com/ics_user/8693`.

A replay snapshot is `{ "calendar": "...complete iCalendar text...", "details":
{ "provider-event-id": { "@type": "Event", "...": "JSON-LD fields" } } }`.
The detail object is the Event JSON-LD from the URL carried by the feed. It is data,
not executable HTML. Preserve original field values during capture. Never use the
synthetic ticket ID `1001` as real evidence.

The adapter uses the existing golang-ical parser. The provider numeric event URL
identifies an occurrence; UID must also be present and unique. Feed and detail
titles, venue, and show times must agree. Unknown recurrences fail the snapshot.
Details must accompany each in-range event; an invalid observation is reported and
retains a previous valid version through the shared reconciler.

HQ and Oriental have no accompaniment exception configured. Their published age
labels support exact Age filtering but do not match With adult without reviewed
metadata. Federal's explicitly All Ages details receive event-linked metadata for
ages 0–17; restricted or ambiguous labels remain unknown. Overrides still win.
`federal_test.go` covers these boundaries and the Federal-only description cleanup.
Raw descriptions are not published; uncertain block boundaries fail closed.
Fixtures retain raw offer prices to verify they are ignored. Only offer URLs are mapped;
no price text or cost category is published.

Run the Go tests through `npm run test:contracts`. See README for replay and the
opt-in desktop/phone browser command. No live HTTP client or scheduler is included.
