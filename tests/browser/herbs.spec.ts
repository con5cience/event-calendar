import { expect, test } from "@playwright/test";
import { openEventDay } from "./filter-controls";

test("Herb's printed Denver times and parent cutoff reach the app", async ({
  page,
  request,
  isMobile,
}) => {
  const base = process.env.HERBS_BASE_URL;
  test.skip(!base, "Requires explicit Herb's publication");
  const response = await request.get(base + "/api/calendar");
  expect(response.status()).toBe(200);
  const { events } = await response.json();
  const event = events.find(
    (e: { venue: string; title: string; date: string }) =>
      e.venue === "Herb's" &&
      e.title === "Mike Maurer Band" &&
      e.date === "2026-09-12",
  );
  expect(event).toBeTruthy();
  expect(event.show_at).toBe("2026-09-12T21:30:00-06:00");
  expect(event.age_category).toBe("21+");
  expect(event).not.toHaveProperty("price");
  expect(event).not.toHaveProperty("doors_at");
  expect(event.with_adult.ranges[0].condition).toBe(
    "Parent required; minors must leave by 10:30 PM",
  );
  await page.goto(base + event.public_path);
  const dialog = page.getByRole("dialog");
  await expect(dialog).toContainText("Herb's");
  await expect(dialog).toContainText(
    "Parent required; minors must leave by 10:30 PM",
  );
  await expect(
    dialog
      .locator(".admission-clearance")
      .getByRole("link", { name: "Venue admission policy", exact: true }),
  ).toHaveAttribute("href", "https://www.herbsbar.com/");
  await expect(
    dialog.getByRole("link", { name: "View Event", exact: true }),
  ).toHaveAttribute("href", event.event_url);
  await expect(
    dialog.getByRole("link", { name: "Buy Tickets", exact: true }),
  ).toHaveCount(0);
  const ics = await request.get(base + "/api" + event.public_path + ".ics");
  expect(ics.status()).toBe(200);
  expect(await ics.text()).toContain("DTSTART:20260913T033000Z");
  await page.getByRole("button", { name: "Close event details" }).click();
  await openEventDay(page, event.date, isMobile);
  await page.getByRole("checkbox", { name: "With Adult", exact: true }).check();
  await page.getByRole("combobox", { name: "Child’s age" }).selectOption("14");
  await expect(
    page.getByTestId("event-" + event.id).filter({ visible: true }),
  ).toHaveCount(1);
});
