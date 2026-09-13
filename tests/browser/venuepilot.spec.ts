import { expect, test } from "@playwright/test";
import { openPhoneEventDay } from "./filter-controls";

test("Levitt admission, event links and exports reach the app", async ({
  page,
  request,
  isMobile,
}) => {
  const base = process.env.LEVITT_BASE_URL;
  test.skip(!base, "Requires explicit Levitt publication");
  const response = await request.get(base + "/api/calendar");
  expect(response.status()).toBe(200);
  const { events } = await response.json();
  const records = events.filter(
    (e: { venue: string }) => e.venue === "Levitt Pavilion Denver",
  );
  expect(records.length).toBeGreaterThan(0);
  for (const category of ["All ages", "21+"]) {
    const event = records.find(
      (e: { age_category: string }) => e.age_category === category,
    );
    expect(event).toBeTruthy();
    expect(event).not.toHaveProperty("price");
    expect(event.event_url).toMatch(
      /^https:\/\/www\.levittdenver\.org\/summer-concert-series#\/events\/\d+$/,
    );
    await page.goto(base + event.public_path);
    const dialog = page.getByRole("dialog");
    await expect(dialog).toContainText("Levitt Pavilion Denver");
    await expect(dialog).toContainText(category);
    await expect(
      dialog.getByRole("link", { name: "View Event", exact: true }),
    ).toHaveAttribute("href", event.event_url);
    await expect(
      dialog.getByRole("link", { name: "Buy Tickets", exact: true }),
    ).toHaveAttribute("href", event.ticket_url);
    if (category === "All ages") {
      expect(event.with_adult.url).toBe("https://www.levittdenver.org/faq");
      await expect(dialog).toContainText("Adult required");
      await expect(
        dialog.locator(".admission-clearance").getByRole("link", {
          name: "Venue admission policy",
          exact: true,
        }),
      ).toHaveAttribute("href", "https://www.levittdenver.org/faq");
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
    const entry = page
      .getByTestId("event-" + event.id)
      .filter({ visible: true });
    if (category === "All ages") await expect(entry).toHaveCount(1);
    else await expect(entry).toHaveCount(0);
  }
});
