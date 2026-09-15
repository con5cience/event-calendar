import { expect, test } from "@playwright/test";

test("Federal publication supports details and With Adult", async ({
  page,
  request,
}, info) => {
  const base = process.env.FEDERAL_BASE_URL;
  test.skip(!base, "Requires explicit Federal publication");
  const openDay = async (date: string) => {
    if (info.project.name === "phone") {
      await page.getByTestId(`date-${date}`).click();
    } else {
      const value = new Date(`${date}T12:00:00Z`);
      const weekday = new Intl.DateTimeFormat("en-US", {
        weekday: "short",
        timeZone: "UTC",
      }).format(value);
      await page
        .locator("[data-calendar-day-header]")
        .filter({
          hasText: `${weekday} ${value.getUTCDate()}`,
        })
        .click();
    }
  };
  const response = await request.get(`${base}/api/calendar`);
  expect(response.status()).toBe(200);
  const { events } = await response.json();
  const federal = events.filter(
    (e: { venue: string }) => e.venue === "The Federal Theatre",
  );
  const allowed = federal.find(
    (e: { age_category?: string }) => e.age_category === "All ages",
  );
  const restricted = federal.find(
    (e: { age_category?: string }) => e.age_category === "18+",
  );
  expect(allowed).toBeTruthy();
  expect(restricted).toBeTruthy();
  expect(restricted.with_adult).toBeUndefined();
  expect(allowed.with_adult.url).toBe(allowed.event_url);
  expect(allowed.with_adult.ranges).toEqual([
    {
      min_age: 0,
      max_age: 17,
      condition:
        "All ages event; attend with adult and follow event conditions",
    },
  ]);
  await page.goto(base + allowed.public_path);
  const dialog = page.getByRole("dialog");
  await expect(dialog).toContainText("The Federal Theatre");
  await expect(dialog).toContainText("Age 14: All ages event");
  expect(allowed).not.toHaveProperty("price");
  await expect(dialog.locator("dt").filter({ hasText: /^Price$/ })).toHaveCount(
    0,
  );
  for (const [name, href] of [
    ["Buy Tickets", allowed.ticket_url],
    ["View Event", allowed.event_url],
    ["Venue admission policy", allowed.event_url],
  ]) {
    await expect(
      dialog.getByRole("link", { name, exact: true }),
    ).toHaveAttribute("href", href);
  }
  await page.screenshot({
    path: `test-results/federal-${info.project.name}.png`,
  });
  await page.getByRole("button", { name: "Close event details" }).click();
  await openDay(allowed.date);
  await page.getByRole("checkbox", { name: "With Adult", exact: true }).check();
  await expect(
    page.getByTestId(`event-${allowed.id}`).filter({ visible: true }),
  ).toBeVisible();
  // Direct event URLs reset filters. Re-enable the filter in the restricted week.
  await page.goto(base + restricted.public_path);
  await page.getByRole("button", { name: "Close event details" }).click();
  // Desktop grids cap week cards to the available cell height. Test age
  // filtering in the event's full day, not its capped week cards.
  await openDay(restricted.date);
  await expect(
    page.getByTestId(`event-${restricted.id}`).filter({ visible: true }),
  ).toBeVisible();
  await page.getByRole("checkbox", { name: "With Adult", exact: true }).check();
  await expect(
    page.getByTestId(`event-${restricted.id}`).filter({ visible: true }),
  ).toHaveCount(0);
});
