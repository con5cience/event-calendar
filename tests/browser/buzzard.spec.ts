import { expect, test } from "@playwright/test";
import { openPhoneEventDay } from "./filter-controls";
test("Black Buzzard keeps separate shows, links and no inferred guardian exception", async ({
  page,
  request,
  isMobile,
}) => {
  const base = process.env.BUZZARD_BASE_URL;
  test.skip(!base, "Requires explicit Black Buzzard publication");
  const r = await request.get(base + "/api/calendar");
  expect(r.status()).toBe(200);
  const { events } = await r.json();
  const rows = events.filter(
    (e: { venue: string }) => e.venue === "The Black Buzzard",
  );
  expect(rows.length).toBeGreaterThan(0);
  const shows = rows.filter((e: { title: string }) =>
    e.title.startsWith("Tim Butterly"),
  );
  expect(shows).toHaveLength(2);
  expect(shows[0].public_path).not.toBe(shows[1].public_path);
  expect(shows[0].event_url).not.toBe(shows[1].event_url);
  for (const event of shows) {
    expect(event.age_category).toBe("18+");
    expect(event.with_adult).toBeUndefined();
    expect(event).not.toHaveProperty("price");
    expect(event).not.toHaveProperty("doors_at");
    expect(event).not.toHaveProperty("show_at");
    await page.goto(base + event.public_path);
    const dialog = page.getByRole("dialog");
    await expect(dialog).toContainText("The Black Buzzard");
    await expect(dialog).toContainText(
      "18+ unless otherwise posted; ID required",
    );
    await expect(
      dialog.getByRole("link", { name: "View Event", exact: true }),
    ).toHaveAttribute("href", event.event_url);
    await expect(
      dialog.getByRole("link", { name: "Buy Tickets", exact: true }),
    ).toHaveAttribute("href", event.ticket_url);
    const ics = await request.get(base + "/api" + event.public_path + ".ics");
    expect(ics.status()).toBe(200);
    expect(await ics.text()).toContain("BEGIN:VEVENT");
  }
  const event = shows[0];
  await page.goto(base + event.public_path);
  await page.getByRole("button", { name: "Close event details" }).click();
  if (isMobile) await openPhoneEventDay(page, event.date);
  await page.getByRole("checkbox", { name: "With Adult", exact: true }).check();
  await page.getByRole("combobox", { name: "Child’s age" }).selectOption("14");
  await expect(
    page.getByTestId("event-" + event.id).filter({ visible: true }),
  ).toHaveCount(0);
});
