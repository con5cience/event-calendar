import { selectCalendarView } from "./view-controls";
import { expect, test } from "@playwright/test";

test("desktop Week shows more than ten when they fit and only offers Show All for overflow", async ({
  page,
}) => {
  await page.clock.setFixedTime(new Date("2026-09-09T01:00:00Z"));
  await page.setViewportSize({ width: 1440, height: 1200 });
  await page.goto("/");
  await selectCalendarView(page, "Week");
  const day = page.getByRole("gridcell", {
    name: "September 8, 2026",
    exact: true,
  });
  const cards = day.locator("[data-event-date]:visible");
  const more = day.getByRole("button", { name: "Show All", exact: true });
  await expect(cards).toHaveCount(14);
  await expect(more).toHaveCount(0);
  await page.setViewportSize({ width: 1024, height: 420 });
  await expect(more).toBeVisible();
  expect(await cards.count()).toBeLessThan(14);
  const bottom = (await day.boundingBox())!;
  const link = (await more.boundingBox())!;
  expect(link.y + link.height).toBeLessThanOrEqual(bottom.y + bottom.height);
  await more.click();
  await expect(page.getByTestId("calendar")).toHaveAttribute(
    "data-view",
    "dayGridDay",
  );
  await expect(
    page.locator('[data-event-date="2026-09-08"]:visible'),
  ).toHaveCount(14);
  await selectCalendarView(page, "Week");
  await page.setViewportSize({ width: 1440, height: 1200 });
  await expect(cards).toHaveCount(14);
  await expect(more).toHaveCount(0);
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByTestId("calendar")).toHaveAttribute(
    "data-view",
    "listWeek",
  );
  await expect(
    page.locator('[data-event-date="2026-09-08"]:visible'),
  ).toHaveCount(10);
});
