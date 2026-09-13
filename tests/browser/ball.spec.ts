import { expect, test } from "@playwright/test";
import { openPhoneEventDay } from "./filter-controls";

test("Ball Arena distinguishes game admission from unknown concert policy", async ({
  page,
  request,
}, info) => {
  const base = process.env.BALL_BASE_URL;
  test.skip(!base, "Requires explicit Ball Arena publication");
  const response = await request.get(`${base}/api/calendar`);
  expect(response.status()).toBe(200);
  const { events } = await response.json();
  const records = events.filter(
    (e: { venue: string }) => e.venue === "Ball Arena",
  );
  for (const game of [true, false]) {
    const event = records.find(
      (e: { age_category?: string; event_url?: string }) =>
        game
          ? e.age_category === "All ages"
          : !e.age_category &&
            e.event_url?.includes("ballarena.com/event-pages/"),
    );
    expect(event).toBeTruthy();
    expect(event).not.toHaveProperty("price");
    await page.goto(base + event.public_path);
    const dialog = page.getByRole("dialog");
    await expect(dialog).toContainText("Ball Arena");
    await expect(dialog).toContainText("Show");
    await expect(
      dialog.getByRole("link", { name: "View Event", exact: true }),
    ).toHaveAttribute("href", event.event_url);
    await expect(
      dialog.getByRole("link", { name: "Buy Tickets", exact: true }),
    ).toHaveAttribute("href", event.ticket_url);
    if (game) {
      await expect(dialog).toContainText("Age 14: Ticket required");
      await expect(
        dialog.locator(".admission-clearance").getByRole("link", {
          name: "Venue admission policy",
          exact: true,
        }),
      ).toHaveAttribute(
        "href",
        "https://www.ballarena.com/arena-information/arena-policies-faq/",
      );
    } else {
      expect(event.with_adult).toBeUndefined();
      await expect(dialog).toContainText("age policies vary");
    }
    const exported = await request.get(`${base}/api${event.public_path}.ics`);
    expect(exported.status()).toBe(200);
    expect(await exported.text()).toContain("BEGIN:VEVENT");
    await page.getByRole("button", { name: "Close event details" }).click();
    if (info.project.name === "phone")
      await openPhoneEventDay(page, event.date);
    for (const age of [0, 2, 3, 14, 17]) {
      await page
        .getByRole("checkbox", { name: "With Adult", exact: true })
        .check();
      await page
        .getByRole("combobox", { name: "Child’s age" })
        .selectOption(String(age));
      const card = page
        .getByTestId(`event-${event.id}`)
        .filter({ visible: true });
      if (game) await expect(card).toBeVisible();
      else await expect(card).toHaveCount(0);
    }
  }
});
