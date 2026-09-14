import { selectCalendarView } from "./view-controls";
import { expect, test } from "@playwright/test";

for (const [venue, category] of [
  ["HQ", "18+"],
  ["The Oriental Theater", "All ages"],
]) {
  test(`HoldMyTicket ${venue} reaches calendar and details`, async ({
    page,
    request,
  }, info) => {
    const base = process.env.HMT_BASE_URL;
    test.skip(!base, "Requires explicit HoldMyTicket publication");
    const response = await request.get(`${base}/api/calendar`);
    expect(response.status()).toBe(200);
    const { events } = await response.json();
    const event = events.find(
      (e: { venue: string; age_category?: string; doors_at?: string }) =>
        e.venue === venue && e.age_category === category && e.doors_at,
    );
    expect(event).toBeTruthy();
    await page.goto(base + event.public_path);
    const dialog = page.getByRole("dialog");
    await expect(dialog).toContainText(venue);
    await expect(dialog).toContainText(event.age_policy);
    expect(event).not.toHaveProperty("price");
    expect(event).not.toHaveProperty("cost_category");
    await expect(
      dialog.locator("dt").filter({ hasText: /^Price$/ }),
    ).toHaveCount(0);
    await expect(
      dialog.getByRole("link", { name: "Buy Tickets", exact: true }),
    ).toHaveAttribute("href", event.ticket_url);
    await expect(
      dialog.getByRole("link", { name: "View Event", exact: true }),
    ).toHaveAttribute("href", event.event_url);
    await page.screenshot({
      path: `test-results/hmt-${venue === "HQ" ? "hq" : "oriental"}-${info.project.name}.png`,
    });
    await page.getByRole("button", { name: "Close event details" }).click();
    await expect(page).toHaveURL(/\/$/);
    await selectCalendarView(page, "Day");
    // Direct URLs open the event week. Use search to locate it within that week.
    await selectCalendarView(page, "Week");
    await page
      .getByRole("searchbox", { name: "Search events" })
      .fill(event.title);
    await expect(
      page.getByTestId(`event-${event.id}`).filter({ visible: true }),
    ).toBeVisible();
  });
}
