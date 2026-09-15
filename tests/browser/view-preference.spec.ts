import { selectCalendarView } from "./view-controls";
import { expect, test } from "@playwright/test";

test("invalid or unavailable storage falls back to Week without preventing changes", async ({
  page,
}) => {
  await page.addInitScript(() =>
    localStorage.setItem("event-calendar.view.v1", "invalid"),
  );
  await page.goto("/");
  await expect(
    page.getByRole("button", {
      name: "Week",
      exact: true,
      includeHidden: true,
    }),
  ).toHaveAttribute("aria-pressed", "true");
  await page.addInitScript(() => {
    Object.defineProperty(window, "localStorage", {
      get() {
        throw new Error("denied");
      },
    });
  });
  await page.reload();
  await expect(
    page.getByRole("button", {
      name: "Week",
      exact: true,
      includeHidden: true,
    }),
  ).toHaveAttribute("aria-pressed", "true");
  await selectCalendarView(page, "Month");
  await expect(
    page.getByRole("button", {
      name: "Month",
      exact: true,
      includeHidden: true,
    }),
  ).toHaveAttribute("aria-pressed", "true");
  await page.reload();
  await expect(
    page.getByRole("button", {
      name: "Week",
      exact: true,
      includeHidden: true,
    }),
  ).toHaveAttribute("aria-pressed", "true");
});

test("direct event links do not replace a saved Month preference", async ({
  page,
  request,
}) => {
  const response = await request.get("/api/calendar");
  const data = await response.json();
  const event = data.events.find(
    (item: { public_path?: string }) => item.public_path,
  );
  await page.goto("/");
  await selectCalendarView(page, "Month");
  await page.goto(event.public_path);
  await expect(
    page.getByRole("button", { name: "Close event details" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Close event details" }).click();
  await expect(page).toHaveURL(/\/\?view=week&date=\d{4}-\d{2}-\d{2}$/);
  expect(
    await page.evaluate(() => localStorage.getItem("event-calendar.view.v1")),
  ).toBe("month");
  await expect(
    page.getByRole("button", {
      name: "Week",
      exact: true,
      includeHidden: true,
    }),
  ).toHaveAttribute("aria-pressed", "true");
  await page.reload();
  await expect(
    page.getByRole("button", {
      name: "Month",
      exact: true,
      includeHidden: true,
    }),
  ).toHaveAttribute("aria-pressed", "true");
});

test("Week is the default and explicit view selections survive reload", async ({
  page,
}) => {
  await page.goto("/");
  await expect(
    page.getByRole("button", {
      name: "Week",
      exact: true,
      includeHidden: true,
    }),
  ).toHaveAttribute("aria-pressed", "true");
  for (const name of ["Month", "Day", "Week"]) {
    await selectCalendarView(page, name);
    await page.reload();
    await expect(
      page.getByRole("button", { name, exact: true, includeHidden: true }),
    ).toHaveAttribute("aria-pressed", "true");
  }
});

test("toolbar search is outside the drawer and fits beside or below view controls", async ({
  page,
}, info) => {
  await page.goto("/");
  const search = page.getByRole("searchbox", { name: "Search events" });
  await expect(search).toBeVisible();
  await expect(page.locator("dialog input[type=search]")).toHaveCount(0);
  const input = (await search.boundingBox())!;
  const picker = (await page.locator(".view-picker").boundingBox())!;
  if (info.project.name === "desktop") {
    expect(Math.abs(input.y - picker.y)).toBeLessThan(2);
    expect(input.x + input.width).toBeLessThan(picker.x);
  } else expect(input.y).toBeGreaterThanOrEqual(picker.y + picker.height);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await search.fill("early mission");
  await page.reload();
  await expect(search).toHaveValue("early mission");
  await search.fill("");
  await expect(search).toHaveValue("");
  await page.screenshot({
    path: `test-results/${info.project.name}-toolbar.png`,
  });
});

test("search icon appears only when empty and unfocused", async ({
  page,
}, info) => {
  await page.goto("/");
  const search = page.getByRole("searchbox", { name: "Search events" });
  const icon = page.locator('.toolbar-search [data-icon="magnifying-glass"]');
  expect(((await search.getAttribute("placeholder")) ?? "").trim()).toBe("");
  await expect(icon).toBeVisible();
  await expect(icon).toHaveAttribute("aria-hidden", "true");
  const inputBox = (await search.boundingBox())!;
  const iconBox = (await icon.boundingBox())!;
  expect(
    inputBox.x + inputBox.width - iconBox.x - iconBox.width,
  ).toBeGreaterThanOrEqual(10);
  expect(
    inputBox.x + inputBox.width - iconBox.x - iconBox.width,
  ).toBeLessThanOrEqual(16);
  expect(
    Math.abs(iconBox.y + iconBox.height / 2 - inputBox.y - inputBox.height / 2),
  ).toBeLessThan(1);
  // Clicking the decoration focuses the input rather than intercepting the click.
  await search.click({
    position: { x: inputBox.width - 18, y: inputBox.height / 2 },
  });
  await expect(search).toBeFocused();
  await expect(icon).toBeHidden();
  await search.fill("mission");
  await selectCalendarView(page, "Week");
  await expect(icon).toBeHidden();
  await page.reload();
  await expect(search).toHaveValue("mission");
  await expect(icon).toBeHidden();
  await search.fill("");
  await expect(icon).toBeHidden();
  await search.press("Tab");
  await expect(icon).toBeVisible();
  await page.screenshot({
    path: `test-results/${info.project.name}-search-icon.png`,
  });
});

test("Event Close button uses a centered symbol and retain accessible labels", async ({
  page,
}, info) => {
  await page.goto("/");
  await selectCalendarView(page, "Day");
  await page.locator(".event-card:visible").first().click();
  const modalClose = page.getByRole("button", { name: "Close event details" });
  await expect(modalClose).toHaveCSS("display", "grid");
  await expect(modalClose).toHaveCSS("place-items", "center");
  await page
    .getByRole("dialog")
    .screenshot({ path: `test-results/${info.project.name}-close-modal.png` });
});
