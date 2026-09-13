import { expect, test } from "@playwright/test";

test("live AEG expansion opens source events and exposes venue filters", async ({
  page,
  request,
}) => {
  test.skip(
    !process.env.LIVE_AEG_BASE_URL,
    "Requires explicitly populated local AEG store",
  );
  const base = process.env.LIVE_AEG_BASE_URL!;
  const response = await request.get(`${base}/api/calendar`);
  expect(response.status()).toBe(200);
  const { events } = await response.json();
  for (const venue of [
    "Bluebird Theater",
    "Ogden Theatre",
    "Fiddler's Green Amphitheatre",
  ]) {
    const event = events.find((e: { venue: string }) => e.venue === venue);
    expect(event).toBeDefined();
    await page.goto(base + event.public_path);
    const dialog = page.getByRole("dialog");
    await expect(
      dialog.getByRole("heading", {
        name: `${event.date} : ${event.title}`,
        exact: true,
      }),
    ).toBeVisible();
    await expect(dialog.getByText(venue, { exact: true })).toBeVisible();
    await expect(
      dialog.getByRole("link", { name: "View Event", exact: true }),
    ).toHaveAttribute("href", event.event_url);
    await expect(
      dialog.getByRole("link", { name: "Buy Tickets", exact: true }),
    ).toHaveAttribute("href", event.ticket_url);
    await page.getByRole("button", { name: "Close event details" }).click();
    await page
      .getByRole("button", { name: "Venues: All venues", exact: true })
      .click();
    await expect(
      page.getByRole("checkbox", { name: venue, exact: true }),
    ).toBeVisible();
  }
});
