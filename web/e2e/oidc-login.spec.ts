import { test, expect } from "@playwright/test";

// Full OIDC BFF flow via Dex (plain HTTP):
// 1. POST /api/auth/login → Go backend redirects to Dex (localhost:5556)
// 2. User enters credentials on Dex login page
// 3. Dex redirects back to /api/auth/callback
// 4. Go backend exchanges code for tokens, creates session
// 5. Browser ends up at / with a valid session cookie
//
// Prerequisites (handled by `make test-e2e-ui`):
// - Dex port-forwarded to localhost:5556
// - solar-ui backend running on :8090 with OIDC configured
// - DEX_LOCAL_PORT env var set to 5556
// - Static user: admin@solar.local / password

test.describe("OIDC login flow", () => {
  // Skip if DEX_LOCAL_PORT is not set (Dex is not port-forwarded)
  const dexPort = process.env.DEX_LOCAL_PORT;

  test("should redirect to Dex on login", async ({ page }) => {
    const loginResponse = await page.request.post("/api/auth/login", {
      maxRedirects: 0,
    });
    expect(loginResponse.status()).toBe(302);

    const location = loginResponse.headers()["location"];
    expect(location).toContain("localhost");
    expect(location).toContain("client_id=solar-ui");
    expect(location).toContain("response_type=code");
  });

  test("should complete full login via Dex", async ({ page }) => {
    if (!dexPort) {
      test.skip();

      return;
    }

    // Verify we start unauthenticated
    await page.goto("/");
    const meBefore = await page.request.get("/api/auth/me");
    expect((await meBefore.json()).authenticated).toBe(false);

    // Initiate OIDC login — this redirects to Dex
    await page.goto("/api/auth/login");

    // We should now be on the Dex login page
    // Dex redirects through /auth → /auth/local/login for the password connector
    await page.waitForURL(/localhost.*5556/, { timeout: 10_000 });

    // Fill in the login form
    await page.fill('input[name="login"]', "admin@solar.local");
    await page.fill('input[name="password"]', "password");
    await page.click('button[type="submit"]');

    // After successful auth, Dex redirects back to /api/auth/callback,
    // which sets the session and redirects to /
    await page.waitForURL("http://localhost:8090/", { timeout: 15_000 });

    // Verify we are authenticated
    const meAfter = await page.request.get("/api/auth/me");
    const meBody = await meAfter.json();
    expect(meBody.authenticated).toBe(true);
    expect(meBody.username).toBe("admin@solar.local");

    // Verify the UI shows the username
    await expect(page.locator("text=admin@solar.local")).toBeVisible();

    // Verify API calls work with the session
    const targets = await page.request.get(
      "/api/namespaces/default/targets",
    );
    expect(targets.status()).toBe(200);
    expect(await targets.json()).toHaveProperty("items");

    // Logout
    await page.goto("/api/auth/logout");
    const meLogout = await page.request.get("/api/auth/me");
    expect((await meLogout.json()).authenticated).toBe(false);
  });
});
