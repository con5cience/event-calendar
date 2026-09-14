import { selectCalendarView } from "./view-controls";
import { expect, test } from "@playwright/test";

function contrast(a: string, b: string) {
  const luminance = (color: string) =>
    color
      .match(/[\d.]+/g)!
      .slice(0, 3)
      .map(Number)
      .map((v) => {
        const s = v / 255;
        return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
      })
      .reduce((sum, v, i) => sum + v * [0.2126, 0.7152, 0.0722][i], 0);
  const values = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (values[0] + 0.05) / (values[1] + 0.05);
}

test("music-finder palette covers calendar, modal, and filters with readable text", async ({
  page,
}, info) => {
  await page.clock.setFixedTime(new Date("2026-09-09T01:00:00Z"));
  await page.goto("/");
  await selectCalendarView(page, "Month");
  await expect(page.locator("html")).toHaveCSS(
    "background-color",
    "rgb(13, 13, 13)",
  );
  await expect(page.locator("html")).toHaveCSS("color", "rgb(224, 224, 224)");
  const selected = page.getByRole("button", {
    name: "Month",
    exact: true,
    includeHidden: true,
  });
  await expect(selected).toHaveCSS("background-color", "rgb(108, 92, 231)");
  await expect(selected).toHaveCSS("color", "rgb(255, 255, 255)");
  if (info.project.name === "desktop") {
    const today = page.getByRole("link", {
      name: "Go to September 9, 2026",
      exact: true,
    });
    await expect(async () => {
      const colors = await today.evaluate((el) =>
        [el, ...el.querySelectorAll("*")].map(
          (node) => getComputedStyle(node).backgroundColor,
        ),
      );
      expect(colors).toContain("rgb(108, 92, 231)");
    }).toPass();
  }
  const card = page
    .getByTestId("event-mission-000000000003")
    .filter({ visible: true });
  await expect(card).toHaveCSS("background-color", "rgb(26, 26, 26)");
  await expect(card).toHaveCSS("border-left-color", "rgb(86, 240, 229)");
  await page.screenshot({
    path: `test-results/theme-calendar-${info.project.name}.png`,
  });
  await card.click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toHaveCSS("background-color", "rgb(26, 26, 26)");
  await expect(dialog.locator("dt").first()).toHaveCSS(
    "color",
    "rgb(136, 136, 136)",
  );
  const link = dialog.getByRole("link", {
    name: "Venue admission policy",
    exact: true,
  });
  await expect(link).toHaveCSS("color", "rgb(169, 156, 255)");
  await link.focus();
  await expect(link).toHaveCSS("outline-color", "rgb(169, 156, 255)");
  await page.screenshot({
    path: `test-results/theme-modal-${info.project.name}.png`,
  });
  await page.getByRole("button", { name: "Close event details" }).click();
  await expect(page.locator('input[type="search"]')).toHaveCSS(
    "border-color",
    "rgb(51, 51, 51)",
  );
  await page
    .getByRole("button", { name: "Venues: All venues", exact: true })
    .click();
  await expect(page.locator(".filter-options")).toHaveCSS(
    "background-color",
    "rgb(26, 26, 26)",
  );
  await page.screenshot({
    path: `test-results/theme-filters-${info.project.name}.png`,
  });
  for (const [foreground, background] of [
    ["rgb(224, 224, 224)", "rgb(13, 13, 13)"],
    ["rgb(224, 224, 224)", "rgb(26, 26, 26)"],
    ["rgb(224, 224, 224)", "rgb(37, 37, 37)"],
    ["rgb(136, 136, 136)", "rgb(26, 26, 26)"],
    ["rgb(169, 156, 255)", "rgb(37, 37, 37)"],
    ["rgb(255, 255, 255)", "rgb(108, 92, 231)"],
    ["rgb(248, 113, 113)", "rgb(26, 26, 26)"],
    ["rgb(196, 181, 253)", "rgb(41, 34, 56)"],
    ["rgb(196, 181, 253)", "rgb(56, 46, 76)"],
  ])
    expect(contrast(foreground, background)).toBeGreaterThanOrEqual(4.5);
});
