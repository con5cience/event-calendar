import { selectCalendarView } from "./view-controls";
import { expect, test } from "@playwright/test";

test("favicon serves the approved transparent purple artwork", async ({
  page,
  request,
}) => {
  await page.goto("/");
  const icon = page.locator('head link[rel="icon"]');
  await expect(icon).toHaveAttribute("type", "image/png");
  await expect(icon).toHaveAttribute("href", "/assets/favicon.png");
  const response = await request.get("/assets/favicon.png");
  expect(response.status()).toBe(200);
  expect(response.headers()["content-type"]).toContain("image/png");
  const pixels = await page.evaluate(async () => {
    const image = new Image();
    image.src = "/assets/favicon.png";
    await image.decode();
    const canvas = document.createElement("canvas");
    canvas.width = image.width;
    canvas.height = image.height;
    const context = canvas.getContext("2d")!;
    context.drawImage(image, 0, 0);
    return {
      width: image.width,
      height: image.height,
      background: [...context.getImageData(0, 0, 1, 1).data],
      artwork: [...context.getImageData(150, 400, 1, 1).data],
    };
  });
  expect(pixels).toEqual({
    width: 554,
    height: 554,
    background: [0, 0, 0, 0],
    artwork: [108, 92, 231, 255],
  });
});

test("uniform brand header frames the calendar without a footer", async ({
  page,
}, info) => {
  await page.goto("/");
  const header = page.getByRole("banner");
  await expect(header).toHaveCSS("border-bottom-width", "0px");
  await expect(page.getByRole("main")).toHaveCSS(
    "padding-top",
    info.project.name === "phone" ? "6px" : "10px",
  );
  await expect(page.getByRole("contentinfo")).toHaveCount(0);
  await expect(header.locator(".header-brand")).toHaveText(
    "withAdult(denver): Bring your people.",
  );
  for (const selector of [".brand-name", ".brand-city", ".brand-tagline"]) {
    expect(
      await header
        .locator(selector)
        .evaluate((el) => getComputedStyle(el).fontSize),
    ).toBe(await header.evaluate((el) => getComputedStyle(el).fontSize));
    await expect(header.locator(selector)).toHaveCSS("font-weight", "700");
    expect(
      await header
        .locator(selector)
        .evaluate((el) => getComputedStyle(el).fontFamily),
    ).toBe(await header.evaluate((el) => getComputedStyle(el).fontFamily));
  }
  await expect(header.locator(".brand-name")).toHaveCSS(
    "color",
    "rgb(108, 92, 231)",
  );
  await expect(header.locator(".brand-city")).toHaveCSS(
    "color",
    "rgb(224, 224, 224)",
  );
  await expect(header.locator(".brand-tagline")).toHaveCSS(
    "color",
    "rgb(224, 224, 224)",
  );
  await expect(header.locator(".brand-tagline")).toHaveText(
    ": Bring your people.",
  );
  await expect(header.locator(".brand-tagline")).toBeVisible();
  expect((await header.boundingBox())!.height).toBe(36);
  for (const view of ["Week", "Month", "Day"]) {
    await selectCalendarView(page, view);
    await expect(async () => {
      const calendar = await page.getByTestId("calendar").boundingBox();
      const main = await page.getByRole("main").boundingBox();
      expect(main!.y + main!.height).toBeGreaterThanOrEqual(
        calendar!.y + calendar!.height - 1,
      );
    }).toPass();
  }
  await selectCalendarView(page, "Week");
  await page.screenshot({
    path: `test-results/branding-${info.project.name}.png`,
    fullPage: true,
  });
});
