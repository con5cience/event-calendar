import { expect, test } from "@playwright/test";
import { openPhoneEventDay } from "./filter-controls";

test("Seventh Circle publishes reviewed admission without inferred clocks or tickets", async ({
  page,
  request,
  isMobile,
}) => {
  const base = process.env.SEVENTH_CIRCLE_BASE_URL;
  test.skip(!base, "Requires explicit Seventh Circle publication");
  const response = await request.get(base + "/api/calendar");
  expect(response.status()).toBe(200);
  const { events } = await response.json();
  const records = events.filter(
    (e: { venue: string }) => e.venue === "Seventh Circle Music Collective",
  );
  expect(records.length).toBeGreaterThan(0);
  const event = records.find((e: { doors_at?: string }) => !e.doors_at);
  expect(event).toBeTruthy();
  for (const e of records) {
    expect(e).not.toHaveProperty("price");
    expect(e).not.toHaveProperty("ticket_url");
    expect(e).not.toHaveProperty("show_at");
    expect(e.age_category).toBe("All ages");
    expect(e.with_adult.ranges[0].condition).toBe(
      "$5 annual fee; show donations encouraged",
    );
  }
  await page.goto(base + event.public_path);
  const dialog = page.getByRole("dialog");
  await expect(dialog).toContainText(
    "$5 annual fee; show donations encouraged",
  );
  await expect(dialog).not.toContainText("membership");
  await expect(
    dialog.getByRole("link", { name: "Buy Tickets", exact: true }),
  ).toHaveCount(0);
  await expect(
    dialog.getByRole("link", { name: "View Event", exact: true }),
  ).toHaveAttribute("href", event.event_url);
  await expect(
    dialog
      .locator(".admission-clearance")
      .getByRole("link", { name: "Venue admission policy", exact: true }),
  ).toHaveAttribute(
    "href",
    "https://www.7thcirclemusiccollective.org/home/about",
  );
  const ics = await request.get(base + "/api" + event.public_path + ".ics");
  expect(ics.status()).toBe(200);
  expect(await ics.text()).toContain("BEGIN:VEVENT");
  await page.getByRole("button", { name: "Close event details" }).click();
  if (isMobile) await openPhoneEventDay(page, event.date);
  await page.getByRole("checkbox", { name: "With Adult", exact: true }).check();
  await page.getByRole("combobox", { name: "Child’s age" }).selectOption("14");
  await expect(
    page.getByTestId("event-" + event.id).filter({ visible: true }),
  ).toHaveCount(1);
  const doors = records.find((e: { doors_at?: string }) => e.doors_at);
  expect(doors).toBeTruthy();
  expect(doors.title).not.toMatch(/6pm doors/i);
});
