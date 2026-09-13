import { expect, test } from "@playwright/test";

test("Cervantes on-site events support parent admission at ages 14–15", async ({
  page,
  request,
}, info) => {
  const base = process.env.CERVANTES_BASE_URL;
  test.skip(!base, "Requires explicit Cervantes publication");
  const response = await request.get(`${base}/api/calendar`);
  expect(response.status()).toBe(200);
  const { events } = await response.json();
  const own = events.filter((e: { venue: string }) => e.venue === "Cervantes");
  const allowed = own.find(
    (e: { age_category?: string; with_adult?: unknown }) =>
      e.age_category === "16+" && e.with_adult,
  );
  expect(allowed).toBeTruthy();
  expect(own.every((e: { off_site: boolean }) => !e.off_site)).toBe(true);
  expect(allowed.with_adult.ranges).toEqual([
    { min_age: 14, max_age: 15, condition: "Parent or guardian required" },
    { min_age: 16, max_age: 17, condition: "Permitted at this age" },
  ]);
  expect(allowed).not.toHaveProperty("price");
  await page.goto(base + allowed.public_path);
  const dialog = page.getByRole("dialog");
  await expect(dialog).toContainText("Cervantes");
  await expect(dialog).toContainText("Age 14: Parent or guardian required");
  await expect(
    dialog
      .locator(".admission-clearance")
      .getByRole("link", { name: "Venue admission policy", exact: true }),
  ).toHaveAttribute("href", "https://cervantesmasterpiece.com/faq/");
  await expect(
    dialog.getByRole("link", { name: "Buy Tickets", exact: true }),
  ).toHaveAttribute("href", allowed.ticket_url);
  await expect(
    dialog.getByRole("link", { name: "View Event", exact: true }),
  ).toHaveAttribute("href", allowed.event_url);
  const exported = await request.get(`${base}/api${allowed.public_path}.ics`);
  expect(exported.status()).toBe(200);
  expect(await exported.text()).toContain("BEGIN:VEVENT");
  await page.screenshot({
    path: `test-results/cervantes-${info.project.name}.png`,
  });
  await page.getByRole("button", { name: "Close event details" }).click();
  await page
    .getByRole("button", { name: "Venues: All venues", exact: true })
    .click();
  await page.getByRole("checkbox", { name: "Cervantes", exact: true }).check();
  await page
    .getByRole("button", { name: "Venues: Cervantes", exact: true })
    .click();
  await page.getByRole("checkbox", { name: "With Adult", exact: true }).check();
  const card = page
    .getByTestId(`event-${allowed.id}`)
    .filter({ visible: true });
  await expect(card).toBeVisible();
  for (const age of [13, 14, 15, 16]) {
    await page
      .getByRole("combobox", { name: "Child’s age" })
      .selectOption(String(age));
    if (age === 13) await expect(card).toHaveCount(0);
    else await expect(card).toBeVisible();
  }
});
