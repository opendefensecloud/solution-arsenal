import { test as setup, expect } from "@playwright/test";

setup("authenticate via Dex", async ({ browser }) => {
  const context = await browser.newContext({ ignoreHTTPSErrors: true });
  const page = await context.newPage();

  // Initiate OIDC login
  await page.goto("/api/auth/login");

  // Wait for Dex login page
  await page.waitForURL(/localhost.*5556/, { timeout: 10_000 });

  // Fill in credentials
  await page.fill('input[name="login"]', "admin@solar.local");
  await page.fill('input[name="password"]', "password");
  await page.click('button[type="submit"]');

  // Wait for redirect back to the app
  await page.waitForURL("http://localhost:8090/", { timeout: 15_000 });

  // Verify authentication
  const me = await page.request.get("/api/auth/me");
  expect((await me.json()).authenticated).toBe(true);

  // Save session state (cookies) for authenticated tests
  await context.storageState({ path: "e2e/.auth/session.json" });
  await context.close();
});
