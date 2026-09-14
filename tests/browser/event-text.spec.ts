import { selectCalendarView } from "./view-controls";
import { expect, test } from "@playwright/test";

test("day shows venue while month and week truncate without losing accessible text", async ({
  page,
}, info) => {
  await page.clock.setFixedTime(new Date("2026-09-09T01:00:00Z"));
  const title =
    "A very long event title with special guests and an extended evening performance";
  await page.route("**/api/calendar", async (route) => {
    const response = await route.fetch();
    const data = await response.json();
    data.events = data.events.filter(
      (event: { id: string }) => event.id === "gothic-000000000001",
    );
    data.events[0].title = title;
    data.events[0].status = "Cancelled";
    await route.fulfill({ json: data });
  });
  await page.goto("/");
  for (const view of ["Month", "Week", "Day"]) {
    await selectCalendarView(page, view);
    const card = page
      .getByTestId("event-gothic-000000000001")
      .filter({ visible: true });
    const text = view === "Day" ? `${title} @ Gothic Theatre` : title;
    await expect(card).toHaveText(text);
    await expect(card).toHaveAccessibleName(text);
    const summary = card.locator(".event-summary");
    await expect(summary).toHaveCSS("text-decoration-line", "line-through");
    await expect(summary).toHaveCSS(
      "text-decoration-color",
      "rgb(248, 113, 113)",
    );
    if (view !== "Day") {
      await expect(summary).toHaveCSS("white-space", "nowrap");
      await expect(summary).toHaveCSS("text-overflow", "ellipsis");
      await expect(summary).toHaveCSS("overflow-x", "hidden");
      await expect(async () => {
        const size = await summary.evaluate((el) => ({
          width: el.clientWidth,
          content: el.scrollWidth,
          height: el.getBoundingClientRect().height,
          line: parseFloat(getComputedStyle(el).lineHeight),
        }));
        expect(size.content).toBeGreaterThan(size.width);
        expect(size.height).toBeCloseTo(size.line, 0);
      }).toPass();
    } else {
      await expect(summary).toHaveCSS("white-space", "normal");
    }
    await page.screenshot({
      path: `test-results/event-text-${info.project.name}-${view}.png`,
    });
    await card.click();
    await expect(
      page
        .getByRole("dialog")
        .getByRole("heading", { name: `2026-09-08 : ${title}`, exact: true }),
    ).toBeVisible();
    await page.getByRole("button", { name: "Close event details" }).click();
  }
});
