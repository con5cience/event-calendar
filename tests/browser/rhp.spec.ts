import { expect, test } from "@playwright/test";
import { openPhoneEventDay } from "./filter-controls";

for (const venue of ["Lost Lake", "Larimer Lounge", "Globe Hall"]) {
  test(`${venue} supports details, export and adult filtering`, async ({
    page,
    request,
  }, info) => {
    const base = process.env.RHP_BASE_URL;
    test.skip(!base, "Requires explicit RHP publication");
    const response = await request.get(`${base}/api/calendar`);
    expect(response.status()).toBe(200);
    const { events } = await response.json();
    const event = events.find(
      (e: { venue: string; with_adult?: unknown }) =>
        e.venue === venue && e.with_adult,
    );
    expect(event).toBeTruthy();
    expect(event).not.toHaveProperty("price");
    expect(event.with_adult.ranges[0]).toEqual({
      min_age: 0,
      max_age: 15,
      condition: "Ticketed guardian aged 21+ required",
    });
    await page.goto(base + event.public_path);
    const dialog = page.getByRole("dialog");
    await expect(dialog).toContainText(venue);
    await expect(dialog).toContainText(
      "Age 14: Ticketed guardian aged 21+ required",
    );
    await expect(
      dialog
        .locator(".admission-clearance")
        .getByRole("link", { name: "Venue admission policy", exact: true }),
    ).toHaveAttribute("href", event.event_url);
    await expect(
      dialog.getByRole("link", { name: "Buy Tickets", exact: true }),
    ).toHaveAttribute("href", event.ticket_url);
    const exported = await request.get(`${base}/api${event.public_path}.ics`);
    expect(exported.status()).toBe(200);
    expect(await exported.text()).toContain("BEGIN:VEVENT");
    await page.screenshot({
      path: `test-results/rhp-${venue.replaceAll(" ", "-")}-${info.project.name}.png`,
    });
    await page.getByRole("button", { name: "Close event details" }).click();
    if (info.project.name === "phone")
      await openPhoneEventDay(page, event.date);
    await page
      .getByRole("checkbox", { name: "With Adult", exact: true })
      .check();
    await expect(
      page.getByTestId(`event-${event.id}`).filter({ visible: true }),
    ).toBeVisible();
    await page
      .getByRole("combobox", { name: "Child’s age" })
      .selectOption("16");
    await expect(
      page.getByTestId(`event-${event.id}`).filter({ visible: true }),
    ).toBeVisible();
  });
}
