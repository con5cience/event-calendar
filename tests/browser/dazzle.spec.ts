import { expect, test } from "@playwright/test";
import { openPhoneEventDay } from "./filter-controls";

test("Dazzle admission, optional tickets and exports reach the app", async ({
  page,
  request,
  isMobile,
}) => {
  const base = process.env.DAZZLE_BASE_URL;
  test.skip(!base, "Requires explicit Dazzle publication");
  const response = await request.get(base + "/api/calendar");
  expect(response.status()).toBe(200);
  const { events } = await response.json();
  const records = events.filter(
    (e: { venue: string }) => e.venue === "Dazzle @ The Arts Complex",
  );
  expect(records.length).toBeGreaterThan(0);
  for (const category of ["All ages", "21+"]) {
    const event = records.find(
      (e: { age_category: string; with_adult?: unknown }) =>
        e.age_category === category &&
        (category !== "All ages" || e.with_adult),
    );
    expect(event).toBeTruthy();
    expect(event).not.toHaveProperty("price");
    await page.goto(base + event.public_path);
    const dialog = page.getByRole("dialog");
    await expect(dialog).toContainText("Dazzle @ The Arts Complex");
    await expect(
      dialog.getByRole("link", { name: "View Event", exact: true }),
    ).toHaveAttribute("href", event.event_url);
    if (event.ticket_url)
      await expect(
        dialog.getByRole("link", { name: "Buy Tickets", exact: true }),
      ).toHaveAttribute("href", event.ticket_url);
    if (category === "All ages") {
      await expect(dialog).toContainText("Under 21 must leave by 11 PM");
      await expect(
        dialog.locator(".admission-clearance").getByRole("link", {
          name: "Venue admission policy",
          exact: true,
        }),
      ).toHaveAttribute("href", "https://www.dazzledenver.com/faq/");
    } else expect(event.with_adult).toBeUndefined();
    const ics = await request.get(base + "/api" + event.public_path + ".ics");
    expect(ics.status()).toBe(200);
    expect(await ics.text()).toContain("BEGIN:VEVENT");
    await page.getByRole("button", { name: "Close event details" }).click();
    if (isMobile) await openPhoneEventDay(page, event.date);
    await page
      .getByRole("checkbox", { name: "With Adult", exact: true })
      .check();
    await page
      .getByRole("combobox", { name: "Child’s age" })
      .selectOption("14");
    await expect(
      page.getByTestId("event-" + event.id).filter({ visible: true }),
    ).toHaveCount(category === "All ages" ? 1 : 0);
  }
  const unticketed = records.find(
    (e: { ticket_url?: string }) => !e.ticket_url,
  );
  expect(unticketed).toBeTruthy();
  await page.goto(base + unticketed.public_path);
  await expect(
    page
      .getByRole("dialog")
      .getByRole("link", { name: "Buy Tickets", exact: true }),
  ).toHaveCount(0);
});
