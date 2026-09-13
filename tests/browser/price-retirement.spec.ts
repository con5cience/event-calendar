import { expect, test } from "@playwright/test";
import { readFile } from "node:fs/promises";

const path = "/events/mission/2026-09-08-early-doors-000000000003";

test("legacy prices are absent from APIs, modal, and calendar export", async ({
  page,
  request,
}) => {
  const response = await request.get("/api/calendar");
  expect(response.status()).toBe(200);
  const { events } = await response.json();
  const event = events.find(
    (e: { public_path: string }) => e.public_path === path,
  );
  expect(event).toBeTruthy();
  expect(event).not.toHaveProperty("price");
  expect(event).not.toHaveProperty("cost_category");
  const detail = await request.get("/api" + path);
  expect(detail.status()).toBe(200);
  expect(await detail.json()).not.toHaveProperty("price");
  await page.goto(path);
  await expect(
    page
      .getByRole("dialog")
      .locator("dt")
      .filter({ hasText: /^Price$/ }),
  ).toHaveCount(0);
  const download = page.waitForEvent("download");
  await page
    .getByRole("link", { name: "Download Calendar Entry", exact: true })
    .click();
  const file = await (await download).path();
  const body = await readFile(file!, "utf8");
  expect(body).toContain("BEGIN:VEVENT");
  expect(body).not.toContain("Price:");
  expect(body).not.toContain("$20");
});

test("modal ignores price fields from an older API response", async ({
  page,
}) => {
  await page.route("**/api/events/**", async (route) => {
    const response = await route.fetch();
    await route.fulfill({
      json: {
        ...(await response.json()),
        price: "$123 legacy",
        cost_category: "$$$",
      },
    });
  });
  await page.goto(path);
  await expect(page.getByRole("dialog")).toBeVisible();
  await expect(page.getByRole("dialog")).not.toContainText("$123 legacy");
});
