import { selectCalendarView } from "./view-controls";
import { expect, test } from "@playwright/test";

test("isolated locale controls branding, timezone, palette and default view", async ({
  page,
  request,
}) => {
  const base = process.env.LOCALE_COASTAL_URL;
  test.skip(!base, "Requires the explicit two-locale isolation workflow");
  await page.clock.setFixedTime(new Date("2026-09-12T14:00:00Z"));
  await page.goto(base!);
  await expect(page.locator(".header-brand")).toHaveText(
    "withAdult(coastal): Coastal test refresh.",
  );
  await expect(
    page.getByRole("button", {
      name: "Month",
      exact: true,
      includeHidden: true,
    }),
  ).toHaveAttribute("aria-pressed", "true");
  await selectCalendarView(page, "Week");
  // Auckland is already Sunday September 13; Denver is still September 12.
  await expect(page.getByTestId("range")).toContainText("13");
  const data = await (await request.get(base + "/api/calendar")).json();
  await page.goto(base + data.events[0].public_path);
  await expect(page.getByRole("dialog")).toContainText(
    "Harbor fixture concert",
  );
  await expect(page.locator('link[rel="canonical"]')).toHaveAttribute(
    "href",
    "https://coastal.example" + data.events[0].public_path,
  );
  await expect(page.locator("html")).toHaveAttribute("lang", "en-NZ");
  const site = await (await request.get(base + "/api/site")).json();
  expect(site.venue_colors).toEqual({ Harbor: "#123456" });
  expect(site.sources).toBeUndefined();
});

test("Denver remains independent of the synthetic locale", async ({
  page,
  request,
}) => {
  const base = process.env.LOCALE_DENVER_URL;
  test.skip(!base, "Requires the explicit two-locale isolation workflow");
  await page.goto(base!);
  await expect(page.locator(".header-brand")).toHaveText(
    "withAdult(denver): Bring your people.",
  );
  expect((await (await request.get(base + "/api/site")).json()).timezone).toBe(
    "America/Denver",
  );
  expect(await (await request.get(base + "/sitemap.xml")).text()).not.toContain(
    "coastal.example",
  );
});
