// web/e2e/detail-views.spec.ts
import { test, expect } from "@playwright/test";

test.describe("resource detail views", () => {
  test("target detail shows conditions and navigates to a related release", async ({
    page,
  }) => {
    await page.goto("/targets");
    await expect(
      page.getByRole("heading", { name: "Targets" }),
    ).toBeVisible();

    // Target list rows are full-width card buttons.
    const rows = page.locator("main button.bg-card");
    const count = await rows.count();
    test.skip(count === 0, "no targets seeded in the dev cluster");

    await rows.first().click();

    // Landed on a target detail page.
    await expect(page).toHaveURL(/\/targets\/[^/]+\/[^/]+/);
    await expect(
      page.getByRole("heading", { name: "Conditions" }),
    ).toBeVisible();
    // The conditions table only renders when the target has conditions;
    // otherwise ConditionsTable shows a "No conditions" message instead.
    if ((await page.getByRole("table").count()) > 0) {
      await expect(
        page.getByRole("columnheader", { name: "Last Transition" }),
      ).toBeVisible();
    }

    // If a release is bound, the link navigates to the release detail page.
    const releaseLink = page.locator('a[href^="/releases/"]').first();
    if ((await releaseLink.count()) > 0) {
      await releaseLink.click();
      await expect(page).toHaveURL(/\/releases\/[^/]+\/[^/]+/);
      await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    }
  });
});
