import { expect, test } from "@playwright/test";

test("child age picker offers only 0–17 at a fixed compact width", async ({
  page,
}) => {
  await page.goto("/");
  await page.getByRole("checkbox", { name: "With Adult", exact: true }).check();
  const age = page.getByRole("combobox", { name: "Child’s age", exact: true });
  await expect(age).toBeVisible();
  await expect(age).toHaveValue("14");
  const values = Array.from({ length: 18 }, (_, index) => String(index));
  await expect(age.locator("option")).toHaveText(values);
  await expect(page.locator(".child-age-caption")).toHaveText("Age:");
  const typography = await page
    .locator(".adult-filter, .child-age-caption, .toolbar-child-age select")
    .evaluateAll((elements) =>
      elements.map((element) => {
        const style = getComputedStyle(element);
        const rect = element.getBoundingClientRect();
        return {
          family: style.fontFamily,
          size: style.fontSize,
          weight: style.fontWeight,
          center: rect.y + rect.height / 2,
        };
      }),
    );
  expect(typography).toHaveLength(3);
  for (const item of typography) {
    expect(item.family).toBe(typography[0].family);
    expect(item.size).toBe(typography[0].size);
    expect(item.weight).toBe(typography[0].weight);
    if (test.info().project.name !== "phone")
      expect(Math.abs(item.center - typography[0].center)).toBeLessThan(1);
  }
  const separator = page.locator(".child-age-separator");
  await expect(separator).toHaveText("·");
  await expect(separator).toHaveAttribute("aria-hidden", "true");
  expect(
    await age
      .locator("option")
      .evaluateAll((options) =>
        options.map((option) => (option as HTMLOptionElement).value),
      ),
  ).toEqual(values);
  for (const value of values) {
    await age.selectOption(value);
    await expect(age).toHaveValue(value);
    expect((await age.boundingBox())!.width).toBe(64);
    expect((await age.boundingBox())!.height).toBeGreaterThanOrEqual(44);
  }
  await page.reload();
  await expect(age).toHaveValue("17");
  await page.screenshot({
    path: `test-results/age-label-${test.info().project.name}.png`,
  });
  await age.focus();
  await expect(age).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(age).not.toBeFocused();
  await page
    .getByRole("checkbox", { name: "With Adult", exact: true })
    .uncheck();
  await expect(age).toHaveCount(0);
  await expect(separator).toHaveCount(0);
});

test("native age picker matches a typed number", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("checkbox", { name: "With Adult", exact: true }).check();
  const age = page.getByRole("combobox", { name: "Child’s age", exact: true });
  await age.click();
  await page.keyboard.press("5");
  await page.keyboard.press("Enter");
  await page.keyboard.press("Tab");
  await expect(age).toHaveValue("5");
});

test("native age picker supports keyboard selection", async ({
  page,
}, info) => {
  test.skip(
    info.project.name === "desktop",
    "Local desktop native-select keyboard automation also fails with a standalone HTML select; requires manual verification.",
  );
  await page.goto("/");
  await page.getByRole("checkbox", { name: "With Adult", exact: true }).check();
  const age = page.getByRole("combobox", { name: "Child’s age", exact: true });
  await age.focus();
  await page.keyboard.press("ArrowUp");
  await page.keyboard.press("Enter");
  await page.keyboard.press("Tab");
  await expect(age).toHaveValue("13");
});

test("admission checkbox uses title case", async ({ page }) => {
  await page.goto("/");
  await expect(
    page.getByRole("checkbox", { name: "With Adult", exact: true }),
  ).toBeVisible();
  await expect(page.locator(".adult-filter")).toHaveText("With Adult");
});

test("filter group is centered between date and search with and without child age", async ({
  page,
}) => {
  await page.goto("/");
  for (const width of [1440, 1920, 2560]) {
    await page.setViewportSize({ width, height: 1000 });
    for (const enabled of [false, true]) {
      await page
        .getByRole("checkbox", { name: "With Adult", exact: true })
        .setChecked(enabled);
      const date = (await page.getByTestId("range").boundingBox())!;
      const filters = (await page.locator(".toolbar-filters").boundingBox())!;
      const search = (await page.getByRole("searchbox").boundingBox())!;
      expect(
        Math.abs(filters.y + filters.height / 2 - search.y - search.height / 2),
      ).toBeLessThan(1);
      const leftGap = filters.x - date.x - date.width;
      const rightGap = search.x - filters.x - filters.width;
      expect(leftGap).toBeGreaterThan(0);
      expect(Math.abs(leftGap - rightGap)).toBeLessThan(1);
    }
  }
});

test("venue and adult controls are outside the drawer and precede search", async ({
  page,
}, info) => {
  await page.goto("/");
  const venues = page.getByRole("button", {
    name: /^Venues:/,
  });
  const adult = page.getByRole("checkbox", { name: "With Adult", exact: true });
  const age = page.getByRole("combobox", {
    name: "Child’s age",
    exact: true,
  });
  await expect(venues).toBeVisible();
  await expect(adult).toBeVisible();
  await expect(age).toHaveCount(0);
  await adult.check();
  await expect(age).toHaveValue("14");
  await age.selectOption("13");
  await page.reload();
  await expect(adult).toBeChecked();
  await expect(age).toHaveValue("13");
  await expect(
    page.locator("dialog .adult-filter, dialog .toolbar-venues"),
  ).toHaveCount(0);
  await adult.uncheck();
  await expect(adult).not.toBeChecked();
  await expect(age).toHaveCount(0);
  if (info.project.name === "desktop") {
    await page.setViewportSize({ width: 1920, height: 1080 });
    const rangeBox = (await page.getByTestId("range").boundingBox())!;
    const venuesBox = (await venues.boundingBox())!;
    const adultBox = (await adult.boundingBox())!;
    const searchBox = (await page.getByRole("searchbox").boundingBox())!;
    expect(rangeBox.x + rangeBox.width).toBeLessThan(venuesBox.x);
    expect(venuesBox.x + venuesBox.width).toBeLessThan(adultBox.x);
    expect(adultBox.x + adultBox.width).toBeLessThan(searchBox.x);
  }
  await venues.click();
  await page
    .getByRole("checkbox", { name: "Mission Ballroom", exact: true })
    .check();
  await page.keyboard.press("Escape");
  await expect(venues).toBeFocused();
  await expect(page.getByTestId("event-gothic-000000000001")).toHaveCount(0);
  for (const width of [1920, 1440, 1024, 767, 390, 320]) {
    await page.setViewportSize({ width, height: 900 });
    await adult.check();
    await expect(age).toBeVisible();
    const adultBounds = (await adult.boundingBox())!;
    const ageBounds = (await age.boundingBox())!;
    if (width >= 768)
      expect(
        Math.abs(
          adultBounds.y +
            adultBounds.height / 2 -
            ageBounds.y -
            ageBounds.height / 2,
        ),
      ).toBeLessThan(2);
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth),
    ).toBeLessThanOrEqual(width);
    await page.screenshot({
      path: `test-results/toolbar-filters-${info.project.name}-${width}.png`,
    });
  }
});
