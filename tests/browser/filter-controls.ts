import { expect, type Page } from "@playwright/test";

// Admission checks need the full day, not either layout's Week overflow limit.
export async function openPhoneEventDay(page: Page, date: string) {
  return openEventDay(page, date, true);
}

export async function openEventDay(page: Page, date: string, mobile: boolean) {
  const label = new Intl.DateTimeFormat("en-US", {
    dateStyle: "long",
    timeZone: "UTC",
  }).format(new Date(`${date}T12:00:00Z`));
  await page
    .getByRole(mobile ? "listitem" : "columnheader", {
      name: label,
      exact: true,
    })
    .getByRole("link", { name: /^Go to/ })
    .click();
  await expect(
    page.getByRole("button", { name: "Day", exact: true }),
  ).toHaveAttribute("aria-pressed", "true");
}
