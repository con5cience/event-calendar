import { expect, test } from "@playwright/test";

test("toolbar has no drawer or reset and ignores retired age selections", async ({
  page,
}) => {
  await page.addInitScript(() =>
    localStorage.setItem(
      "event-calendar.filters.v1",
      JSON.stringify({
        venues: [],
        query: "",
        ages: ["21+"],
        withAdult: false,
        childAge: 14,
      }),
    ),
  );
  await page.goto("/");
  await expect(
    page.getByRole("button", { name: "Week", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Filters", exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "Reset filters", exact: true }),
  ).toHaveCount(0);
  await expect(page.getByRole("button", { name: /^Age:/ })).toHaveCount(0);
  await expect(page.locator(".filter-drawer")).toHaveCount(0);
  await expect(
    page.getByTestId("event-gothic-000000000001").filter({ visible: true }),
  ).toBeVisible();
  await page.reload();
  await expect(
    page.getByTestId("event-gothic-000000000001").filter({ visible: true }),
  ).toBeVisible();
  for (const width of [1920, 1440, 1024, 390, 320]) {
    await page.setViewportSize({ width, height: 900 });
    const previous = (await page
      .getByRole("button", { name: "Previous", exact: true })
      .boundingBox())!;
    const calendar = (await page.getByTestId("calendar").boundingBox())!;
    expect(Math.abs(previous.x - calendar.x)).toBeLessThan(1);
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth),
    ).toBeLessThanOrEqual(width);
    await page.screenshot({
      path: `test-results/toolbar-simple-${test.info().project.name}-${width}.png`,
    });
  }
});
