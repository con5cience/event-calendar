import { expect, test } from "@playwright/test";

test.beforeEach(async ({ page }) => {
  await page.clock.setFixedTime(new Date("2026-09-09T01:00:00Z"));
});

test("venue typing focuses matching names without changing selection", async ({
  page,
}) => {
  await page.goto("/");
  const trigger = page.getByRole("button", { name: /^Venues:/ });
  await trigger.click();
  const group = page.getByRole("group", { name: "Venues", exact: true });
  const boxes = group.getByRole("checkbox");
  const names = await group.locator("label").allTextContents();
  const matching = names
    .map((name, index) => ({ name: name.trim(), index }))
    .filter(({ name }) => name.toLowerCase().startsWith("g"));
  expect(matching.length).toBeGreaterThan(0);
  await page.keyboard.press("g");
  await expect(boxes.nth(matching[0].index)).toBeFocused();
  await page.keyboard.press("g");
  await expect(boxes.nth(matching[1 % matching.length].index)).toBeFocused();
  await expect(trigger).toHaveText("All venues");
  await page.keyboard.press("Escape");
  await trigger.click();
  await page.keyboard.type("mission");
  const mission = group.getByRole("checkbox", {
    name: "Mission Ballroom",
    exact: true,
  });
  await expect(mission).toBeFocused();
  await page.keyboard.press("z");
  await expect(mission).toBeFocused();
  await page.keyboard.press("Space");
  await expect(mission).toBeChecked();
  await expect(trigger).toHaveText("Mission Ballroom");
  await page.keyboard.press("Escape");
  await page.reload();
  await expect(trigger).toHaveText("Mission Ballroom");
});

for (const [name, option] of [["Venues", "Gothic Theatre"]]) {
  test(`${name} dropdown labels and row padding toggle without closing`, async ({
    page,
  }, info) => {
    await page.goto("/");
    const trigger = page.getByRole("button", { name: new RegExp(`^${name}:`) });
    await trigger.click();
    const group = page.getByRole("group", { name, exact: true });
    const box = group.getByRole("checkbox", { name: option, exact: true });
    const row = group.locator("label").filter({
      has: page.getByRole("checkbox", { name: option, exact: true }),
    });
    const activate = async (position: { x: number; y: number }) => {
      if (info.project.name === "phone") await row.tap({ position });
      else await row.click({ position });
    };
    // Text is beyond the checkbox; left padding is outside its hit area.
    await activate({ x: 55, y: 22 });
    await expect(box).toBeChecked();
    await expect(trigger).toHaveAttribute("aria-expanded", "true");
    await activate({ x: 3, y: 22 });
    await expect(box).not.toBeChecked();
    await expect(trigger).toHaveAttribute("aria-expanded", "true");
    await activate({ x: 55, y: 22 });
    const allRow = group.locator("label").filter({ hasText: /^All$/ });
    if (info.project.name === "phone")
      await allRow.tap({ position: { x: 55, y: 22 } });
    else await allRow.click({ position: { x: 55, y: 22 } });
    await expect(box).not.toBeChecked();
    await expect(
      group.getByRole("checkbox", { name: "All", exact: true }),
    ).toBeChecked();
    await expect(trigger).toHaveAttribute("aria-expanded", "true");
    await box.focus();
    await page.keyboard.press("Space");
    await expect(box).toBeChecked();
    await page.keyboard.press("Escape");
    await expect(trigger).toBeFocused();
    await expect(group).toBeHidden();
    await trigger.click();
    await page.getByTestId("range").click();
    await expect(group).toBeHidden();
  });
}

test("retired age and cost selections cannot hide events", async ({ page }) => {
  await page.addInitScript(() => {
    if (!localStorage.getItem("event-calendar.filters.v1")) {
      localStorage.setItem(
        "event-calendar.filters.v1",
        JSON.stringify({
          venues: [],
          ages: ["21+"],
          costs: ["$$$$"],
          query: "",
          includePast: false,
        }),
      );
    }
  });
  await page.goto("/");
  await expect(page.getByRole("button", { name: /^Cost:/ })).toHaveCount(0);
  await expect(
    page.getByTestId("event-gothic-000000000001").filter({ visible: true }),
  ).toBeVisible();
  await page.reload();
  await expect(
    page.getByTestId("event-gothic-000000000001").filter({ visible: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Week", exact: true }).click();
  await page
    .getByTestId("event-mission-000000000003")
    .filter({ visible: true })
    .click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await expect(page.getByRole("dialog")).not.toContainText("$20");
});

test("show-only metadata is labeled Show and artist breaks time and venue ties", async ({
  page,
}) => {
  await page.route("**/api/calendar", async (route) => {
    const response = await route.fetch();
    const data = await response.json();
    data.events.find(
      (e: { id: string }) => e.id === "mission-000000000005",
    ).artist = "Zulu";
    data.events.find(
      (e: { id: string }) => e.id === "mission-000000000006",
    ).artist = "Alpha";
    await route.fulfill({ response, json: data });
  });
  await page.goto("/");
  await page.getByRole("button", { name: "Day", exact: true }).click();
  await expect(
    page.getByTestId("event-mission-000000000006").filter({ visible: true }),
  ).toBeVisible();
  const cards = await page.locator(".event-card:visible").allTextContents();
  expect(cards.findIndex((t) => t.includes("Cedar"))).toBeLessThan(
    cards.findIndex((t) => t.includes("Birch")),
  );
  await page
    .getByTestId("event-gothic-000000000004")
    .filter({ visible: true })
    .click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.locator("dt")).toContainText(["Show"]);
  await expect(dialog.getByText("Doors", { exact: true })).toHaveCount(0);
  await dialog.getByRole("button", { name: "Close event details" }).click();
  await page
    .getByTestId("event-mission-000000000003")
    .filter({ visible: true })
    .click();
  await expect(dialog.getByText("Doors", { exact: true })).toBeVisible();
  await dialog.getByRole("button", { name: "Close event details" }).click();
  await page
    .getByTestId("event-mission-000000000006")
    .filter({ visible: true })
    .click();
  await expect(
    dialog.getByRole("heading", { name: "2026-09-08 : Cedar", exact: true }),
  ).toBeVisible();
  await expect(
    dialog.locator("dt").filter({ hasText: /^Artist$/ }),
  ).toHaveCount(0);
  await expect(dialog.locator("dt").first()).toHaveText("Venue");
  await expect(dialog.locator("dt").nth(1)).toHaveText("Status");
});

test("venue and search filters combine, preserve navigation, and survive reload", async ({
  page,
}, info) => {
  await page.goto("/");
  const search = page.getByRole("searchbox", { name: "Search events" });
  await expect(search).toBeVisible();
  const range = await page.getByTestId("range").textContent();
  await page
    .getByRole("button", { name: "Venues: All venues", exact: true })
    .click();
  await expect(
    page.getByRole("checkbox", { name: "All", exact: true }),
  ).toBeChecked();
  await page
    .getByRole("checkbox", { name: "Mission Ballroom", exact: true })
    .check();
  await expect(
    page.getByRole("checkbox", { name: "All", exact: true }),
  ).not.toBeChecked();
  await expect(page.getByTestId("event-gothic-000000000001")).toHaveCount(0);
  await search.fill("EARLY mission");
  await expect(
    page.getByTestId("event-mission-000000000003").filter({ visible: true }),
  ).toBeVisible();
  await expect(page.locator(".event-card:visible")).toHaveCount(1);
  await expect(page.getByTestId("range")).toHaveText(range!);
  await expect(page).toHaveURL("/");
  await page.reload();
  await expect(search).toHaveValue("EARLY mission");
  await page
    .getByRole("button", { name: "Venues: Mission Ballroom", exact: true })
    .click();
  await expect(
    page.getByRole("checkbox", { name: "Mission Ballroom", exact: true }),
  ).toBeChecked();
  await expect(
    page.getByRole("checkbox", { name: "Include past events" }),
  ).toHaveCount(0);
  await expect(page.locator(".event-card:visible")).toHaveCount(1);
  await search.fill("no-such-event");
  await expect(
    page.getByText("No events match your filters.", { exact: true }).first(),
  ).toBeVisible();
  await page.getByRole("searchbox", { name: "Search events" }).fill("");
  await page.getByRole("button", { name: /^Venues:/ }).click();
  await page
    .getByRole("group", { name: "Venues", exact: true })
    .getByRole("checkbox", { name: "All", exact: true })
    .check();
  await page.keyboard.press("Escape");
  await expect(page.locator('input[type="search"]')).toHaveValue("");
  await page
    .getByRole("button", { name: "Venues: All venues", exact: true })
    .click();
  await expect(
    page.getByRole("checkbox", { name: "All", exact: true }),
  ).toBeChecked();
  await expect(
    page.getByRole("checkbox", { name: "Include past events" }),
  ).toHaveCount(0);
  await page
    .getByRole("checkbox", { name: "Gothic Theatre", exact: true })
    .check();
  await page
    .getByRole("checkbox", { name: "Mission Ballroom", exact: true })
    .check();
  await expect(
    page.getByTestId("event-gothic-000000000001").filter({ visible: true }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: "Week", exact: true }).click();
  await expect(
    page.getByTestId("event-mission-000000000003").filter({ visible: true }),
  ).toBeVisible();
  await expect(page.locator('[data-testid^="event-hq-"]')).toHaveCount(0);
  await page.screenshot({
    path: `test-results/${info.project.name}-filters.png`,
    fullPage: true,
  });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
});

test("venue dropdown supports summaries, keyboard, outside dismissal, and saved selections", async ({
  page,
}, info) => {
  await page.goto("/");
  const trigger = page.getByRole("button", { name: /^Venues:/ });
  const all = page.getByRole("checkbox", { name: "All", exact: true });
  await expect(trigger).toHaveText("All venues");
  await expect(trigger).toHaveAttribute("aria-expanded", "false");
  await expect(all).toBeHidden();
  await trigger.focus();
  await page.keyboard.press("Enter");
  await expect(trigger).toHaveAttribute("aria-expanded", "true");
  await page.keyboard.press("Tab");
  await expect(all).toBeFocused();
  await page
    .getByRole("checkbox", { name: "Gothic Theatre", exact: true })
    .check();
  await expect(trigger).toHaveText("Gothic Theatre");
  await page
    .getByRole("checkbox", { name: "Mission Ballroom", exact: true })
    .check();
  await expect(trigger).toHaveText("2 venues selected");
  await expect(all).toBeVisible();
  await page.screenshot({
    path: `test-results/${info.project.name}-venue-dropdown.png`,
    fullPage: true,
  });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
  await page.keyboard.press("Escape");
  await expect(all).toBeHidden();
  await expect(trigger).toBeFocused();
  await trigger.click();
  await page.getByTestId("range").click();
  await expect(all).toBeHidden();
  await page.reload();
  await expect(trigger).toHaveText("2 venues selected");
  await expect(trigger).toHaveAttribute("aria-expanded", "false");
  await trigger.click();
  await expect(
    page.getByRole("checkbox", { name: "Gothic Theatre", exact: true }),
  ).toBeChecked();
  await expect(
    page.getByRole("checkbox", { name: "Mission Ballroom", exact: true }),
  ).toBeChecked();
  await all.check();
  await expect(trigger).toHaveText("All venues");
  await trigger.click();
  await expect(all).toBeHidden();
  await trigger.click();
  await page.getByRole("checkbox", { name: "Warehouse", exact: true }).focus();
  await page.keyboard.press("Tab");
  await expect(all).toBeHidden();
  await expect(
    page.getByRole("checkbox", { name: "With Adult", exact: true }),
  ).toBeFocused();
});

test("past events remain visible while search includes later weeks without navigating", async ({
  page,
}) => {
  await page.clock.setFixedTime(new Date("2026-09-10T01:00:00Z")); // September 9 Denver.
  await page.goto("/");
  await page.getByRole("button", { name: "Month", exact: true }).click();
  const past = page
    .getByTestId("event-gothic-000000000001")
    .filter({ visible: true });
  await expect(past).toBeVisible();
  await expect(
    page.getByRole("checkbox", { name: "Include past events" }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "Previous", exact: true }).click();
  const range = await page.getByTestId("range").textContent();
  await page.getByRole("searchbox", { name: "Search events" }).fill("gothic");
  await expect(page.getByTestId("range")).toHaveText(range!);
  await page.getByRole("button", { name: "Next", exact: true }).click();
  await expect(
    page.getByTestId("event-gothic-000000000015").filter({ visible: true }),
  ).toBeVisible();
});

test("blocked storage does not prevent filtering or loading", async ({
  page,
}) => {
  await page.addInitScript(() => {
    Object.defineProperty(window, "localStorage", {
      get() {
        throw new Error("storage denied");
      },
    });
  });
  await page.goto("/");
  await page
    .getByRole("searchbox", { name: "Search events" })
    .fill("early mission");
  await expect(page.locator(".event-card:visible")).toHaveCount(1);
  await page.reload();
  await expect(
    page.getByRole("searchbox", { name: "Search events" }),
  ).toHaveValue("");
});
