import { expect, test } from "@playwright/test";

test("header contact link stays at the right without wrapping branding", async ({
  page,
}) => {
  await page.goto("/");
  const link = page.getByRole("link", { name: "Contact Us", exact: true });
  await expect(link).toHaveAttribute("href", "mailto:contact@withadult.com");
  await expect(link.locator('[data-icon="envelope"]')).toHaveAttribute(
    "aria-hidden",
    "true",
  );
  for (const width of [1440, 390, 320]) {
    await page.setViewportSize({ width, height: 900 });
    const box = (await link.boundingBox())!;
    const brand = (await page.locator(".brand-name").boundingBox())!;
    const tagline = (await page.locator(".brand-tagline").boundingBox())!;
    expect(Math.abs(brand.y - tagline.y)).toBeLessThan(1);
    expect(tagline.x + tagline.width).toBeLessThanOrEqual(box.x);
    expect(box.width).toBeGreaterThanOrEqual(32);
    expect(box.height).toBeGreaterThanOrEqual(32);
    expect(
      Math.abs(width - Math.max(12, width * 0.05) - box.x - box.width),
    ).toBeLessThan(1);
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth),
    ).toBeLessThanOrEqual(width);
    await link.focus();
    await expect(page.getByRole("tooltip")).toHaveText("Contact Us");
    const tip = (await page.getByRole("tooltip").boundingBox())!;
    expect(tip.x).toBeGreaterThanOrEqual(0);
    expect(tip.x + tip.width).toBeLessThanOrEqual(width);
    await page.screenshot({
      path: `test-results/contact-${test.info().project.name}-${width}.png`,
    });
    await link.press("Tab");
  }
});
