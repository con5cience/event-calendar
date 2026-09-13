import { expect, test } from "@playwright/test";

test("desktop month fits five and six weeks after viewport resize", async ({
  page,
}, info) => {
  test.skip(info.project.name !== "desktop", "Desktop grid sizing only");
  await page.clock.setFixedTime(new Date("2026-09-09T01:00:00Z"));
  await page.route("**/api/calendar", async (route) => {
    const response = await route.fetch();
    const data = await response.json();
    data.events.push(
      ...data.events
        .filter((event: { date: string }) => event.date === "2026-09-08")
        .map((event: { id: string }) => ({
          ...event,
          id: `august-${event.id}`,
          date: "2026-08-31",
        })),
    );
    await route.fulfill({ json: data });
  });
  await page.goto("/");
  await page.getByRole("button", { name: "Month", exact: true }).click();
  for (const viewport of [
    { width: 1440, height: 900 },
    { width: 1280, height: 800 },
    { width: 1024, height: 768 },
    { width: 1440, height: 1200 },
    { width: 1440, height: 2000 },
  ]) {
    await page.setViewportSize(viewport);
    for (const [month, weeks] of [
      ["September", 5],
      ["August", 6],
    ] as const) {
      await expect(page.getByTestId("range")).toHaveText(`${month} 2026`);
      await expect(page.getByRole("row", { name: /^Week / })).toHaveCount(
        weeks,
      );
      await expect
        .poll(() =>
          page
            .getByTestId("calendar")
            .evaluate((node) => node.getBoundingClientRect().bottom),
        )
        .toBeLessThanOrEqual(viewport.height);
      const rows = await page.getByRole("row", { name: /^Week / }).all();
      for (const row of rows) {
        const bounds = (await row.boundingBox())!;
        expect(bounds.y + bounds.height).toBeLessThanOrEqual(viewport.height);
      }
      if (month === "August") {
        const lastDay = page.getByRole("gridcell", {
          name: "August 31, 2026",
          exact: true,
        });
        await expect(
          lastDay.getByRole("button", { name: "Show All", exact: true }),
        ).toBeInViewport();
        await lastDay
          .getByRole("button", { name: "Show All", exact: true })
          .click();
        await expect(
          page.locator('[data-event-date="2026-08-31"]:visible'),
        ).toHaveCount(14);
        await page.getByRole("button", { name: "Month", exact: true }).click();
      }
      await page
        .getByRole("button", {
          name: month === "September" ? "Previous" : "Next",
          exact: true,
        })
        .click();
    }
    const visible = page.locator('[data-event-date="2026-09-08"]:visible');
    expect(await visible.count()).toBeLessThan(14);
    if (viewport.height === 2000)
      await expect.poll(() => visible.count()).toBeGreaterThan(5);
    await page
      .getByRole("button", { name: "Show All", exact: true })
      .first()
      .click();
    await expect(page.getByTestId("calendar")).toHaveAttribute(
      "data-view",
      "dayGridDay",
    );
    await expect(
      page.locator('[data-event-date="2026-09-08"]:visible'),
    ).toHaveCount(14);
    await page.getByRole("button", { name: "Month", exact: true }).click();
  }
  await page.screenshot({
    path: "test-results/desktop-month-fit.png",
    fullPage: true,
  });
});

test("desktop Month omits Show All when every event fits", async ({ page }) => {
  await page.clock.setFixedTime(new Date("2026-09-09T01:00:00Z"));
  await page.setViewportSize({ width: 1440, height: 4000 });
  await page.goto("/");
  await page.getByRole("button", { name: "Month", exact: true }).click();
  const day = page.getByRole("gridcell", {
    name: "September 8, 2026",
    exact: true,
  });
  const cards = day.locator("[data-event-date]:visible");
  const more = day.getByRole("button", { name: "Show All", exact: true });
  await expect(cards).toHaveCount(14);
  await expect(more).toHaveCount(0);
  await page.setViewportSize({ width: 1024, height: 768 });
  await expect(more).toBeVisible();
  expect(await cards.count()).toBeLessThan(14);
  const cell = (await day.boundingBox())!;
  const link = (await more.boundingBox())!;
  expect(link.y + link.height).toBeLessThanOrEqual(cell.y + cell.height);
  await more.click();
  await expect(
    page.locator('[data-event-date="2026-09-08"]:visible'),
  ).toHaveCount(14);
  await page.getByRole("button", { name: "Month", exact: true }).click();
  await page.setViewportSize({ width: 1440, height: 4000 });
  await expect(cards).toHaveCount(14);
  await expect(more).toHaveCount(0);
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByTestId("calendar")).toHaveAttribute(
    "data-view",
    "listMonth",
  );
  await expect(
    page.locator('[data-event-date="2026-09-08"]:visible'),
  ).toHaveCount(5);
});
