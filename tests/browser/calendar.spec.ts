import { selectCalendarView } from "./view-controls";
import { expect, test } from "@playwright/test";

test.beforeEach(async ({ page }) => {
  await page.clock.setFixedTime(new Date("2026-09-09T01:00:00Z"));
});

test("explicit month selection survives reload on both layouts", async ({
  page,
}, info) => {
  await page.goto("/");
  await selectCalendarView(page, "Month");
  const view = info.project.name === "phone" ? "listMonth" : "dayGridMonth";
  await expect(page.getByTestId("calendar")).toHaveAttribute("data-view", view);
  await expect(
    page.getByRole("button", {
      name: "Month",
      exact: true,
      includeHidden: true,
    }),
  ).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByTestId("range")).toHaveAttribute(
    "aria-label",
    "September 2026",
  );
  await expect
    .poll(() => page.locator('[data-event-date="2026-09-08"]:visible').count())
    .toBeGreaterThan(0);
  expect(
    await page.locator('[data-event-date="2026-09-08"]:visible').count(),
  ).toBeLessThanOrEqual(info.project.name === "phone" ? 5 : 14);
  if (info.project.name === "phone")
    await expect(
      page.locator('[data-event-date="2026-09-08"]:visible'),
    ).toHaveCount(5);
  await page.reload();
  await expect(page.getByTestId("calendar")).toHaveAttribute("data-view", view);
});

test("compact branded header preserves toolbar access", async ({
  page,
}, info) => {
  await page.goto("/");
  const toggle = page.getByRole("button", { name: "Previous", exact: true });
  await expect(toggle).toBeVisible();
  await expect(page.locator(".page-header")).toContainText("withAdult(denver)");
  expect((await page.locator(".page-header").boundingBox())?.height).toBe(36);
  expect((await page.getByTestId("calendar").boundingBox())!.y).toBeLessThan(
    info.project.name === "phone" ? 316 : 196,
  );
  await expect(page.getByRole("searchbox")).toBeVisible();
});

test("month shows only its dates and uses the required week rows", async ({
  page,
}, info) => {
  await page.route("**/api/calendar", async (route) => {
    const response = await route.fetch();
    const data = await response.json();
    for (const date of [
      "2026-08-31",
      "2026-09-30",
      "2026-10-01",
      "2026-10-05",
    ]) {
      data.events.push({
        id: date,
        title: "Boundary " + date,
        venue: "Gothic Theatre",
        date,
        timezone: "America/Denver",
      });
    }
    await route.fulfill({ json: data });
  });
  await page.goto("/");
  await selectCalendarView(page, "Month");
  await expect(page.getByTestId("range")).toHaveAttribute(
    "aria-label",
    "September 2026",
  );
  await expect(page.getByTestId("event-2026-09-30")).toBeVisible();
  for (const date of ["2026-08-31", "2026-10-01", "2026-10-05"]) {
    await expect(page.getByTestId("event-" + date)).toHaveCount(0);
  }
  if (info.project.name === "desktop") {
    await expect(page.getByRole("row", { name: /^Week / })).toHaveCount(5);
    await expect(
      page.getByRole("gridcell", { name: "October 1, 2026", exact: true }),
    ).toHaveCount(0);
  }
  await page.getByRole("button", { name: "Next", exact: true }).click();
  await expect(page.getByTestId("range")).toHaveAttribute(
    "aria-label",
    "October 2026",
  );
  await expect(page.getByTestId("event-2026-10-01")).toBeVisible();
  await expect(page.getByTestId("event-2026-09-30")).toHaveCount(0);
  await page.getByRole("button", { name: "Previous", exact: true }).click();
  await page.getByRole("button", { name: "Previous", exact: true }).click();
  await expect(page.getByTestId("range")).toHaveAttribute(
    "aria-label",
    "August 2026",
  );
  await expect(page.getByTestId("event-2026-08-31")).toBeVisible();
  if (info.project.name === "desktop") {
    await expect(page.getByRole("row", { name: /^Week / })).toHaveCount(6);
  }
});

test("records a small-fixture performance baseline", async ({ page }, info) => {
  const samples = [];
  for (let sample = 1; sample <= 5; sample++) {
    const started = performance.now();
    await page.goto("/");
    await selectCalendarView(page, "Week");
    await expect(
      page.locator('[data-event-date="2026-09-08"]:visible'),
    ).toHaveCount(info.project.name === "phone" ? 10 : 14);
    const readyMs = performance.now() - started;
    const navigationStarted = performance.now();
    await selectCalendarView(page, "Month");
    await expect
      .poll(() =>
        page.locator('[data-event-date="2026-09-08"]:visible').count(),
      )
      .toBeGreaterThan(0);
    expect(
      await page.locator('[data-event-date="2026-09-08"]:visible').count(),
    ).toBeLessThanOrEqual(info.project.name === "phone" ? 5 : 14);
    const monthMs = performance.now() - navigationStarted;
    samples.push({
      sample,
      readyMs: Math.round(readyMs),
      monthMs: Math.round(monthMs),
    });
  }
  console.log(
    JSON.stringify({
      baseline: info.project.name,
      fixture: "16 visible synthetic events",
      samples,
    }),
  );
  await info.attach("performance-baseline", {
    body: JSON.stringify(samples, null, 2),
    contentType: "application/json",
  });
});

test("phone cards use available width and date heading opens the day", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  const first = page.getByTestId("event-gothic-000000000001");
  await expect(first).toBeVisible();
  const card = await first.boundingBox();
  const calendar = await page.getByTestId("calendar").boundingBox();
  expect(card!.width / calendar!.width).toBeGreaterThan(0.8);
  const colors = await page.getByTestId("date-2026-09-08").evaluate((node) => {
    const style = getComputedStyle(node.parentElement!);
    return [style.color, style.backgroundColor];
  });
  const luminance = (color: string) => {
    const channels = color
      .match(/[\d.]+/g)!
      .slice(0, 3)
      .map(Number)
      .map((value) => {
        const v = value / 255;
        return v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4;
      });
    return channels[0] * 0.2126 + channels[1] * 0.7152 + channels[2] * 0.0722;
  };
  const values = colors.map(luminance).sort((a, b) => b - a);
  expect((values[0] + 0.05) / (values[1] + 0.05)).toBeGreaterThanOrEqual(4.5);
  await page.getByTestId("date-2026-09-08").click();
  await expect(page.getByTestId("calendar")).toHaveAttribute(
    "data-view",
    "listDay",
  );
  await expect(
    page.locator('[data-event-date="2026-09-08"]:visible'),
  ).toHaveCount(14);
});

test("fixture HTTP contract", async ({ request }) => {
  const response = await request.get("/api/calendar");
  expect(response.status(), await response.text()).toBe(200);
  const data = await response.json();
  expect(data.events.map((event: { id: string }) => event.id).sort()).toEqual([
    "gothic-000000000001",
    "gothic-000000000004",
    "gothic-000000000015",
    "hq-000000000007",
    "hq-000000000008",
    "hq-000000000009",
    "hq-000000000010",
    "hq-000000000011",
    "hq-000000000012",
    "hq-000000000013",
    "hq-000000000014",
    "mission-000000000002",
    "mission-000000000003",
    "mission-000000000005",
    "mission-000000000006",
    "mission-000000000016",
  ]);
});

test("configured day limit can expand without hiding events permanently", async ({
  page,
}, info) => {
  await page.route("**/api/calendar", async (route) => {
    const response = await route.fetch();
    const data = await response.json();
    await route.fulfill({
      response,
      json: { ...data, limits: { day: 3, week: 4, month: 2 } },
    });
  });
  await page.goto("/");
  await selectCalendarView(page, "Week");
  const cards = page.locator('[data-event-date="2026-09-08"]:visible');
  await expect(cards).toHaveCount(info.project.name === "phone" ? 4 : 14);
  await selectCalendarView(page, "Day");
  await expect(cards).toHaveCount(3);
  await page
    .getByText(/Show All/)
    .first()
    .click();
  await expect(cards).toHaveCount(14);
});

test("normal startup is empty", async ({ page }) => {
  await page.goto(process.env.CALENDAR_EMPTY_URL || "http://127.0.0.1:8090");
  await expect(
    page.getByText("No events available", { exact: true }).first(),
  ).toBeVisible();
});

test("sparse event details, touch, focus containment, and resize", async ({
  page,
}, info) => {
  await page.goto("/");
  await selectCalendarView(page, "Week");
  const event = page
    .getByTestId("event-gothic-000000000001")
    .filter({ visible: true });
  if (info.project.name === "phone") await event.tap();
  else await event.click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await expect(
    dialog.getByRole("heading", { name: "Untimed Alpha" }),
  ).toBeVisible();
  await expect(dialog.locator("dt")).toHaveText(["Venue", "Status"]);
  await expect(dialog.getByText("Scheduled", { exact: true })).toBeVisible();
  await expect(
    dialog.getByRole("link", { name: "Download Calendar Entry" }),
  ).toBeVisible();
  await expect(
    dialog.getByRole("button", { name: "Close event details" }),
  ).toBeFocused();
  await page.keyboard.press("Tab");
  expect(
    await dialog.evaluate((node) => node.contains(document.activeElement)),
  ).toBe(true);
  await page.setViewportSize({
    width: info.project.name === "phone" ? 1440 : 390,
    height: 900,
  });
  await expect(dialog).toBeVisible();
  await page.screenshot({
    path: `test-results/${info.project.name}-details.png`,
  });
  await dialog.getByRole("button", { name: "Close event details" }).click();
  await expect(dialog).not.toBeVisible();
  await expect(
    page.getByRole("button", {
      name: info.project.name === "desktop" ? "Choose calendar view" : "Week",
      exact: true,
      includeHidden: true,
    }),
  ).toBeFocused();
  await expect(page.getByTestId("range")).toContainText("Sep 6");
});

test("desktop date header opens the selected day", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto("/");
  await selectCalendarView(page, "Week");
  await page
    .locator("[data-calendar-day-header]")
    .filter({ hasText: "Tue 8" })
    .click();
  await expect(page.getByTestId("calendar")).toHaveAttribute(
    "data-view",
    "dayGridDay",
  );
  await expect(
    page.locator('[data-event-date="2026-09-08"]:visible'),
  ).toHaveCount(14);
});

test("an unavailable artifact is not reported as an empty calendar", async ({
  page,
}) => {
  await page.route("**/api/calendar", (route) =>
    route.fulfill({ status: 503, body: "Calendar data is unavailable" }),
  );
  await page.goto("/");
  await expect(page.getByRole("alert")).toHaveText(
    "Unable to load events. Please try again later.",
  );
  await expect(page.getByText("No events available")).toHaveCount(0);
});

test("calendar views, limits, ordering, details, and date navigation", async ({
  page,
}, info) => {
  const mobile = info.project.name === "phone";
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto("/");
  await selectCalendarView(page, "Week");
  const calendar = page.getByTestId("calendar");
  await expect(calendar).toHaveAttribute(
    "data-view",
    mobile ? "listWeek" : "dayGridWeek",
  );
  await expect(page.getByTestId("range")).toContainText("Sep 6");
  await expect(page.getByTestId("range")).toHaveAttribute(
    "aria-label",
    /12, 2026/,
  );
  await expect(
    page.getByTestId("event-gothic-000000000001").filter({ visible: true }),
  ).toBeVisible();
  const cards = page.locator('[data-event-date="2026-09-08"]:visible');
  await expect(cards).toHaveCount(mobile ? 10 : 14);
  await expect(cards.first()).toContainText("Untimed Alpha");
  if (!mobile) {
    const headers = page.locator("[data-calendar-day-header]");
    await expect(headers).toHaveCount(7);
    const boxes = await headers.evaluateAll((nodes) =>
      nodes.map((node) => {
        const box = node.getBoundingClientRect();
        return { x: box.x, y: box.y };
      }),
    );
    expect(new Set(boxes.map((box) => Math.round(box.y))).size).toBe(1);
    expect(new Set(boxes.map((box) => Math.round(box.x))).size).toBe(7);
  }
  await page.screenshot({
    path: `test-results/${info.project.name}-week.png`,
    fullPage: true,
  });
  if (mobile)
    await page
      .getByText(/Show All/)
      .first()
      .click();
  else
    await page
      .locator("[data-calendar-day-header]")
      .filter({ hasText: "Tue 8" })
      .click();
  await expect(calendar).toHaveAttribute(
    "data-view",
    mobile ? "listDay" : "dayGridDay",
  );
  await expect(cards).toHaveCount(14);
  await expect(cards.locator(".event-title")).toHaveText([
    "Untimed Alpha",
    "Untimed Zulu",
    "Early doors",
    "Amber",
    "Birch",
    "Cedar",
    "Dune",
    "Ember",
    "Fern",
    "Grove",
    "Hazel",
    "Iris",
    "Juniper",
    "Kestrel",
  ]);
  const event = page
    .getByTestId("event-mission-000000000003")
    .filter({ visible: true });
  await event.focus();
  await page.keyboard.press("Enter");
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await expect(dialog).toContainText("18:00 MDT");
  await expect(
    dialog.getByRole("link", { name: "View Event", exact: true }),
  ).toHaveAttribute("href", "https://example.com/event");
  await expect(
    dialog.getByRole("link", { name: "Buy Tickets", exact: true }),
  ).toHaveAttribute("href", "https://example.com/tickets");
  await page.keyboard.press("Escape");
  await expect(dialog).not.toBeVisible();
  await expect(event).toBeFocused();
  await selectCalendarView(page, "Month");
  await expect.poll(() => cards.count()).toBeGreaterThan(0);
  expect(await cards.count()).toBeLessThanOrEqual(mobile ? 5 : 14);
  await page
    .getByRole("button", { name: /Show All/ })
    .first()
    .click();
  await expect(cards).toHaveCount(14);
  await selectCalendarView(page, "Week");
  await page.getByRole("button", { name: "Previous", exact: true }).click();
  await expect(page.getByTestId("range")).toContainText("Aug 30");
  await page.getByRole("button", { name: "Next", exact: true }).click();
  await expect(cards).toHaveCount(mobile ? 10 : 14);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
  expect(errors).toEqual([]);
});

test("responsive view changes preserve the selected week", async ({ page }) => {
  await page.goto("/");
  await selectCalendarView(page, "Week");
  await page.setViewportSize({ width: 1440, height: 1000 });
  await expect(page.getByTestId("calendar")).toHaveAttribute(
    "data-view",
    "dayGridWeek",
  );
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByTestId("calendar")).toHaveAttribute(
    "data-view",
    "listWeek",
  );
  await expect(page.getByTestId("range")).toContainText("Sep 6");
});

test("catalog metadata reaches the rendered calendar", async ({
  page,
  request,
}) => {
  const response = await request.get("/api/calendar");
  expect(response.status()).toBe(200);
  const data = await response.json();
  const early = data.events.find(
    (e: { title: string }) => e.title === "Early doors",
  );
  expect(early.age_policy).toBe("16+");
  expect(early.age_policy_url).toBe("https://example.com/age-policy");
  expect(
    data.events.find((e: { title: string }) => e.title === "Untimed Zulu")
      .age_policy,
  ).toBe("21+");
  const offsite = data.events.find(
    (e: { title: string }) => e.title === "Off-site session",
  );
  expect(offsite).toMatchObject({
    venue: "Warehouse",
    off_site: true,
    timezone: "",
  });
  expect(offsite.age_policy).toBeUndefined();
  expect(
    data.events.some((e: { title: string }) =>
      /Removed session|Expired session/.test(e.title),
    ),
  ).toBe(false);
  await page.goto("/");
  await selectCalendarView(page, "Week");
  await page
    .getByTestId("event-mission-000000000003")
    .filter({ visible: true })
    .click();
  const dialog = page.getByRole("dialog");
  await expect(
    dialog.getByRole("link", { name: "Venue admission policy", exact: true }),
  ).toHaveAttribute("href", "https://example.com/age-policy");
  await dialog.getByRole("button", { name: "Close event details" }).click();
  await page
    .getByTestId("event-mission-000000000016")
    .filter({ visible: true })
    .click();
  await expect(dialog).toContainText("Off-site");
  await expect(dialog).toContainText("Warehouse");
  await expect(dialog.getByText("Age policy", { exact: true })).toHaveCount(0);
});
