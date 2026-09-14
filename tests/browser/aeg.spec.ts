import { selectCalendarView } from "./view-controls";
import { expect, test } from "@playwright/test";

test.beforeEach(async ({ page }) => {
  await page.clock.setFixedTime(new Date("2026-09-09T01:00:00Z"));
});

test("cancelled cards use only red strikethrough and popup status stays bold red", async ({
  page,
}) => {
  await page.route("**/api/calendar", async (route) => {
    const response = await route.fetch();
    const data = await response.json();
    data.events.find(
      (event: { id: string }) => event.id === "gothic-000000000001",
    ).status = "Cancelled";
    await route.fulfill({ response, json: data });
  });
  await page.goto("/");
  const card = page
    .getByTestId("event-gothic-000000000001")
    .filter({ visible: true });
  for (const view of ["Week", "Month", "Day"]) {
    await selectCalendarView(page, view);
    await expect(card).toHaveText(
      view === "Day" ? "Untimed Alpha @ Gothic Theatre" : "Untimed Alpha",
    );
    await expect(card.locator(".event-summary")).toHaveCSS(
      "text-decoration-line",
      "line-through",
    );
    await expect(card.locator(".event-summary")).toHaveCSS(
      "text-decoration-color",
      "rgb(248, 113, 113)",
    );
    await expect(card).not.toContainText("Buy Tickets");
    await expect(card.getByRole("link")).toHaveCount(0);
  }
  await page.screenshot({
    path: `test-results/cancelled-entry-${test.info().project.name}.png`,
  });
  await card.click();
  await expect(
    page.getByRole("dialog").getByText("Cancelled", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("dialog").getByText("Cancelled", { exact: true }),
  ).toHaveCSS("color", "rgb(248, 113, 113)");
  await expect(
    page.getByRole("dialog").getByText("Cancelled", { exact: true }),
  ).toHaveCSS("font-weight", "700");
  await page.screenshot({
    path: `test-results/cancelled-modal-${test.info().project.name}.png`,
  });
});

test("scheduled cards omit raw status and off-site text while details use short link labels", async ({
  page,
}) => {
  await page.route("**/api/calendar", async (route) => {
    const response = await route.fetch();
    const data = await response.json();
    const event = data.events.find(
      (event: { id: string }) => event.id === "mission-000000000003",
    );
    event.status = "Buy Tickets";
    event.off_site = true;
    await route.fulfill({ response, json: data });
  });
  await page.goto("/");
  const card = page
    .getByTestId("event-mission-000000000003")
    .filter({ visible: true });
  await selectCalendarView(page, "Week");
  for (const view of ["Week", "Month", "Day"]) {
    await selectCalendarView(page, view);
    await expect(card).toHaveText(
      view === "Day" ? "Early doors @ Mission Ballroom" : "Early doors",
    );
    await expect(card.locator(".event-summary")).toHaveCSS(
      "text-decoration-line",
      "none",
    );
  }
  await card.click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByText("Scheduled", { exact: true })).toBeVisible();
  await expect(
    dialog.getByText("Mission Ballroom", { exact: true }),
  ).toBeVisible();
  await expect(dialog.getByText("Scheduled", { exact: true })).not.toHaveCSS(
    "color",
    "rgb(248, 113, 113)",
  );
  await expect(dialog).toContainText("18:00 MDT");
  await expect(dialog.getByText("Off-site", { exact: true })).toBeVisible();
  await expect(
    dialog.getByRole("link", { name: "View Event", exact: true }),
  ).toHaveAttribute("href", "https://example.com/event");
  await expect(
    dialog.getByRole("link", { name: "Buy Tickets", exact: true }),
  ).toHaveAttribute("href", "https://example.com/tickets");
  await expect(dialog).not.toContainText("Buy Tickets");
});

test("AEG replay artifact reaches the calendar", async ({ page }) => {
  test.skip(
    !process.env.AEG_BASE_URL,
    "Requires the isolated AEG replay container workflow",
  );
  await page.goto(process.env.AEG_BASE_URL!);
  await selectCalendarView(page, "Week");
  const cards = page
    .getByRole("button")
    .filter({ hasText: "Fixture Ensemble" });
  await expect(cards).toHaveCount(5);
  await expect(cards.first()).not.toContainText("Cancelled");
  await expect(cards.first().locator(".event-summary")).toHaveCSS(
    "text-decoration-line",
    "line-through",
  );
  await cards.first().click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByText("Cancelled", { exact: true })).toBeVisible();
  await expect(dialog).toContainText("19:00 MDT");
  await expect(dialog.getByRole("link", { name: /ticket/i })).toHaveAttribute(
    "href",
    "https://example.com/tickets/1001",
  );
  await page.screenshot({
    path: `test-results/aeg-${test.info().project.name}.png`,
  });
  await page.getByRole("button", { name: "Close event details" }).click();
  await page.getByRole("checkbox", { name: "With Adult", exact: true }).check();
  await expect(cards).toHaveCount(4);
  await cards.first().click();
  await expect(dialog).toContainText("Bluebird Theater");
  await expect(dialog).toContainText("Age 14: Ticketed adult required");
});
