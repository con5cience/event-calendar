import { expect, test } from "@playwright/test";

test("Paramount event-specific admission reaches filters, details, and export", async ({
  page,
  request,
}) => {
  const base = process.env.KSE_BASE_URL;
  test.skip(!base, "Requires explicit Paramount publication");
  const response = await request.get(`${base}/api/calendar`);
  expect(response.status()).toBe(200);
  const { events } = await response.json();
  const records = events.filter(
    (e: { venue: string }) => e.venue === "Paramount Theatre",
  );
  for (const [policy, ages] of [
    [
      "12+",
      [
        [11, false],
        [12, true],
        [14, true],
        [17, true],
      ],
    ],
    [
      "15+",
      [
        [14, false],
        [15, true],
      ],
    ],
    [
      "16+",
      [
        [14, false],
        [16, true],
      ],
    ],
    [
      "18+",
      [
        [14, false],
        [17, false],
      ],
    ],
    [
      "Under 14 must be accompanied by a person aged 18+",
      [
        [0, true],
        [13, true],
        [14, true],
        [17, true],
      ],
    ],
  ] as const) {
    const event = records.find(
      (e: { age_policy?: string }) => e.age_policy === policy,
    );
    expect(event).toBeTruthy();
    expect(event).not.toHaveProperty("price");
    await page.goto(base + event.public_path);
    const dialog = page.getByRole("dialog");
    await expect(dialog).toContainText("Paramount Theatre");
    await expect(dialog).toContainText(policy);
    await expect(
      dialog.getByRole("link", { name: "View Event", exact: true }),
    ).toHaveAttribute("href", event.event_url);
    await expect(
      dialog.getByRole("link", { name: "Buy Tickets", exact: true }),
    ).toHaveAttribute("href", event.ticket_url);
    if (event.with_adult) expect(event.with_adult.url).toBe(event.event_url);
    const exported = await request.get(`${base}/api${event.public_path}.ics`);
    expect(exported.status()).toBe(200);
    expect(await exported.text()).toContain("BEGIN:VEVENT");
    await page.getByRole("button", { name: "Close event details" }).click();
    for (const [age, visible] of ages) {
      await page
        .getByRole("checkbox", { name: "With Adult", exact: true })
        .check();
      await page
        .getByRole("combobox", { name: "Child’s age" })
        .selectOption(String(age));
      const card = page
        .getByTestId(`event-${event.id}`)
        .filter({ visible: true });
      if (visible) await expect(card).toBeVisible();
      else await expect(card).toHaveCount(0);
    }
  }
  const unknown = records.find((e: { age_policy?: string }) =>
    e.age_policy?.startsWith("Age restrictions vary"),
  );
  expect(unknown).toBeTruthy();
  expect(unknown.with_adult).toBeUndefined();
  expect(unknown.age_category).toBeUndefined();
  const recommendation = records.find((e: { age_policy?: string }) =>
    e.age_policy?.toLowerCase().startsWith("recommended"),
  );
  expect(recommendation).toBeTruthy();
  expect(recommendation.with_adult).toBeUndefined();
  expect(recommendation.age_category).toBeUndefined();
});
