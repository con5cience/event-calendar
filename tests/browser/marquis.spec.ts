import { expect, test } from "@playwright/test";
import { openPhoneEventDay } from "./filter-controls";

for (const source of ["marquis", "summit", "fillmore"]) {
  const venue =
    source === "fillmore"
      ? "Fillmore Auditorium"
      : source === "marquis"
        ? "Marquis"
        : "Summit";
  test(`${venue} admission, times, links and export reach the app`, async ({
    page,
    request,
    isMobile,
  }) => {
    const base = process.env[`${source.toUpperCase()}_BASE_URL`];
    test.skip(!base, `Requires explicit ${venue} publication`);
    const response = await request.get(`${base}/api/calendar`);
    expect(response.status()).toBe(200);
    const { events } = await response.json();
    const records = events.filter((e: { venue: string }) => e.venue === venue);
    for (const category of source === "fillmore"
      ? ["All ages", "16+", "18+", "21+"]
      : ["All ages", "18+"]) {
      const event = records.find(
        (e: { age_category?: string }) => e.age_category === category,
      );
      expect(event).toBeTruthy();
      expect(event).not.toHaveProperty("price");
      await page.goto(base + event.public_path);
      const dialog = page.getByRole("dialog");
      await expect(dialog).toContainText(venue);
      await expect(dialog).toContainText("Doors");
      await expect(
        dialog.getByRole("link", { name: "View Event", exact: true }),
      ).toHaveAttribute("href", event.event_url);
      await expect(
        dialog.getByRole("link", { name: "Buy Tickets", exact: true }),
      ).toHaveAttribute("href", event.ticket_url);
      if (category === "All ages") {
        await expect(dialog).toContainText("Age 14: Ticket required");
        await expect(
          dialog.locator(".admission-clearance").getByRole("link", {
            name: "Venue admission policy",
            exact: true,
          }),
        ).toHaveAttribute("href", `https://www.${source}denver.com/visit`);
      } else if (category === "16+" && source === "fillmore") {
        expect(event.with_adult.ranges).toEqual([
          {
            min_age: 16,
            max_age: 17,
            condition:
              "Ticket and valid ID required; no admission under 16 even with an adult",
          },
        ]);
      } else expect(event.with_adult).toBeUndefined();
      const exported = await request.get(`${base}/api${event.public_path}.ics`);
      expect(exported.status()).toBe(200);
      expect(await exported.text()).toContain("BEGIN:VEVENT");
      await page.getByRole("button", { name: "Close event details" }).click();
      if (isMobile) await openPhoneEventDay(page, event.date);
      for (const age of [0, 2, 3, 14, 15, 16, 17]) {
        await page
          .getByRole("checkbox", { name: "With Adult", exact: true })
          .check();
        await page
          .getByRole("combobox", { name: "Child’s age" })
          .selectOption(String(age));
        const card = page
          .getByTestId(`event-${event.id}`)
          .filter({ visible: true });
        if (
          category === "All ages" ||
          (source === "fillmore" && category === "16+" && age >= 16)
        )
          await expect(card).toBeVisible();
        else await expect(card).toHaveCount(0);
      }
    }
  });
}
