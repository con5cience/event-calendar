import { selectCalendarView } from "./view-controls";
import { expect, test } from "@playwright/test";

test.beforeEach(async ({ page }) => {
  await page.clock.setFixedTime(new Date("2026-09-09T01:00:00Z"));
});

test("proportional gutters and desktop grids follow the window", async ({
  page,
}) => {
  await page.goto("/");
  for (const size of [
    { width: 2560, height: 1440 },
    { width: 1440, height: 900 },
    { width: 1024, height: 768 },
  ]) {
    await page.setViewportSize(size);
    for (const view of ["Month", "Week"]) {
      await selectCalendarView(page, view);
      const calendar = page.getByTestId("calendar");
      await expect
        .poll(async () => Math.round((await calendar.boundingBox())!.x))
        .toBe(Math.round(size.width * 0.05));
      await expect
        .poll(async () => Math.round((await calendar.boundingBox())!.width))
        .toBe(Math.round(size.width * 0.9));
      await expect
        .poll(async () => (await calendar.boundingBox())!.height)
        .toBeGreaterThan(size.height - 216); // Includes the 36px header.
      await expect
        .poll(
          async () =>
            (await calendar.boundingBox())!.y +
            (await calendar.boundingBox())!.height,
        )
        .toBeLessThanOrEqual(size.height);
    }
  }
});

test("short and narrow windows preserve reachable controls and state", async ({
  page,
}) => {
  await page.goto("/");
  const search = page.getByRole("searchbox");
  await search.fill("mission");
  for (const size of [
    { width: 2560, height: 1440 },
    { width: 844, height: 390 },
    { width: 320, height: 568 },
  ]) {
    await page.setViewportSize(size);
    await expect(search).toBeFocused();
    await expect(search).toHaveValue("mission");
    const venues = page.getByRole("button", { name: /^Venues:/ });
    await venues.scrollIntoViewIfNeeded();
    await expect(venues).toBeInViewport();
    await expect(
      page.getByRole("checkbox", { name: "With Adult", exact: true }),
    ).toBeVisible();
    await search.focus();
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth),
    ).toBeLessThanOrEqual(size.width);
  }
  await selectCalendarView(page, "Week");
  await expect(page.getByTestId("range")).toHaveAttribute(
    "aria-label",
    "Sep 6 – 12, 2026",
  );
  const range = await page.getByTestId("range").getAttribute("aria-label");
  await page.setViewportSize({ width: 1024, height: 768 });
  await expect(page.getByTestId("range")).toHaveAttribute("aria-label", range!);
  await expect(page.getByTestId("calendar")).toHaveAttribute(
    "data-view",
    "dayGridWeek",
  );
  await page
    .getByTestId("event-mission-000000000002")
    .filter({ visible: true })
    .click();
  const path = new URL(page.url()).pathname;
  await page.setViewportSize({ width: 320, height: 568 });
  await expect(page.getByRole("dialog")).toBeVisible();
  expect(new URL(page.url()).pathname).toBe(path);
  await expect(
    page.getByRole("button", { name: "Close event details" }),
  ).toBeFocused();
});

test("short month can scroll instead of compressing its grid", async ({
  page,
}) => {
  await page.setViewportSize({ width: 844, height: 390 });
  await page.goto("/");
  await selectCalendarView(page, "Month");
  await page.getByRole("button", { name: "Previous", exact: true }).click();
  await expect(page.getByTestId("range")).toHaveAttribute(
    "aria-label",
    "August 2026",
  );
  await expect
    .poll(
      async () => (await page.getByTestId("calendar").boundingBox())!.height,
    )
    .toBeGreaterThanOrEqual(576);
  const last = page.getByRole("gridcell", {
    name: "August 31, 2026",
    exact: true,
  });
  await last.scrollIntoViewIfNeeded();
  await expect(last).toBeInViewport();
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(
    844,
  );
});

test("200 percent text and zoom-equivalent reflow keep controls usable", async ({
  page,
}) => {
  // A 1280x900 window at 200% browser zoom has a 640x450 CSS viewport.
  // Check that reflow separately from doubling the user's default text size.
  await page.setViewportSize({ width: 640, height: 450 });
  await page.goto("/");
  for (const size of ["100%", "200%"]) {
    await page.evaluate((value) => {
      document.documentElement.style.fontSize = value;
    }, size);
    await page
      .getByRole("button", { name: "Venues: All venues", exact: true })
      .click();
    await page
      .getByRole("checkbox", { name: "Mission Ballroom", exact: true })
      .check();
    await page.keyboard.press("Escape");
    const venues = page.getByRole("button", { name: /^Venues:/ });
    await venues.scrollIntoViewIfNeeded();
    await expect(venues).toBeInViewport();
    expect((await venues.boundingBox())!.height).toBeGreaterThanOrEqual(44);
    await venues.click();
    await page.getByRole("checkbox", { name: "All", exact: true }).check();
    await page.keyboard.press("Escape");
    await expect(
      page.getByRole("button", { name: "Venues: All venues", exact: true }),
    ).toBeVisible();
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth),
    ).toBe(640);
    await selectCalendarView(page, "Day");
    await selectCalendarView(page, "Month");
  }
});
