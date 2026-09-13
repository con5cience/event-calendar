import { expect, test } from "@playwright/test";

for (const category of ["All ages", "13+"]) {
  test(`Red Rocks ${category} policy reaches details and age filtering`, async ({
    page,
    request,
  }, info) => {
    const base = process.env.CLIQUE_BASE_URL;
    test.skip(!base, "Requires explicit Red Rocks publication");
    const response = await request.get(`${base}/api/calendar`);
    expect(response.status()).toBe(200);
    const { events } = await response.json();
    const event = events.find(
      (e: { venue: string; age_category?: string; status?: string }) =>
        e.venue === "Red Rocks Amphitheatre" &&
        e.age_category === category &&
        e.status !== "Cancelled",
    );
    expect(event).toBeTruthy();
    expect(event).not.toHaveProperty("price");
    expect(event.off_site).not.toBe(true);
    expect(
      event.with_adult.ranges.some(
        (r: { min_age: number; max_age: number }) =>
          r.min_age <= 14 && r.max_age >= 14,
      ),
    ).toBe(true);
    await page.goto(base + event.public_path);
    const dialog = page.getByRole("dialog");
    await expect(dialog).toContainText("Red Rocks Amphitheatre");
    await expect(dialog).toContainText("Age 14:");
    await expect(
      dialog
        .locator(".admission-clearance")
        .getByRole("link", { name: "Venue admission policy", exact: true }),
    ).toHaveAttribute("href", event.with_adult.url);
    await expect(
      dialog.getByRole("link", { name: "View Event", exact: true }),
    ).toHaveAttribute("href", event.event_url);
    if (event.ticket_url)
      await expect(
        dialog.getByRole("link", { name: "Buy Tickets", exact: true }),
      ).toHaveAttribute("href", event.ticket_url);
    const ics = await request.get(`${base}/api${event.public_path}.ics`);
    expect(ics.status()).toBe(200);
    expect(await ics.text()).toContain("BEGIN:VEVENT");
    await page.screenshot({
      path: `test-results/clique-${category.replace(/\W/g, "")}-${info.project.name}.png`,
    });
    await page.getByRole("button", { name: "Close event details" }).click();
    await page
      .getByRole("button", { name: "Venues: All venues", exact: true })
      .click();
    await page
      .getByRole("checkbox", { name: "Red Rocks Amphitheatre", exact: true })
      .check();
    await page
      .getByRole("button", {
        name: "Venues: Red Rocks Amphitheatre",
        exact: true,
      })
      .click();
    await page
      .getByRole("checkbox", { name: "With Adult", exact: true })
      .check();
    const card = page
      .getByTestId(`event-${event.id}`)
      .filter({ visible: true });
    await expect(card).toBeVisible();
    for (const age of category === "13+" ? [12, 13, 14] : [1, 2, 14, 17]) {
      await page
        .getByRole("combobox", { name: "Child’s age" })
        .selectOption(String(age));
      if (category === "13+" && age === 12) await expect(card).toHaveCount(0);
      else await expect(card).toBeVisible();
    }
  });
}
test("Red Rocks cancellation stays cancelled in the modal and export", async ({
  page,
  request,
}) => {
  const base = process.env.CLIQUE_BASE_URL;
  test.skip(!base, "Requires explicit Red Rocks publication");
  const { events } = await (await request.get(`${base}/api/calendar`)).json();
  const e = events.find(
    (e: { venue: string; status: string }) =>
      e.venue === "Red Rocks Amphitheatre" && e.status === "Cancelled",
  );
  expect(e).toBeTruthy();
  await page.goto(base + e.public_path);
  await expect(page.getByRole("dialog")).toContainText("Cancelled");
  const ics = await request.get(`${base}/api${e.public_path}.ics`);
  expect(await ics.text()).toContain("STATUS:CANCELLED");
});
