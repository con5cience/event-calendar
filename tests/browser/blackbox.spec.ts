import { expect, test } from "@playwright/test";
import { openPhoneEventDay } from "./filter-controls";

test("Black Box event terms, links, exports and adult filtering reach the app", async ({
  page,
  request,
  isMobile,
}) => {
  const base = process.env.BLACKBOX_BASE_URL;
  test.skip(!base, "Requires explicit Black Box publication");
  const response = await request.get(base + "/api/calendar");
  expect(response.status()).toBe(200);
  const { events } = await response.json();
  const records = events.filter(
    (e: { venue: string }) => e.venue === "The Black Box",
  );
  expect(records.length).toBeGreaterThan(0);
  for (const event of records.slice(0, 2)) {
    expect(event.age_category).toBe("18+");
    expect(event.with_adult).toBeUndefined();
    expect(event).not.toHaveProperty("price");
    await page.goto(base + event.public_path);
    const dialog = page.getByRole("dialog");
    await expect(dialog).toContainText("The Black Box");
    await expect(dialog).toContainText("18+");
    await expect(
      dialog.getByRole("link", { name: "View Event", exact: true }),
    ).toHaveAttribute("href", event.event_url);
    await expect(
      dialog.getByRole("link", { name: "Buy Tickets", exact: true }),
    ).toHaveAttribute("href", event.ticket_url);
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
    ).toHaveCount(0);
  }
});
