import { expect, test } from "@playwright/test";

test("a stalled startup restores the fallback after the bounded wait", async ({
  page,
}) => {
  await page.clock.install();
  let release!: () => void;
  const gate = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.route("**/api/site", async (route) => {
    await gate;
    await route.abort();
  });
  try {
    await page.goto("/", { waitUntil: "domcontentloaded" });
    await expect(page.locator("#boot-status")).toBeVisible();
    await page.clock.fastForward(15001);
    await expect(page.locator(".server-fallback")).toBeVisible();
    await expect(page.locator("#boot-status")).toBeHidden();
  } finally {
    release();
  }
});

test("slow JavaScript and site configuration never flash the fallback list", async ({
  page,
}) => {
  let releaseModule!: () => void;
  const moduleGate = new Promise<void>((resolve) => {
    releaseModule = resolve;
  });
  let releaseSite!: () => void;
  const siteGate = new Promise<void>((resolve) => {
    releaseSite = resolve;
  });
  await page.route(/\/assets\/index-[^/]+\.js$/, async (route) => {
    await moduleGate;
    await route.continue();
  });
  await page.route("**/api/site", async (route) => {
    await siteGate;
    await route.continue();
  });
  try {
    await page.goto("/", { waitUntil: "commit" });
    await expect(page.locator("#boot-status")).toBeVisible();
    await expect(page.locator(".server-fallback")).toBeHidden();
    await expect(page.locator("html")).toHaveCSS(
      "background-color",
      "rgb(13, 13, 13)",
    );
    releaseModule();
    await expect(page.locator("#boot-status")).toBeVisible();
    await expect(page.locator(".server-fallback")).toBeHidden();
    releaseSite();
    await expect(page.getByTestId("calendar")).toBeVisible();
    await expect(page.locator("#boot-status")).toBeHidden();
    await expect(page.locator(".server-fallback")).toHaveCount(0);
  } finally {
    releaseModule();
    releaseSite();
  }
});

for (const failure of ["site", "module"] as const) {
  test(`startup ${failure} failure restores the readable fallback`, async ({
    page,
  }) => {
    if (failure === "site")
      await page.route("**/api/site", (route) =>
        route.fulfill({ status: 503, body: "Unavailable" }),
      );
    else
      await page.route(/\/assets\/index-[^/]+\.js$/, (route) => route.abort());
    await page.goto("/");
    await expect(page.locator(".server-fallback")).toBeVisible();
    await expect(
      page.locator('.server-fallback a[href^="/events/"]').first(),
    ).toBeVisible();
    await expect(page.locator("#boot-status")).toBeHidden();
  });
}

test("no-JavaScript homepage has a visible chronologically sorted list", async ({
  browser,
  baseURL,
}) => {
  const context = await browser.newContext({ javaScriptEnabled: false });
  try {
    const page = await context.newPage();
    await page.goto(baseURL!);
    await expect(page.locator(".server-fallback")).toBeVisible();
    await expect(page.locator("#boot-status")).toBeHidden();
    const rows = await page.locator(".server-fallback li a").allTextContents();
    expect(rows.length).toBeGreaterThan(0);
    const dates = rows.map((row) => row.slice(0, 10));
    expect(dates).toEqual([...dates].sort());
  } finally {
    await context.close();
  }
});
