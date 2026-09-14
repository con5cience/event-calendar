import { selectCalendarView } from "./view-controls";
import { expect, test } from "@playwright/test";

test("desktop cards fit text height while phone cards keep touch size", async ({
  page,
}, info) => {
  await page.clock.setFixedTime(new Date("2026-09-09T01:00:00Z"));
  await page.route("**/api/calendar", async (route) => {
    const response = await route.fetch();
    const data = await response.json();
    data.events = data.events.filter((event: { id: string }) =>
      ["gothic-000000000001", "mission-000000000002"].includes(event.id),
    );
    data.events[0].title = "Short";
    data.events[1].title = "A longer event title on two lines";
    await route.fulfill({ json: data });
  });
  await page.goto("/");
  for (const view of ["Month", "Week", "Day"]) {
    await selectCalendarView(page, view);
    const card = page
      .getByTestId("event-gothic-000000000001")
      .filter({ visible: true });
    await expect(card).toBeVisible();
    await expect(async () => {
      const size = await card.evaluate((node) => {
        const style = getComputedStyle(node);
        return {
          height: node.getBoundingClientRect().height,
          line: parseFloat(style.lineHeight),
          padding:
            parseFloat(style.paddingTop) + parseFloat(style.paddingBottom),
          font: style.fontSize,
        };
      });
      if (info.project.name === "desktop") {
        expect(Math.abs(size.height - size.line - size.padding)).toBeLessThan(
          1,
        );
        expect(size.height).toBeLessThan(44);
        expect(size.font).toBe("12.8px");
        expect(size.padding).toBe(view === "Month" ? 8 : 16);
        if (view === "Month") {
          const wrapped = page
            .getByTestId("event-mission-000000000002")
            .filter({ visible: true });
          const bounds = (await wrapped.boundingBox())!;
          expect(bounds.height).toBeCloseTo(size.height, 0);
        }
      } else {
        expect(size.height).toBeGreaterThanOrEqual(44);
        expect(size.padding).toBe(20);
      }
    }).toPass();
    await expect(
      page.getByRole("button", { name: "Previous", exact: true }),
    ).toHaveCSS("min-height", "44px");
    await page.screenshot({
      path: `test-results/card-height-${info.project.name}-${view}.png`,
    });
  }
});
