import { test, expect } from "@playwright/test";

test.describe("SPA serving", () => {
  test("should load the dashboard page", async ({ page }) => {
    await page.goto("/");
    await expect(
      page.getByRole("heading", { name: "Dashboard" }),
    ).toBeVisible();
  });

  test("should handle client-side routing to /targets", async ({ page }) => {
    await page.goto("/targets");
    await expect(
      page.getByRole("heading", { name: "Targets" }),
    ).toBeVisible();
  });

  test("should handle client-side routing to /releases", async ({ page }) => {
    await page.goto("/releases");
    await expect(
      page.getByRole("heading", { name: "Releases" }),
    ).toBeVisible();
  });

  test("should handle client-side routing to /components", async ({
    page,
  }) => {
    await page.goto("/components");
    await expect(
      page.getByRole("heading", { name: "Components" }),
    ).toBeVisible();
  });

  test("should handle client-side routing to /profiles", async ({ page }) => {
    await page.goto("/profiles");
    await expect(
      page.getByRole("heading", { name: "Profiles" }),
    ).toBeVisible();
  });

  test("should navigate between pages via sidebar", async ({ page }) => {
    await page.goto("/");

    // Click on Targets in the sidebar
    await page.click('a[href="/targets"]');
    await expect(page).toHaveURL(/\/targets/);
    await expect(
      page.getByRole("heading", { name: "Targets" }),
    ).toBeVisible();

    // Click on Releases
    await page.click('a[href="/releases"]');
    await expect(page).toHaveURL(/\/releases/);
    await expect(
      page.getByRole("heading", { name: "Releases" }),
    ).toBeVisible();

    // Click on Dashboard
    await page.click('a[href="/"]');
    await expect(page).toHaveURL(/\/$/);
    await expect(
      page.getByRole("heading", { name: "Dashboard" }),
    ).toBeVisible();
  });
});
