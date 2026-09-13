import { expect, test } from "@playwright/test";
import { openPhoneEventDay } from "./filter-controls";

test("Meow Wolf admission, cancellation, links and downloads reach the app", async ({
  page,
  request,
  isMobile,
}) => {
  const base = process.env.MEOWWOLF_BASE_URL;
  test.skip(!base, "Requires explicit Meow Wolf publication");
  const response = await request.get(base + "/api/calendar");
  expect(response.status()).toBe(200);
  const { events } = await response.json();
  const rows = events.filter(
    (e: { venue: string }) => e.venue === "Meow Wolf Denver",
  );
  expect(rows.length).toBeGreaterThan(0);
  for (const category of ["All ages", "18+"]) {
    const event = rows.find(
      (e: { age_category: string; status: string }) =>
        e.age_category === category && e.status === "Scheduled",
    );
    expect(event).toBeTruthy();
    expect(event).not.toHaveProperty("price");
    await page.goto(base + event.public_path);
    const dialog = page.getByRole("dialog");
    await expect(dialog).toContainText("Meow Wolf Denver");
    await expect(
      dialog.getByRole("link", { name: "View Event", exact: true }),
    ).toHaveAttribute("href", event.event_url);
    await expect(
      dialog.getByRole("link", { name: "Buy Tickets", exact: true }),
    ).toHaveAttribute("href", event.ticket_url);
    if (category === "All ages")
      await expect(dialog).toContainText("Ticketed guardian over 18 required");
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
    ).toHaveCount(category === "All ages" ? 1 : 0);
  }
  const cancelled = rows.find(
    (e: { status: string }) => e.status === "Cancelled",
  );
  expect(cancelled).toBeTruthy();
  await page.goto(base + cancelled.public_path);
  await expect(page.getByRole("dialog")).toContainText("Cancelled");
  await expect(
    page
      .getByRole("dialog")
      .getByRole("link", { name: "Buy Tickets", exact: true }),
  ).toHaveCount(0);
  const ics = await request.get(base + "/api" + cancelled.public_path + ".ics");
  expect(await ics.text()).toContain("STATUS:CANCELLED");
});
