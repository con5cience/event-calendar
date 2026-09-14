import { type Page } from "@playwright/test";

export async function selectCalendarView(page: Page, name: string) {
  await page.locator(".view-picker").waitFor({ state: "visible" });
  const trigger = page.getByRole("button", { name: "Choose calendar view" });
  if (await trigger.isVisible()) {
    if ((await trigger.getAttribute("aria-expanded")) !== "true")
      await trigger.click();
  }
  await page.getByRole("button", { name, exact: true }).click();
}
