import { expect, test } from "@playwright/test";

test("With Adult uses child age, leaves no hidden restriction filter, and explains conditions", async ({
  page,
}) => {
  await page.clock.setFixedTime(new Date("2026-09-09T01:00:00Z"));
  await page.route("**/api/calendar", async (route) => {
    const response = await route.fetch();
    const data = await response.json();
    data.events[0].with_adult = {
      url: "https://example.com/policy",
      reviewed_on: "2026-09-09",
      ranges: [
        { min_age: 11, max_age: 15, condition: "Ticketed adult required" },
      ],
    };
    data.events[0].age_category = "16+";
    await route.fulfill({ json: data });
  });
  await page.goto("/");
  await page.getByRole("checkbox", { name: "With Adult", exact: true }).check();
  const age = page.getByRole("combobox", {
    name: "Child’s age",
    exact: true,
  });
  await expect(age).toHaveValue("14");
  const cards = page.locator(".event-card:visible");
  await expect(cards).toHaveCount(1);
  await cards.first().click();
  await expect(page.getByRole("dialog")).toContainText(
    "Age 14: Ticketed adult required",
  );
  await expect(
    page.getByRole("link", { name: "Venue admission policy", exact: true }),
  ).toHaveAttribute("href", "https://example.com/policy");
  const dialog = page.getByRole("dialog");
  const clearance = dialog.locator("dl .admission-clearance");
  await expect(clearance).toContainText("Age 14: Ticketed adult required");
  await expect(clearance.locator("xpath=preceding-sibling::dt[1]")).toHaveText(
    "With Adult",
  );
  await expect(dialog).not.toContainText("reviewed 2026-09-09");
  const policy = clearance.getByRole("link", {
    name: "Venue admission policy",
    exact: true,
  });
  expect(await policy.innerText()).toBe("");
  await expect(policy.locator("svg")).toHaveAttribute(
    "data-icon",
    "arrow-up-right-from-square",
  );
  await expect(policy.locator("svg")).toHaveAttribute("aria-hidden", "true");
  await policy.focus();
  await expect(dialog.getByRole("tooltip")).toHaveText(
    "Venue admission policy",
  );
  const alignment = await clearance.evaluate((dd) => {
    const range = document.createRange();
    range.selectNodeContents(dd.previousElementSibling!);
    const label = range.getBoundingClientRect();
    range.selectNodeContents(dd.firstChild!);
    const value = range.getClientRects()[0];
    const svg = dd.querySelector("svg")!.getBoundingClientRect();
    const link = dd.querySelector("a")!.getBoundingClientRect();
    return {
      offset: value.top - label.top,
      iconWidth: svg.width,
      iconHeight: svg.height,
      iconInset: svg.left - link.left,
      targetWidth: link.width,
      targetHeight: link.height,
    };
  });
  expect(Math.abs(alignment.offset)).toBeLessThan(1);
  expect(alignment.iconWidth).toBe(14);
  expect(alignment.iconHeight).toBe(14);
  expect(Math.abs(alignment.iconInset)).toBeLessThan(1);
  expect(alignment.targetWidth).toBeGreaterThanOrEqual(44);
  expect(alignment.targetHeight).toBeGreaterThanOrEqual(44);
  const spacing = await dialog.locator("dl").evaluate((dl) => {
    const labels = Array.from(dl.querySelectorAll("dt"));
    const tops = labels.map((label) => {
      const range = document.createRange();
      range.selectNodeContents(label);
      return range.getBoundingClientRect().top;
    });
    return {
      normal: tops[1] - tops[0],
      admission: tops.at(-1)! - tops.at(-2)!,
    };
  });
  expect(Math.abs(spacing.admission - spacing.normal)).toBeLessThan(1);
  const row = await clearance.boundingBox();
  const icon = await policy.boundingBox();
  expect(row).toBeTruthy();
  expect(icon).toBeTruthy();
  expect(icon!.x).toBeGreaterThanOrEqual(row!.x);
  expect(icon!.x + icon!.width).toBeLessThanOrEqual(row!.x + row!.width + 1);
  await page.screenshot({
    path: `test-results/admission-metadata-${test.info().project.name}.png`,
  });
  await page.getByRole("button", { name: "Close event details" }).click();
  await expect(page).toHaveURL(/\/$/);
  await page.reload();
  await expect(cards).toHaveCount(1);
  await expect(
    page.getByRole("checkbox", { name: "With Adult", exact: true }),
  ).toBeChecked();
  await age.selectOption("10");
  await expect(
    page.getByText("No events match your filters.", { exact: true }),
  ).toBeVisible();
  await page
    .getByRole("checkbox", { name: "With Adult", exact: true })
    .uncheck();
  await expect(age).toHaveCount(0);
  await expect(cards.first()).toBeVisible();
  await expect(
    page.getByRole("checkbox", { name: "With Adult", exact: true }),
  ).not.toBeChecked();
});
