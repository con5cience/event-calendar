import { expect, test } from "@playwright/test";
import { openPhoneEventDay } from "./filter-controls";

test("Ophelia's guardian exceptions reach filters, details and export", async ({
  page,
  request,
  isMobile,
}) => {
  const base = process.env.OPHELIAS_BASE_URL;
  test.skip(!base, "Requires explicit Ophelia's publication");
  const response = await request.get(base + "/api/calendar");
  expect(response.status()).toBe(200);
  const { events } = await response.json();
  const rows = events.filter(
    (e: { venue: string }) => e.venue === "Ophelia's Electric Soapbox",
  );
  expect(rows.length).toBeGreaterThan(0);
  for (const category of ["16+", "18+", "21+"]) {
    const event = rows.find(
      (e: { age_category: string }) => e.age_category === category,
    );
    expect(event).toBeTruthy();
    expect(event).not.toHaveProperty("price");
    await page.goto(base + event.public_path);
    const dialog = page.getByRole("dialog");
    await expect(dialog).toContainText("Ophelia's Electric Soapbox");
    await expect(dialog).toContainText(category);
    await expect(
      dialog.getByRole("link", { name: "View Event", exact: true }),
    ).toHaveAttribute("href", event.event_url);
    await expect(
      dialog.getByRole("link", { name: "Buy Tickets", exact: true }),
    ).toHaveAttribute("href", event.ticket_url);
    if (category !== "21+")
      await expect(dialog).toContainText("Ticket and legal guardian required");
    else expect(event.with_adult).toBeUndefined();
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
    ).toHaveCount(category === "21+" ? 0 : 1);
    await page
      .getByRole("combobox", { name: "Child’s age" })
      .selectOption("12");
    await expect(
      page.getByTestId("event-" + event.id).filter({ visible: true }),
    ).toHaveCount(0);
  }
});
