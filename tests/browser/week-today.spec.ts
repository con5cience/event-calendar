import { selectCalendarView } from "./view-controls";
import { expect, test } from "@playwright/test";

test("desktop Week marks today without changing other dates or navigation", async ({
  page,
}) => {
  await page.clock.setFixedTime(new Date("2026-09-09T18:00:00Z"));
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto("/");
  await selectCalendarView(page, "Week");
  const today = page.getByRole("columnheader", {
    name: "September 9, 2026",
    exact: true,
  });
  const marker = today.locator(".week-today-label");
  await expect(marker).toHaveText("Wed 9");
  await expect(marker).toHaveCSS("background-color", "rgb(108, 92, 231)");
  await expect(marker).toHaveCSS("color", "rgb(255, 255, 255)");
  await expect(marker).toHaveCSS("border-radius", "12px");
  const box = (await marker.boundingBox())!;
  expect(box.width).toBeGreaterThan(box.height);
  expect(box.height).toBe(24);
  await expect(page.locator(".week-today-cell")).toHaveCount(1);
  await expect(
    page.getByRole("gridcell", { name: "September 9, 2026", exact: true }),
  ).toHaveCSS("background-color", "rgb(21, 18, 28)");
  await expect(
    page.getByRole("gridcell", { name: "September 8, 2026", exact: true }),
  ).not.toHaveClass(/week-today-cell/);
  await page.screenshot({ path: "test-results/week-today.png" });
  await today.getByRole("button").click();
  await expect(page.getByTestId("calendar")).toHaveAttribute(
    "data-view",
    "dayGridDay",
  );
  await expect(page.locator(".week-today-label, .week-today-cell")).toHaveCount(
    0,
  );
  await selectCalendarView(page, "Week");
  await page.getByRole("button", { name: "Next", exact: true }).click();
  await expect(page.locator(".week-today-label, .week-today-cell")).toHaveCount(
    0,
  );
  await page.getByRole("button", { name: "Today", exact: true }).click();
  await expect(marker).toBeVisible();
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByTestId("calendar")).toHaveAttribute(
    "data-view",
    "listWeek",
  );
  await expect(page.locator(".week-today-label, .week-today-cell")).toHaveCount(
    0,
  );
});
