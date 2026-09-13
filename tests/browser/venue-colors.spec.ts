import { expect, test } from "@playwright/test";

test("venue accents agree across cards, picker and details and survive filtering", async ({
  page,
}, info) => {
  await page.clock.setFixedTime(new Date("2026-09-09T01:00:00Z"));
  await page.goto("/events/mission/2026-09-08-early-doors-000000000003");
  const marker = page.getByRole("dialog").locator(".venue-marker");
  await expect(marker).toHaveCSS("background-color", "rgb(86, 240, 229)");
  await expect(marker).toHaveAttribute("aria-hidden", "true");
  await page.screenshot({
    path: `test-results/venue-modal-${info.project.name}.png`,
  });
  await page.getByRole("button", { name: "Close event details" }).click();
  for (const view of ["Day", "Week", "Month"]) {
    await page.getByRole("button", { name: view, exact: true }).click();
    await expect(
      page.getByTestId("event-mission-000000000003").filter({ visible: true }),
    ).toHaveCSS("border-left-color", "rgb(86, 240, 229)");
  }
  await page.getByRole("button", { name: "Venues: All venues" }).click();
  const option = page
    .locator(".filter-options label")
    .filter({ hasText: "Mission Ballroom" });
  await expect(option.locator(".venue-marker")).toHaveCSS(
    "background-color",
    "rgb(86, 240, 229)",
  );
  await option.locator(".venue-marker").click();
  await expect(option.getByRole("checkbox")).toBeChecked();
  await page.screenshot({
    path: `test-results/venue-picker-${info.project.name}.png`,
  });
  await page.getByRole("button", { name: "Venues: Mission Ballroom" }).click();
  await page.reload();
  await expect(
    page.getByTestId("event-mission-000000000003").filter({ visible: true }),
  ).toHaveCSS("border-left-color", "rgb(86, 240, 229)");
  await page.screenshot({
    path: `test-results/venue-colors-${info.project.name}.png`,
  });
});
