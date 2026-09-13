import { expect, test } from "@playwright/test";
import { openPhoneEventDay } from "./filter-controls";

test("Hi-Dive publishes known and unknown restrictions with guardian clearance", async ({
  page,
  request,
  isMobile,
}) => {
  const base = process.env.PLOT_BASE_URL;
  test.skip(!base, "Requires explicit Hi-Dive publication");
  const response = await request.get(`${base}/api/calendar`);
  expect(response.status()).toBe(200);
  const { events } = await response.json();
  const records = events.filter(
    (e: { venue: string }) => e.venue === "Hi-Dive",
  );
  expect(records.length).toBeGreaterThan(0);
  for (const known of [true, false]) {
    const event = records.find((e: { age_category?: string }) =>
      known ? e.age_category === "21+" : !e.age_category,
    );
    expect(event).toBeTruthy();
    expect(event).not.toHaveProperty("price");
    expect(event.with_adult.ranges).toEqual([
      {
        min_age: 0,
        max_age: 17,
        condition: "Parent or legal guardian required",
      },
    ]);
    await page.goto(base + event.public_path);
    const dialog = page.getByRole("dialog");
    await expect(dialog).toContainText("Hi-Dive");
    await expect(dialog).toContainText(
      "Age 14: Parent or legal guardian required",
    );
    await expect(
      dialog
        .locator(".admission-clearance")
        .getByRole("link", { name: "Venue admission policy", exact: true }),
    ).toHaveAttribute("href", "https://hi-dive.com/faqs/");
    await expect(
      dialog.getByRole("link", { name: "View Event", exact: true }),
    ).toHaveAttribute("href", event.event_url);
    if (event.ticket_url)
      await expect(
        dialog.getByRole("link", { name: "Buy Tickets", exact: true }),
      ).toHaveAttribute("href", event.ticket_url);
    const ics = await request.get(`${base}/api${event.public_path}.ics`);
    expect(ics.status()).toBe(200);
    expect(await ics.text()).toContain("BEGIN:VEVENT");
    await page.getByRole("button", { name: "Close event details" }).click();
    if (isMobile) await openPhoneEventDay(page, event.date);
    await page
      .getByRole("checkbox", { name: "With Adult", exact: true })
      .check();
    await expect(
      page.getByTestId(`event-${event.id}`).filter({ visible: true }),
    ).toBeVisible();
    for (const age of [0, 17]) {
      await page
        .getByRole("combobox", { name: "Child’s age" })
        .selectOption(String(age));
      await expect(
        page.getByTestId(`event-${event.id}`).filter({ visible: true }),
      ).toBeVisible();
    }
    await page
      .getByRole("combobox", { name: "Child’s age" })
      .selectOption("14");
  }
});
