import { expect, test } from "@playwright/test";
import { openEventDay } from "./filter-controls";
test("Roxy event admission, doors, external unknowns and export reach the app", async ({
  page,
  request,
  isMobile,
}) => {
  const base = process.env.ROXY_BASE_URL;
  test.skip(!base, "Requires explicit Roxy publication");
  const r = await request.get(base + "/api/calendar");
  expect(r.status()).toBe(200);
  const { events } = await r.json();
  const rows = events.filter(
    (e: { venue: string }) => e.venue === "The Roxy Theatre",
  );
  expect(rows.length).toBeGreaterThan(0);
  const native = rows.find(
    (e: { title: string }) => e.title === "STTDM 17! Night 1",
  );
  const external = rows.find(
    (e: { title: string }) => e.title === "Toni Romiti",
  );
  expect(native).toBeTruthy();
  expect(external).toBeTruthy();
  expect(native.doors_at).toBe("2026-09-26T18:00:00-06:00");
  expect(native.show_at).toBe("2026-09-26T19:00:00-06:00");
  expect(native.age_category).toBe("All ages");
  expect(external.with_adult).toBeUndefined();
  expect(external.age_category).toBeUndefined();
  expect(external.show_at).toBeUndefined();
  expect(external.doors_at).toBeUndefined();
  for (const event of [native, external]) {
    expect(event).not.toHaveProperty("price");
    await page.goto(base + event.public_path);
    const dialog = page.getByRole("dialog");
    await expect(dialog).toContainText("The Roxy Theatre");
    await expect(
      dialog.getByRole("link", { name: "View Event", exact: true }),
    ).toHaveAttribute("href", event.event_url);
    await expect(
      dialog.getByRole("link", { name: "Buy Tickets", exact: true }),
    ).toHaveAttribute("href", event.ticket_url);
    if (event === native)
      await expect(dialog).toContainText(
        "All ages; event ticket requirements apply",
      );
    const ics = await request.get(base + "/api" + event.public_path + ".ics");
    expect(ics.status()).toBe(200);
    expect(await ics.text()).toContain(
      event === native
        ? "DTSTART:20260927T000000Z"
        : "DTSTART;VALUE=DATE:20260918",
    );
    await page.getByRole("button", { name: "Close event details" }).click();
    await openEventDay(page, event.date, isMobile);
    await page
      .getByRole("checkbox", { name: "With Adult", exact: true })
      .check();
    await page
      .getByRole("combobox", { name: "Child’s age" })
      .selectOption("14");
    await expect(
      page.getByTestId("event-" + event.id).filter({ visible: true }),
    ).toHaveCount(event === native ? 1 : 0);
  }
});
