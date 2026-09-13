import { expect, test } from "@playwright/test";

test("Show All is horizontally centered and still opens the full day", async ({
  page,
}, info) => {
  await page.clock.setFixedTime(new Date("2026-09-09T01:00:00Z"));
  await page.goto("/");
  // Month overflow remains centered at different heights; phone keeps its cap.
  for (const height of info.project.name === "desktop" ? [900, 2000] : [844]) {
    await page.setViewportSize({
      width: info.project.name === "desktop" ? 1440 : 390,
      height,
    });
    await page.getByRole("button", { name: "Month", exact: true }).click();
    const more = page.getByRole("button", { name: /^Show All/ }).first();
    await expect(more).toBeVisible();
    await expect(more).toHaveCSS("background-color", "rgb(41, 34, 56)");
    await expect(more).toHaveCSS("color", "rgb(196, 181, 253)");
    await expect(more).toHaveCSS("font-weight", "700");
    // FullCalendar can temporarily hide/recreate overflow links during resize.
    // Retry the actual measurement, not only the preceding visibility check.
    let normalHeight = -1;
    await expect
      .poll(async () => {
        normalHeight = (await more.boundingBox())?.height ?? -1;
        return normalHeight;
      })
      .toBeCloseTo(info.project.name === "desktop" ? 22 : 44, 0);
    if (info.project.name === "desktop") {
      await more.hover();
      await expect(more).toHaveCSS("background-color", "rgb(56, 46, 76)");
      await expect
        .poll(async () => (await more.boundingBox())?.height)
        .toBe(normalHeight);
      await page.mouse.move(0, 0);
    }
    await page.keyboard.press("Tab");
    await more.focus();
    await expect(more).toHaveCSS("outline-style", "solid");
    await expect(more).toHaveCSS("outline-width", "3px");
    await more.evaluate((node) => (node as HTMLElement).blur());
    await expect(more).toHaveCSS("background-color", "rgb(41, 34, 56)");
    const centers = await more.evaluate((node) => {
      const range = document.createRange();
      range.selectNodeContents(node);
      const text = range.getBoundingClientRect();
      const cell = (
        node.closest('[role="gridcell"]') || node
      ).getBoundingClientRect();
      return { text: text.x + text.width / 2, cell: cell.x + cell.width / 2 };
    });
    expect(Math.abs(centers.text - centers.cell)).toBeLessThan(3);
    await page.screenshot({
      path: `test-results/show-all-${info.project.name}-${height}.png`,
    });
    await more.click();
    await expect(page.getByTestId("calendar")).toHaveAttribute(
      "data-view",
      info.project.name === "desktop" ? "dayGridDay" : "listDay",
    );
    await expect(
      page.locator('[data-event-date="2026-09-08"]:visible'),
    ).toHaveCount(14);
  }
});
