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

    // Verify we start unauthenticated — use the API directly. Do NOT navigate
    // the page to "/" first: the React app would fire auth-required queries
    // (namespace selector, etc.), receive 401s, and trigger client.ts's
    // window.location redirect to /api/auth/login. That redirect races with
    // the manual login navigation below and corrupts the OIDC state cookie.
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

    // Verify the UI shows the username. toBeVisible() auto-waits up to
    // the configured expect timeout (10s) — don't use waitForLoadState
    // "networkidle" here: the page opens a long-lived SSE EventSource so
    // the network never goes idle, and the wait would always time out.
    await expect(page.locator("text=admin@solar.local")).toBeVisible();

    // Verify API calls work with the session. Use a namespace where admin
    // can list — admin has cluster-admin so any namespace works; we use one
    // of the role namespaces seeded by hack/seed-demo-data.sh.
    const targets = await page.request.get(
      "/api/namespaces/k8s-cluster-user/targets",
    );
    expect(targets.status()).toBe(200);
    expect(await targets.json()).toHaveProperty("items");

    // Logout
    await page.goto("/api/auth/logout");
    const meLogout = await page.request.get("/api/auth/me");
    expect((await meLogout.json()).authenticated).toBe(false);
  });
});
