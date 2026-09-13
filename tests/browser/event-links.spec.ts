import { expect, test } from "@playwright/test";
import { readFile } from "node:fs/promises";

const early = "/events/mission/2026-09-08-early-doors-000000000003";
const untimed = "/events/gothic/2026-09-08-untimed-alpha-000000000001";
const removed = "/events/gothic/2026-09-08-removed-session-000000000017";
const expired = "/events/gothic/2000-01-01-expired-session-000000000018";

test.beforeEach(async ({ page }) => {
  await page.clock.setFixedTime(new Date("2026-09-09T01:00:00Z"));
});

test("modal heading combines the purple date and white title without overflow", async ({
  page,
}) => {
  await page.goto(early);
  const heading = page.locator("#event-title");
  await expect(
    page
      .getByRole("dialog")
      .locator("dt")
      .filter({ hasText: /^Age Policy$/ }),
  ).toBeVisible();
  await expect(heading).toHaveText("2026-09-08 : Early doors");
  const date = heading.locator("time");
  await expect(date).toHaveAttribute("datetime", "2026-09-08");
  await expect(date).toHaveCSS("font-size", "25.6px");
  const layout = await heading.evaluate((el) => {
    const date = el.querySelector("time")!;
    const title = el.querySelector(".event-heading-title")!;
    return {
      dateColor: getComputedStyle(date).color,
      titleColor: getComputedStyle(title).color,
      overflow: el.scrollWidth > el.clientWidth,
      titleSize: getComputedStyle(title).fontSize,
    };
  });
  expect(layout.dateColor).not.toBe(layout.titleColor);
  expect(layout.titleSize).toBe("25.6px");
  expect(layout.overflow).toBe(false);
  await page.screenshot({
    path: `test-results/modal-heading-${test.info().project.name}.png`,
  });
});

test("modal actions have labeled, keyboard-accessible Font Awesome buttons", async ({
  page,
}, info) => {
  await page.goto(early);
  const dialog = page.getByRole("dialog");
  for (const [role, name, icon] of [
    ["button", "Copy Event Link", "copy"],
    ["link", "Download Calendar Entry", "download"],
    ["link", "View Event", "arrow-up-right-from-square"],
    ["link", "Buy Tickets", "ticket"],
  ] as const) {
    const action = dialog.getByRole(role, { name, exact: true });
    await expect(action).toHaveAccessibleName(name);
    await expect(action).toHaveAttribute("aria-label", name);
    await expect(action).not.toHaveAttribute("title");
    expect(await action.innerText()).toBe("");
    await expect(action).toBeVisible();
    const svg = action.locator("svg");
    await expect(svg).toHaveAttribute("data-icon", icon);
    await expect(svg).toHaveAttribute("aria-hidden", "true");
    await expect(svg).toHaveAttribute("focusable", "false");
    await action.focus();
    await expect(action).toBeFocused();
    await expect(dialog.getByRole("tooltip")).toHaveText(name);
    const style = await action.evaluate((el) => {
      const css = getComputedStyle(el);
      return {
        height: el.getBoundingClientRect().height,
        width: el.getBoundingClientRect().width,
        border: css.borderTopWidth,
        background: css.backgroundColor,
        outline: css.outlineStyle,
      };
    });
    expect(style.height).toBeGreaterThanOrEqual(44);
    expect(style.width).toBeGreaterThanOrEqual(44);
    expect(style.border).toBe("1px");
    expect(style.background).toBe("rgb(26, 26, 26)");
    expect(style.outline).toBe("solid");
  }
  await page.screenshot({
    path: `test-results/modal-actions-${info.project.name}.png`,
  });
});

test("modal action slots are equal and tooltips use a 300ms hover delay", async ({
  page,
}, info) => {
  await page.clock.install({ time: new Date("2026-09-09T01:00:00Z") });
  await page.goto(early);
  const dialog = page.getByRole("dialog");
  const slots = dialog.locator(".event-action-slot");
  await expect(slots).toHaveCount(4);
  const boxes = await slots.evaluateAll((nodes) =>
    nodes.map((node) => {
      const r = node.getBoundingClientRect();
      return { x: r.x, width: r.width, y: r.y };
    }),
  );
  expect(
    Math.max(...boxes.map((b) => b.width)) -
      Math.min(...boxes.map((b) => b.width)),
  ).toBeLessThan(1);
  const steps = boxes.slice(1).map((b, i) => b.x - boxes[i].x);
  expect(Math.max(...steps) - Math.min(...steps)).toBeLessThan(1);
  expect(new Set(boxes.map((b) => b.y)).size).toBe(1);
  const copy = dialog.getByRole("button", {
    name: "Copy Event Link",
    exact: true,
  });
  if (info.project.name === "desktop") {
    await page.clock.pauseAt(new Date("2026-09-09T01:01:00Z"));
    await copy.hover();
    await page.clock.runFor(299);
    await expect(dialog.getByRole("tooltip")).toHaveCount(0);
    await page.clock.runFor(1);
    await expect(dialog.getByRole("tooltip")).toHaveText("Copy Event Link");
    await page.keyboard.press("Escape");
    await expect(dialog.getByRole("tooltip")).toHaveCount(0);
    await expect(dialog).toBeVisible();
    await page.mouse.move(0, 0);
  }
  await copy.focus();
  await expect(dialog.getByRole("tooltip")).toHaveText("Copy Event Link");
  await dialog.getByRole("link", { name: "Buy Tickets", exact: true }).focus();
  await expect(dialog.getByRole("tooltip")).toHaveCount(1);
  await expect(dialog.getByRole("tooltip")).toHaveText("Buy Tickets");
  await page.keyboard.press("Escape");
  await expect(dialog.getByRole("tooltip")).toHaveCount(0);
  await expect(dialog).toBeVisible();
  // Without an event URL, only the export action remains.
  await page.goto(untimed);
  await expect(slots).toHaveCount(1);
  await expect(
    dialog.getByRole("button", { name: "Copy Event Link", exact: true }),
  ).toHaveCount(0);
  await expect(
    dialog.getByRole("link", { name: "Download Calendar Entry", exact: true }),
  ).toBeVisible();
});

test("background dismissal preserves inside interaction, focus, and calendar context", async ({
  page,
}, info) => {
  await page.goto("/");
  await page.getByRole("button", { name: "Month", exact: true }).click();
  const card = page
    .getByTestId("event-gothic-000000000001")
    .filter({ visible: true });
  await card.click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await dialog.getByRole("heading", { name: "Untimed Alpha" }).click();
  await expect(dialog).toBeVisible();
  const box = await dialog.boundingBox();
  if (!box) throw new Error("Missing dialog bounds");
  const outside = { x: 2, y: 2 };
  expect(box.x > outside.x || box.y > outside.y).toBe(true);
  await page.mouse.move(box.x + 5, box.y + 5);
  await page.mouse.down();
  await page.mouse.move(outside.x, outside.y);
  await page.mouse.up();
  await expect(dialog).toBeVisible();
  if (info.project.name === "phone")
    await page.touchscreen.tap(outside.x, outside.y);
  else await page.mouse.click(outside.x, outside.y);
  await expect(dialog).not.toBeVisible();
  await expect(page).toHaveURL(/\/$/);
  await expect(card).toBeFocused();
  await expect(page.getByTestId("range")).toHaveText("September 2026");
  await page.goto(early);
  await expect(dialog).toBeVisible();
  await page.mouse.click(outside.x, outside.y);
  await expect(dialog).not.toBeVisible();
  await expect(page).toHaveURL(/\/$/);
  await expect(
    page.getByRole("button", { name: "Week", exact: true }),
  ).toBeFocused();
  await expect(page.getByTestId("range")).toContainText("Sep 6");
});

test("card links support Back, Forward, close, reload, and Copy Event Link", async ({
  page,
}) => {
  await page.addInitScript(() => {
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: {
        writeText: async (text: string) => {
          sessionStorage.setItem("copied-link", text);
        },
      },
    });
  });
  await page.goto("/");
  const card = page
    .getByTestId("event-mission-000000000003")
    .filter({ visible: true });
  await page.getByRole("button", { name: "Week", exact: true }).click();
  await card.click();
  await expect(page).toHaveURL(new RegExp(early + "$"));
  const eventUrl = await page
    .getByRole("link", { name: "View Event", exact: true })
    .getAttribute("href");
  expect(eventUrl).toBeTruthy();
  expect(eventUrl).not.toBe(page.url());
  await page
    .getByRole("button", { name: "Copy Event Link", exact: true })
    .click();
  await expect(page.getByRole("status")).toContainText("Event link copied");
  expect(await page.evaluate(() => sessionStorage.getItem("copied-link"))).toBe(
    eventUrl,
  );
  await page.goBack();
  await expect(page.getByRole("dialog")).not.toBeVisible();
  await page.goForward();
  await expect(page.getByRole("dialog")).toContainText("Early doors");
  await page.keyboard.press("Escape");
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByTestId("range")).toContainText("Sep 6");
  await card.click();
  const response = await page.reload();
  expect(response?.status()).toBe(200);
  await expect(page.getByRole("dialog")).toContainText("Early doors");
  await page.getByRole("button", { name: "Close event details" }).click();
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByTestId("range")).toContainText("Sep 6");
});

test("direct links use temporary defaults, open the event week, and preserve saved preferences", async ({
  page,
}, info) => {
  const saved = {
    venues: ["HQ"],
    ages: ["21+"],
    costs: ["$$$$"],
    query: "absent",
    includePast: true,
  };
  await page.addInitScript(
    (value) =>
      localStorage.setItem("event-calendar.filters.v1", JSON.stringify(value)),
    saved,
  );
  const response = await page.goto(early);
  expect(response?.status()).toBe(200);
  await expect(page.getByRole("dialog")).toContainText("Early doors");
  await expect(page.getByTestId("calendar")).toHaveAttribute(
    "data-view",
    info.project.name === "phone" ? "listWeek" : "dayGridWeek",
  );
  await page.getByRole("button", { name: "Close event details" }).click();
  await expect(page.locator('input[type="search"]')).toHaveValue("");
  await expect(
    page.getByRole("checkbox", { name: "Include past events" }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "Venues: All venues", exact: true }),
  ).toBeVisible();
  expect(
    await page.evaluate(() =>
      JSON.parse(localStorage.getItem("event-calendar.filters.v1")!),
    ),
  ).toEqual(saved);
  await expect(page.getByTestId("range")).toContainText("Sep 6");
});

test("retained links open without listing removed events; unknown and expired links return 404", async ({
  page,
  request,
}) => {
  const response = await page.goto(removed);
  expect(response?.status()).toBe(200);
  await expect(page.getByRole("dialog")).toContainText("No longer listed");
  await expect(page.getByTestId("event-gothic-000000000017")).toHaveCount(0);
  for (const path of [expired, "/events/gothic/unknown"]) {
    for (const route of [path, "/api" + path, path + ".ics"]) {
      expect((await request.get(route)).status()).toBe(404);
    }
  }
});

test("single-event downloads preserve doors or date-only time without inventing an end", async ({
  page,
}) => {
  for (const [path, start] of [
    [early, "DTSTART:20260909T000000Z"],
    [untimed, "DTSTART;VALUE=DATE:20260908"],
  ]) {
    await page.goto(path);
    const downloadPromise = page.waitForEvent("download");
    await page
      .getByRole("link", { name: "Download Calendar Entry", exact: true })
      .click();
    const download = await downloadPromise;
    expect(download.suggestedFilename()).toMatch(/\.ics$/);
    const bytes = await readFile((await download.path())!);
    const text = bytes.toString("utf8");
    expect(text).toContain(start + "\r\n");
    expect(text.match(/BEGIN:VEVENT/g)).toHaveLength(1);
    expect(text).not.toContain("DTEND");
    expect(text.replaceAll("\r\n", "")).not.toContain("\n");
    if (path === untimed) expect(text).toContain("Time not provided");
    await expect(page.getByRole("dialog")).toBeVisible();
  }
});

test("copy denial gives a usable fallback and detail fetch failures are not missing events", async ({
  page,
}) => {
  await page.addInitScript(() =>
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: {
        writeText: async () => {
          throw new Error("denied");
        },
      },
    }),
  );
  await page.goto(early);
  const eventUrl = await page
    .getByRole("link", { name: "View Event", exact: true })
    .getAttribute("href");
  await page
    .getByRole("button", { name: "Copy Event Link", exact: true })
    .click();
  await expect(page.getByRole("status")).toContainText(
    `Copy this event link: ${eventUrl}`,
  );
  await page.route("**/api/events/**", (route) =>
    route.fulfill({ status: 503 }),
  );
  await page.reload();
  await expect(page.getByRole("alert")).toContainText(
    "Unable to load event details",
  );
  await expect(page.getByText("Event not found", { exact: true })).toHaveCount(
    0,
  );
});
