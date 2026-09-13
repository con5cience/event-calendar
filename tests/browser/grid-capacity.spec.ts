import { expect, test } from "@playwright/test";

test("desktop overflow leaves less than one unused event row", async ({
  page,
}) => {
  await page.clock.setFixedTime(new Date("2026-09-09T01:00:00Z"));
  await page.setViewportSize({ width: 2048, height: 1300 });
  await page.goto("/");
  const day = page.getByRole("gridcell", {
    name: "September 8, 2026",
    exact: true,
  });
  const more = day.getByRole("button", { name: "Show All", exact: true });
  for (const [view, height] of [
    ["Month", 1300],
    ["Month", 1100],
    ["Week", 420],
    ["Week", 500],
  ] as const) {
    await page.getByRole("button", { name: view, exact: true }).click();
    await expect(page.getByTestId("calendar")).toHaveAttribute(
      "data-view",
      `dayGrid${view}`,
    );
    await page.setViewportSize({ width: 2048, height });
    await expect(more).toBeVisible();
    await expect
      .poll(
        () =>
          day.evaluate((node) => {
            const cards = Array.from(
              node.querySelectorAll("[data-event-date]"),
            ).filter(
              (card) =>
                getComputedStyle(card).visibility !== "hidden" &&
                card.getBoundingClientRect().height > 0,
            );
            const more = node.querySelector(".calendar-more-link");
            if (cards.length < 2 || !more) return false;
            const first = cards[0].getBoundingClientRect();
            const second = cards[1].getBoundingClientRect();
            const cell = node.getBoundingClientRect();
            const link = more.getBoundingClientRect();
            const remaining = cell.y + cell.height - (link.y + link.height);
            const rowHeight = second.y - first.y;
            // Allow one pixel for borders/rounding, but not another whole event row.
            return remaining >= -1 && remaining < rowHeight + 1;
          }),
        {
          message: `${view} at ${height}px must use the available row capacity`,
        },
      )
      .toBe(true);
  }
});
