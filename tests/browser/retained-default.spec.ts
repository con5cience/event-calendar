import { expect, test } from "@playwright/test";

test("retained past events remain visible without a toggle or timezone notice", async ({
  page,
}) => {
  await page.clock.setFixedTime(new Date("2026-09-10T01:00:00Z"));
  await page.addInitScript(() => {
    if (!localStorage.getItem("event-calendar.filters.v1"))
      localStorage.setItem(
        "event-calendar.filters.v1",
        JSON.stringify({
          venues: ["Gothic Theatre"],
          ages: [],
          query: "untimed",
          includePast: false,
        }),
      );
  });
  await page.goto("/");
  for (let pass = 0; pass < 2; pass++) {
    await expect(page.getByRole("contentinfo")).toHaveCount(0);
    await expect(
      page.getByTestId("event-gothic-000000000001").filter({ visible: true }),
    ).toBeVisible();
    await expect(
      page.getByRole("checkbox", { name: "Include past events" }),
    ).toHaveCount(0);
    await expect(page.locator('input[type="search"]')).toHaveValue("untimed");
    await expect(
      page.getByRole("button", { name: "Venues: Gothic Theatre", exact: true }),
    ).toBeVisible();
    await page.reload();
  }
  await page.getByRole("searchbox", { name: "Search events" }).fill("");
  await page.getByRole("button", { name: /^Venues:/ }).click();
  await page
    .getByRole("group", { name: "Venues", exact: true })
    .getByRole("checkbox", { name: "All", exact: true })
    .check();
  await page.keyboard.press("Escape");
  await expect(
    page.getByTestId("event-gothic-000000000001").filter({ visible: true }),
  ).toBeVisible();
  await expect(page.getByRole("searchbox")).toHaveValue("");
});
