import { expect, test } from "@playwright/test";

test("With Adult help works with focus, hover and tap without toggling the filter", async ({
  page,
}, info) => {
  await page.goto("/");
  const button = page.getByRole("button", {
    name: "About With Adult",
    exact: true,
  });
  const checkbox = page.getByRole("checkbox", {
    name: "With Adult",
    exact: true,
  });
  const tooltip = page.getByRole("tooltip");
  await expect(button).toBeVisible();
  await expect(checkbox).not.toBeChecked();
  await button.focus();
  await expect(tooltip).toHaveText(
    "Shows events whose reviewed venue policies allow someone of the selected age to attend with an adult. Conditions vary—check the event details and venue policy before buying tickets.",
  );
  await expect(button).toHaveAttribute(
    "aria-describedby",
    (await tooltip.getAttribute("id")) as string,
  );
  await page.keyboard.press("Escape");
  await expect(tooltip).toBeHidden();
  await page.keyboard.press("Tab");
  if (info.project.name === "phone") await button.tap();
  else await button.click();
  await expect(tooltip).toBeVisible();
  await expect(checkbox).not.toBeChecked();
  const box = (await tooltip.boundingBox())!;
  expect(box.x).toBeGreaterThanOrEqual(0);
  expect(box.x + box.width).toBeLessThanOrEqual(page.viewportSize()!.width);
  await page.screenshot({
    path: `test-results/admission-help-${info.project.name}.png`,
  });
  await page.getByTestId("range").click();
  await expect(tooltip).toBeHidden();
  if (info.project.name === "desktop") {
    await button.hover();
    await expect(tooltip).toBeVisible();
    await tooltip.hover();
    await expect(tooltip).toBeVisible();
    await page.getByTestId("range").hover();
    await expect(tooltip).toBeHidden();
  }
  await checkbox.check();
  await button.focus();
  await expect(tooltip).toBeVisible();
  const enabledBox = (await tooltip.boundingBox())!;
  expect(enabledBox.x).toBeGreaterThanOrEqual(0);
  expect(enabledBox.x + enabledBox.width).toBeLessThanOrEqual(
    page.viewportSize()!.width,
  );
  await expect(checkbox).toBeChecked();
  await page.keyboard.press("Escape");
  await expect(tooltip).toBeHidden();
});
