import { expect, test } from "@playwright/test";

test("event metadata and content work without JavaScript", async ({
  browser,
  request,
}) => {
  const response = await request.get("/api/calendar");
  const { events } = await response.json();
  const event = events.find(
    (entry: { public_path?: string }) => entry.public_path,
  );
  expect(event).toBeTruthy();
  const context = await browser.newContext({ javaScriptEnabled: false });
  const page = await context.newPage();
  try {
    const origin = new URL(response.url()).origin;
    await page.goto(origin + event.public_path);
    await expect(page.getByRole("heading", { level: 1 })).toHaveText(
      event.title,
    );
    await expect(page).toHaveTitle(
      `${event.title} @ ${event.venue} · ${event.date} | withAdult(denver)`,
    );
    await expect(page.locator('link[rel="canonical"]')).toHaveAttribute(
      "href",
      "https://denver.withadult.com" + event.public_path,
    );
    await expect(page.locator('meta[property="og:title"]')).toHaveAttribute(
      "content",
      await page.title(),
    );
    await expect(page.locator('meta[property="og:image"]')).toHaveAttribute(
      "content",
      "https://denver.withadult.com/assets/favicon.png",
    );
    const image = await request.get("/assets/favicon.png");
    expect(image.status()).toBe(200);
    expect(image.headers()["content-type"]).toContain("image/png");
    await page.goto(origin);
    await expect(page.locator(`a[href="${event.public_path}"]`)).toHaveCount(1);
    expect((await request.get("/sitemap.xml")).status()).toBe(200);
    expect(await (await request.get("/robots.txt")).text()).toContain(
      "Sitemap: https://denver.withadult.com/sitemap.xml",
    );
  } finally {
    await context.close();
  }
});

test("React replaces fallback content and opens shared event details", async ({
  page,
  request,
}) => {
  const { events } = await (await request.get("/api/calendar")).json();
  const event = events.find(
    (entry: { public_path?: string }) => entry.public_path,
  );
  await page.goto(event.public_path);
  await expect(page.getByRole("dialog")).toBeVisible();
  await expect(page.getByRole("dialog")).toContainText(event.title);
  await expect(
    page.getByText(
      "Enable JavaScript to use the interactive calendar and filters.",
    ),
  ).toHaveCount(0);
  await expect(page.getByTestId("calendar")).toBeVisible();
});
