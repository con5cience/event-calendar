import { expect, test } from "@playwright/test";
import { openPhoneEventDay } from "./filter-controls";

test("Nocturne separates sets and displays conditional admission", async ({
  page,
  request,
  isMobile,
}) => {
  const base = process.env.NOCTURNE_BASE_URL;
  test.skip(!base, "Requires explicit Nocturne publication");
  const response = await request.get(base + "/api/calendar");
  expect(response.status()).toBe(200);
  const { events } = await response.json();
  const records = events.filter(
    (e: { venue: string }) => e.venue === "Nocturne",
  );
  expect(records.length).toBeGreaterThan(0);
  const event = records.find((e: { date: string }) => e.date === "2026-09-16");
  expect(event).toBeTruthy();
  const pair = records.filter(
    (e: { event_url: string; date: string }) =>
      e.event_url === event.event_url && e.date === event.date,
  );
  expect(pair).toHaveLength(2);
  expect(pair[0].show_at).not.toBe(pair[1].show_at);
  for (const e of records) {
    expect(e).not.toHaveProperty("price");
    expect(e).not.toHaveProperty("doors_at");
    expect(e.age_category).toBe("21+");
    expect(e.with_adult.ranges[0]).toMatchObject({ min_age: 10, max_age: 17 });
  }
  await page.goto(base + event.public_path);
  const dialog = page.getByRole("dialog");
  await expect(dialog).toContainText(
    "Parent/guardian and table reservation required; confirm admission with venue.",
  );
  await expect(
    dialog.getByRole("link", { name: "View Event", exact: true }),
  ).toHaveAttribute("href", event.event_url);
  await expect(
    dialog
      .locator(".admission-clearance")
      .getByRole("link", { name: "Venue admission policy", exact: true }),
  ).toHaveAttribute("href", "https://nocturnejazz.com/faq");
  const ics = await request.get(base + "/api" + event.public_path + ".ics");
  expect(ics.status()).toBe(200);
  expect(await ics.text()).toContain("BEGIN:VEVENT");
  await page.getByRole("button", { name: "Close event details" }).click();
  if (isMobile) await openPhoneEventDay(page, event.date);
  await page.getByRole("checkbox", { name: "With Adult", exact: true }).check();
  const age = page.getByRole("combobox", { name: "Child’s age" });
  for (const value of ["10", "14", "17"]) {
    await age.selectOption(value);
    await expect(
      page.getByTestId("event-" + event.id).filter({ visible: true }),
    ).toHaveCount(1);
  }
  await age.selectOption("9");
  await expect(
    page.getByTestId("event-" + event.id).filter({ visible: true }),
  ).toHaveCount(0);
});
