import { expect, test } from "@playwright/test";

test("mobile toolbar keeps branding, stable filters, and search last", async ({
  page,
}) => {
  await page.goto("/");
  for (const width of [390, 320, 767]) {
    await page.setViewportSize({ width, height: 844 });
    const tagline = page.locator(".brand-tagline");
    await expect(tagline).toBeVisible();
    const brand = (await page.locator(".brand-name").boundingBox())!;
    const tag = (await tagline.boundingBox())!;
    expect(Math.abs(brand.y - tag.y)).toBeLessThan(1);
    const venue = page.getByRole("button", { name: /^Venues:/ });
    const before = (await venue.boundingBox())!;
    const adult = page.getByRole("checkbox", {
      name: "With Adult",
      exact: true,
    });
    await adult.check();
    const after = (await venue.boundingBox())!;
    expect(after.y).toBe(before.y);
    const age = (await page
      .getByRole("combobox", { name: "Child’s age" })
      .boundingBox())!;
    const adultBox = (await adult.boundingBox())!;
    expect(age.y).toBeGreaterThan(adultBox.y);
    const search = (await page.getByRole("searchbox").boundingBox())!;
    expect(search.y).toBeGreaterThanOrEqual(age.y + age.height);
    await expect(
      page.getByRole("button", { name: "Choose calendar view" }),
    ).toBeVisible();
    expect(
      (await page
        .getByRole("button", { name: "Choose calendar view" })
        .boundingBox())!.height,
    ).toBe(44);
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth),
    ).toBeLessThanOrEqual(width);
    await page.screenshot({
      path: `test-results/mobile-toolbar-${test.info().project.name}-${width}.png`,
    });
    await adult.uncheck();
  }
});

test("touch dropdown triggers toggle closed and view selection persists", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  const venue = page.getByRole("button", { name: /^Venues:/ });
  for (const trigger of [
    venue,
    page.getByRole("button", { name: "Choose calendar view" }),
  ]) {
    if (test.info().project.name === "phone") await trigger.tap();
    else await trigger.click();
    await expect(trigger).toHaveAttribute("aria-expanded", "true");
    if (test.info().project.name === "phone") await trigger.tap();
    else await trigger.click();
    await expect(trigger).toHaveAttribute("aria-expanded", "false");
  }
  await page.getByRole("button", { name: "Choose calendar view" }).click();
  await page.getByRole("button", { name: "Month", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Choose calendar view" }),
  ).toHaveText("Month");
  await page.reload();
  await expect(
    page.getByRole("button", { name: "Choose calendar view" }),
  ).toHaveText("Month");
});

test("mobile view menu dismisses outside and with Escape, and supports keyboard selection", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  const trigger = page.getByRole("button", { name: "Choose calendar view" });
  await trigger.click();
  await page.getByRole("searchbox").click();
  await expect(trigger).toHaveAttribute("aria-expanded", "false");
  await trigger.focus();
  await trigger.press("Enter");
  await page.keyboard.press("Tab");
  await expect(
    page.getByRole("button", { name: "Day", exact: true }),
  ).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(trigger).toBeFocused();
  await expect(trigger).toHaveAttribute("aria-expanded", "false");
  await trigger.press("Enter");
  await page.keyboard.press("Tab");
  await page.keyboard.press("Enter");
  await expect(trigger).toHaveText("Day");
  await expect(trigger).toBeFocused();
});
