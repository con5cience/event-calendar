import {
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
} from "react";
import { site } from "./site";
import FullCalendar, { type CalendarRef } from "@fullcalendar/react";
import dayGridPlugin from "@fullcalendar/react/daygrid";
import listPlugin from "@fullcalendar/react/list";
import interactionPlugin from "@fullcalendar/react/interaction";
import { FilterDropdown } from "./FilterDropdown";
import { VenueMarker } from "./VenueMarker";
import { venueColor } from "./venueColors";
import { ActionIcon } from "./ActionIcon";
import { ActionTooltip } from "./ActionTooltip";
import { readView, saveView, type View } from "./view";
import { containDialogFocus, isBackdropInteraction } from "./dialog";
import monarchPlugin from "@fullcalendar/react/themes/monarch";
import {
  defaultPreferences,
  adultCondition,
  filterEvents,
  readPreferences,
  savePreferences,
} from "./filters";
import {
  displayTime,
  displayStatus,
  displayAgePolicy,
  projectEvents,
  type CalendarData,
  type CalendarEvent,
  type ProjectedEvent,
} from "./calendar";

const plugins = [dayGridPlugin, listPlugin, interactionPlugin, monarchPlugin];
const viewName = (view: View, mobile: boolean) =>
  `${mobile ? "list" : "dayGrid"}${view[0].toUpperCase()}${view.slice(1)}`;

export function App() {
  useLayoutEffect(() => {
    // React has replaced the fallback; reveal the app before this frame paints.
    window.dispatchEvent(new Event("calendar-startup-ready"));
  }, []);
  const [data, setData] = useState<CalendarData | null>(null);
  const [error, setError] = useState(false);
  useEffect(() => {
    const controller = new AbortController();
    fetch("/api/calendar", { signal: controller.signal })
      .then((response) => {
        if (!response.ok) throw new Error("Calendar unavailable");
        return response.json() as Promise<CalendarData>;
      })
      .then(setData)
      .catch(() => {
        if (!controller.signal.aborted) setError(true);
      });
    return () => controller.abort();
  }, []);
  return (
    <div className="app-shell">
      <header className="page-header">
        <span className="brand-name">
          withAdult(<span className="brand-city">{site.city}</span>)
        </span>
        <span className="brand-tagline">: {site.tagline}</span>
      </header>
      <main>
        {error ? (
          <p role="alert">Unable to load events. Please try again later.</p>
        ) : data ? (
          <Calendar data={data} />
        ) : (
          <p role="status">Loading calendar…</p>
        )}
      </main>
    </div>
  );
}

function Calendar({ data }: { data: CalendarData }) {
  const ref = useRef<CalendarRef>(null);
  const temporaryDefaults = useRef(location.pathname.startsWith("/events/"));
  const [preferences, updatePreferences] = useState(() =>
    temporaryDefaults.current ? defaultPreferences() : readPreferences(),
  );
  function setPreferences(value: Parameters<typeof updatePreferences>[0]) {
    temporaryDefaults.current = false;
    updatePreferences(value);
  }
  const [path, setPath] = useState(location.pathname);
  const [detailError, setDetailError] = useState("");
  const [copyStatus, setCopyStatus] = useState("");
  const cardPath = useRef<string | null>(null);
  const historyClosing = useRef(false);
  const backdropPointerDown = useRef(false);
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    if (!temporaryDefaults.current) savePreferences(preferences);
  }, [preferences]);
  useEffect(() => {
    const update = () => setNow(new Date());
    const timer = window.setInterval(update, 60_000);
    window.addEventListener("focus", update);
    return () => {
      window.clearInterval(timer);
      window.removeEventListener("focus", update);
    };
  }, []);
  const venues = useMemo(
    () =>
      [
        ...new Set([
          ...data.events.map((event) => event.venue),
          ...preferences.venues,
        ]),
      ].sort((a, b) => a.localeCompare(b)),
    [data.events, preferences.venues],
  );
  const matchingEvents = useMemo(
    () => filterEvents(data.events, preferences),
    [data.events, preferences],
  );
  const [view, setView] = useState<View>(readView);
  function selectView(value: View) {
    saveView(value);
    setView(value);
  }
  const [mobile, setMobile] = useState(
    () => matchMedia("(max-width: 767px)").matches,
  );
  const [title, setTitle] = useState("");
  const [selected, setSelected] = useState<CalendarEvent | null>(null);
  const dialogRef = useRef<HTMLDialogElement>(null);
  const viewPickerRef = useRef<HTMLDivElement>(null);
  const returnFocus = useRef<HTMLElement | null>(null);
  const actualView = viewName(view, mobile);
  const fitGrid = view !== "day" && !mobile;
  const [dayExpanded, setDayExpanded] = useState(false);
  const limit =
    (view === "day" && dayExpanded) || fitGrid ? null : data.limits[view];
  const events = useMemo(
    () => projectEvents(matchingEvents, limit, mobile || fitGrid),
    [matchingEvents, limit, mobile, fitGrid],
  );
  useEffect(() => {
    const media = matchMedia("(max-width: 767px)");
    const update = () => setMobile(media.matches);
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);
  useEffect(() => {
    ref.current?.getApi().changeView(actualView);
  }, [actualView]);
  useEffect(() => {
    if (selected && !dialogRef.current?.open) dialogRef.current?.showModal();
  }, [selected]);
  useEffect(() => {
    const update = () => setPath(location.pathname);
    window.addEventListener("popstate", update);
    return () => window.removeEventListener("popstate", update);
  }, []);
  useEffect(() => {
    setCopyStatus("");
    setDetailError("");
    if (!path.startsWith("/events/")) {
      setSelected(null);
      if (dialogRef.current?.open) {
        historyClosing.current = true;
        dialogRef.current.close();
      }
      return;
    }
    if (cardPath.current === path) {
      cardPath.current = null;
      return;
    }
    const controller = new AbortController();
    fetch("/api" + path, { signal: controller.signal })
      .then(async (response) => {
        if (!response.ok)
          throw new Error(
            response.status === 404
              ? "Event not found"
              : "Unable to load event details",
          );
        return response.json() as Promise<CalendarEvent>;
      })
      .then((event) => {
        if (controller.signal.aborted) return;
        setView("week");
        ref.current?.getApi().changeView(viewName("week", mobile), event.date);
        setSelected(event);
      })
      .catch((error: Error) => {
        if (!controller.signal.aborted) setDetailError(error.message);
      });
    return () => controller.abort();
  }, [path, mobile]);

  function drillDay(date: string | Date, expand = false) {
    setDayExpanded(expand);
    selectView("day");
    ref.current?.getApi().changeView(viewName("day", mobile), date);
  }
  function openEvent(event: CalendarEvent) {
    returnFocus.current =
      document.activeElement instanceof HTMLElement
        ? document.activeElement
        : null;
    setSelected(event);
    setCopyStatus("");
    if (event.public_path && location.pathname !== event.public_path) {
      cardPath.current = event.public_path;
      history.pushState(null, "", event.public_path);
      setPath(event.public_path);
    }
  }
  const today = new Intl.DateTimeFormat("en-CA", {
    timeZone: site.timezone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(now);
  const emptyMessage =
    data.events.length === 0
      ? "No events available"
      : "No events match your filters.";
  return (
    <>
      {detailError && <p role="alert">{detailError}</p>}
      <section aria-label="Event calendar">
        <div className="toolbar">
          <div className="navigation">
            <button
              onClick={() => {
                setDayExpanded(false);
                ref.current?.getApi().prev();
              }}
              aria-label="Previous"
            >
              ←
            </button>
            <button
              onClick={() => {
                setDayExpanded(false);
                ref.current?.getApi().gotoDate(today);
              }}
            >
              Today
            </button>
            <button
              onClick={() => {
                setDayExpanded(false);
                ref.current?.getApi().next();
              }}
              aria-label="Next"
            >
              →
            </button>
          </div>
          <h2 data-testid="range" aria-live="polite">
            {title}
          </h2>
          <div className="toolbar-filters">
            <FilterDropdown
              label="Venues"
              emptyLabel="All venues"
              plural="venues"
              options={venues}
              renderMarker={(venue) => <VenueMarker venue={venue} />}
              selected={preferences.venues}
              onChange={(venues) =>
                setPreferences((current) => ({ ...current, venues }))
              }
            />
            <div className="toolbar-admission">
              <label className="adult-filter">
                <input
                  type="checkbox"
                  checked={preferences.withAdult}
                  onChange={(event) =>
                    setPreferences((current) => ({
                      ...current,
                      withAdult: event.target.checked,
                    }))
                  }
                />
                With Adult
              </label>
              <ActionTooltip
                help
                label="Shows events whose reviewed venue policies allow someone of the selected age to attend with an adult. Conditions vary—check the event details and venue policy before buying tickets."
              >
                <button
                  type="button"
                  className="admission-help-button"
                  aria-label="About With Adult"
                >
                  <span aria-hidden="true">ⓘ</span>
                </button>
              </ActionTooltip>
              {preferences.withAdult && (
                <label className="search-field toolbar-child-age">
                  <span className="child-age-separator" aria-hidden="true">
                    ·
                  </span>
                  <span className="child-age-caption" aria-hidden="true">
                    Age:
                  </span>
                  <span id="child-age-label" className="sr-only">
                    Child’s age
                  </span>
                  <select
                    aria-labelledby="child-age-label"
                    value={preferences.childAge}
                    onChange={(event) =>
                      setPreferences((current) => ({
                        ...current,
                        childAge: Number(event.target.value),
                      }))
                    }
                    aria-describedby="adult-filter-help"
                  >
                    {Array.from({ length: 18 }, (_, age) => (
                      <option key={age} value={age}>
                        {age}
                      </option>
                    ))}
                  </select>
                  <small id="adult-filter-help" className="sr-only">
                    Ages 0–17. Only reviewed policies match. Follow the
                    accompaniment conditions in event details.
                  </small>
                </label>
              )}
            </div>
          </div>
          <div className="view-search">
            <label className="search-field toolbar-search">
              <input
                type="search"
                aria-label="Search events"
                value={preferences.query}
                // Keep :placeholder-shown available for the icon without visible text.
                placeholder=" "
                onChange={(event) =>
                  setPreferences((current) => ({
                    ...current,
                    query: event.target.value,
                  }))
                }
              />
              <ActionIcon name="magnifying-glass" />
            </label>
            <div
              ref={viewPickerRef}
              className="view-picker"
              aria-label="Calendar view"
            >
              {(["day", "week", "month"] as View[]).map((option) => (
                <button
                  key={option}
                  aria-pressed={view === option}
                  onClick={() => {
                    setDayExpanded(false);
                    selectView(option);
                  }}
                >
                  {option[0].toUpperCase() + option.slice(1)}
                </button>
              ))}
            </div>
          </div>
        </div>
        {matchingEvents.length === 0 && !mobile && (
          <p className="empty" role="status">
            {emptyMessage}
          </p>
        )}
        <div data-testid="calendar" data-view={actualView}>
          <FullCalendar
            ref={ref}
            plugins={plugins}
            initialView={actualView}
            views={{
              dayGridMonth: {
                fixedWeekCount: false,
                showNonCurrentDates: false,
              },
            }}
            initialDate={data.initial_date || today}
            firstDay={site.week_start}
            locale={site.language}
            timeZone="UTC"
            headerToolbar={false}
            height={fitGrid ? "100%" : "auto"}
            titleFormat={
              view === "month"
                ? { year: "numeric", month: "long" }
                : { year: "numeric", month: "short", day: "numeric" }
            }
            events={events}
            eventOrder="sortIndex"
            eventOrderStrict
            displayEventTime={false}
            eventInteractive={false}
            dayMaxEvents={fitGrid ? true : limit || false}
            eventSlicing={!fitGrid}
            navLinks
            navLinkDayClick={(date) => drillDay(date)}
            moreLinkClass="calendar-more-link"
            moreLinkClick={(info) => {
              drillDay(info.date, true);
              return viewName("day", mobile);
            }}
            moreLinkContent={(info) =>
              fitGrid ? "Show All" : `Show All (${info.num} more)`
            }
            datesSet={(info) => setTitle(info.view.title)}
            noEventsContent={
              matchingEvents.length === 0 ? emptyMessage : "No events available"
            }
            listDayClass="mobile-date-section"
            listDayHeaderClass="mobile-date-header"
            listDayHeaderInnerClass={(info) =>
              info.level ? "mobile-date-extra" : "mobile-date-link"
            }
            listDayBodyClass="mobile-date-events"
            listItemEventBeforeClass="mobile-event-marker"
            listItemEventInnerClass="mobile-event-inner"
            listDayFormat={{ weekday: "short", month: "short", day: "numeric" }}
            listDayAltFormat={false}
            listDayHeaderContent={(info) =>
              info.level ? null : (
                <span
                  data-testid={`date-${info.date.toISOString().slice(0, 10)}`}
                >
                  {info.text}
                </span>
              )
            }
            dayHeaderContent={(info) => (
              <button
                className="date-button"
                data-calendar-day-header
                onClick={() => drillDay(info.date)}
              >
                {view === "week" && !mobile ? (
                  <span
                    className={
                      info.isToday
                        ? "week-date-label week-today-label"
                        : "week-date-label"
                    }
                  >
                    {info.text}
                  </span>
                ) : (
                  info.text
                )}
              </button>
            )}
            dayCellClass={(info) =>
              view === "week" && !mobile && info.isToday
                ? "week-today-cell"
                : ""
            }
            eventClass="calendar-event-shell"
            eventContent={(info) => {
              const row = info.event
                .extendedProps as ProjectedEvent["extendedProps"];
              if (row.kind === "more")
                return (
                  <button
                    className="event-card show-all"
                    onClick={() => drillDay(row.date, true)}
                  >
                    {fitGrid
                      ? "Show All"
                      : `Show All (${row.hiddenCount} more)`}
                  </button>
                );
              const event = row.record!;
              const cancelled = displayStatus(event) === "Cancelled";
              return (
                <button
                  className="event-card"
                  data-testid={`event-${event.id}`}
                  data-event-date={event.date}
                  style={
                    {
                      "--venue-color": venueColor(event.venue),
                    } as CSSProperties
                  }
                  onClick={() => openEvent(event)}
                >
                  <span
                    className={`event-summary${cancelled ? " cancelled-summary" : ""}`}
                  >
                    <span className="event-title">{event.title}</span>
                    {view === "day" && ` @ ${event.venue}`}
                  </span>
                </button>
              );
            }}
          />
        </div>
      </section>
      <dialog
        ref={dialogRef}
        aria-labelledby="event-title"
        onPointerDown={(event) => {
          backdropPointerDown.current =
            event.button === 0 && isBackdropInteraction(event);
        }}
        onPointerCancel={() => {
          backdropPointerDown.current = false;
        }}
        onClick={(event) => {
          const dismiss =
            backdropPointerDown.current && isBackdropInteraction(event);
          backdropPointerDown.current = false;
          if (dismiss) event.currentTarget.close();
        }}
        onKeyDown={containDialogFocus}
        onClose={() => {
          backdropPointerDown.current = false;
          if (historyClosing.current) {
            historyClosing.current = false;
            return;
          }
          if (location.pathname.startsWith("/events/")) {
            history.pushState(null, "", "/");
            setPath("/");
          }
          setSelected(null);
          if (returnFocus.current?.isConnected) returnFocus.current.focus();
          else
            viewPickerRef.current
              ?.querySelector<HTMLButtonElement>('[aria-pressed="true"]')
              ?.focus();
        }}
      >
        {selected && (
          <>
            <button
              className="dialog-close"
              onClick={() => dialogRef.current?.close()}
              autoFocus
              aria-label="Close event details"
            >
              ×
            </button>
            <h2 id="event-title">
              <time className="event-heading-date" dateTime={selected.date}>
                {selected.date}
              </time>{" "}
              : <span className="event-heading-title">{selected.title}</span>
            </h2>
            {selected.listed === false && <p>No longer listed</p>}
            <dl>
              <dt>Venue</dt>
              <dd>
                <VenueMarker venue={selected.venue} />
                {selected.venue}
              </dd>
              <dt>Status</dt>
              <dd
                className={
                  displayStatus(selected) === "Cancelled"
                    ? "event-cancelled"
                    : undefined
                }
              >
                {displayStatus(selected)}
              </dd>
              {selected.off_site && (
                <>
                  <dt>Location</dt>
                  <dd>Off-site</dd>
                </>
              )}
              {displayTime(selected) && (
                <>
                  <dt>{selected.doors_at ? "Doors" : "Show"}</dt>
                  <dd>{displayTime(selected)}</dd>
                </>
              )}
              {selected.age_policy && (
                <>
                  <dt>Age Policy</dt>
                  <dd>
                    {displayAgePolicy(selected.age_policy)}
                    {selected.age_policy_url && (
                      <>
                        {" "}
                        <VenuePolicyLink url={selected.age_policy_url} />
                      </>
                    )}
                  </dd>
                </>
              )}
              {selected.with_adult && (
                <>
                  <dt className="admission-label">With Adult</dt>
                  <dd className="admission-clearance">
                    {adultCondition(selected, preferences.childAge)
                      ? `Age ${preferences.childAge}: ${adultCondition(selected, preferences.childAge)}.`
                      : `No reviewed permission for age ${Number.isInteger(preferences.childAge) ? preferences.childAge : "—"}.`}{" "}
                    <VenuePolicyLink url={selected.with_adult.url} />
                  </dd>
                </>
              )}
            </dl>
            <div className="event-links" key={selected.id}>
              {selected.event_url && (
                <ActionTooltip label="Copy Event Link">
                  <button
                    aria-label="Copy Event Link"
                    onClick={async () => {
                      try {
                        await navigator.clipboard.writeText(
                          selected.event_url!,
                        );
                        setCopyStatus("Event link copied");
                      } catch {
                        setCopyStatus(
                          `Copy this event link: ${selected.event_url}`,
                        );
                      }
                    }}
                  >
                    <ActionIcon name="copy" />
                  </button>
                </ActionTooltip>
              )}
              {selected.public_path && (
                <ActionTooltip label="Download Calendar Entry">
                  <a
                    href={"/api" + selected.public_path + ".ics"}
                    download
                    aria-label="Download Calendar Entry"
                  >
                    <ActionIcon name="download" />
                  </a>
                </ActionTooltip>
              )}
              {selected.event_url && (
                <ActionTooltip label="View Event">
                  <a
                    href={selected.event_url}
                    aria-label="View Event"
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    <ActionIcon name="arrow-up-right-from-square" />
                  </a>
                </ActionTooltip>
              )}
              {selected.ticket_url && (
                <ActionTooltip label="Buy Tickets">
                  <a
                    href={selected.ticket_url}
                    aria-label="Buy Tickets"
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    <ActionIcon name="ticket" />
                  </a>
                </ActionTooltip>
              )}
            </div>
            {copyStatus && <p role="status">{copyStatus}</p>}
          </>
        )}
      </dialog>
    </>
  );
}

function VenuePolicyLink({ url }: { url: string }) {
  return (
    <ActionTooltip label="Venue admission policy" inline>
      <a
        href={url}
        aria-label="Venue admission policy"
        className="policy-icon-link"
        target="_blank"
        rel="noopener noreferrer"
      >
        <ActionIcon name="arrow-up-right-from-square" />
      </a>
    </ActionTooltip>
  );
}
