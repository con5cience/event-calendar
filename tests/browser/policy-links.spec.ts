import { expect, test } from "@playwright/test";

test("age policy is normalized plain text with an accessible external policy icon", async ({
  page,
}, info) => {
  await page.clock.setFixedTime(new Date("2026-09-09T01:00:00Z"));
  await page.route(
    "**/api/events/gothic/2026-09-08-untimed-alpha-000000000001",
    async (route) => {
      const response = await route.fetch();
      const data = await response.json();
      data.age_policy =
        "All ages, ticketed guests under 16 ONLY ADMITTED WITH TICKETED GUARDIAN 21+";
      data.age_policy_url = "https://example.com/age-policy";
      await route.fulfill({ json: data });
    },
  );
  await page.context().route("https://example.com/age-policy", (route) =>
    route.fulfill({
      contentType: "text/html",
      body: "<h1>Policy fixture</h1>",
    }),
  );
  await page.goto("/events/gothic/2026-09-08-untimed-alpha-000000000001");
  const row = page
    .getByRole("dialog")
    .locator("dt")
    .filter({ hasText: /^Age Policy$/ })
    .locator("+ dd");
  await expect(row).toHaveText(
    "All ages, ticketed guests under 16 only admitted with ticketed guardian 21+",
    { useInnerText: true },
  );
  const link = row.getByRole("link", {
    name: "Venue admission policy",
    exact: true,
  });
  expect(await link.innerText()).toBe("");
  await expect(link).toHaveAttribute("target", "_blank");
  await expect(link).toHaveAttribute("rel", "noopener noreferrer");
  await expect(link.locator("svg")).toHaveAttribute("aria-hidden", "true");
  await link.focus();
  await expect(row.getByRole("tooltip")).toHaveText("Venue admission policy");
  await page.screenshot({
    path: `test-results/plain-policy-${info.project.name}.png`,
  });
  const popupPromise = page.waitForEvent("popup");
  await link.click();
  const popup = await popupPromise;
  await expect(popup.getByRole("heading")).toHaveText("Policy fixture");
  await expect(page.getByRole("dialog")).toBeVisible();
  await popup.close();
});
