import { expect, test } from "@playwright/test";
import { openPhoneEventDay } from "./filter-controls";

for (const [venue, category, condition] of [
  ["Bluebird Theater", "16+", "Ticketed adult required"],
  ["Gothic Theatre", "16+", "Ticketed adult required"],
  ["Mission Ballroom", "16+", "Ticketed adult required"],
  ["Ogden Theatre", "16+", "Ticketed adult required"],
  [
    "Fiddler's Green Amphitheatre",
    "All ages",
    "Ticket generally required from age 2",
  ],
]) {
  test(`published ${venue} admission reaches the adult filter and modal`, async ({
    page,
    request,
  }, info) => {
    test.skip(
      !process.env.LIVE_ADMISSION_BASE_URL,
      "Requires explicit five-venue enrichment",
    );
    const base = process.env.LIVE_ADMISSION_BASE_URL!;
    const response = await request.get(`${base}/api/calendar`);
    expect(response.status()).toBe(200);
    const { events } = await response.json();
    const event = events.find(
      (e: { venue: string; age_category?: string }) =>
        e.venue === venue && e.age_category === category,
    );
    expect(event.with_adult.ranges[1].condition).toBe(condition);
    await page.goto(base + event.public_path);
    const dialog = page.getByRole("dialog");
    await expect(dialog).toContainText(`Age 14: ${condition}`);
    await expect(dialog).toContainText(
      category === "16+" ? "16 & Over" : "All Ages",
    );
    await expect(
      dialog.getByRole("link", { name: "Venue admission policy", exact: true }),
    ).toHaveAttribute("href", event.with_adult.url);
    await page.screenshot({
      path: `test-results/live-admission-modal-${event.id}-${info.project.name}.png`,
    });
    await page.getByRole("button", { name: "Close event details" }).click();
    await expect(page).toHaveURL(/\/\?view=week&date=\d{4}-\d{2}-\d{2}$/);
    if (info.project.name === "phone")
      await openPhoneEventDay(page, event.date);
    await page
      .getByRole("checkbox", { name: "With Adult", exact: true })
      .check();
    await expect(
      page.getByRole("combobox", { name: "Child’s age", exact: true }),
    ).toHaveValue("14");
    await page.screenshot({
      path: `test-results/live-admission-filter-${event.id}-${info.project.name}.png`,
    });
    await expect(
      page.getByTestId(`event-${event.id}`).filter({ visible: true }),
    ).toBeVisible();
    const allowed = new Set(
      events
        .filter(
          (e: {
            id: string;
            with_adult?: { ranges: { min_age: number; max_age: number }[] };
          }) =>
            e.with_adult?.ranges.some(
              (r) => r.min_age <= 14 && r.max_age >= 14,
            ),
        )
        .map((e: { id: string }) => `event-${e.id}`),
    );
    const visible = await page
      .locator(".event-card[data-testid]:visible")
      .evaluateAll((nodes) => nodes.map((n) => n.getAttribute("data-testid")));
    expect(visible.every((id) => allowed.has(id))).toBe(true);
  });
}
