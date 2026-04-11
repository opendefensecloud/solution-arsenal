import { test, expect } from "@playwright/test";

test.describe("Authentication", () => {
  test("should show unauthenticated state initially", async ({ page }) => {
    await page.goto("/");

    // The user info section should not show a username
    // (no session cookie, so /api/auth/me returns authenticated: false)
    const response = await page.request.get("/api/auth/me");
    const body = await response.json();
    expect(body.authenticated).toBe(false);
  });

  test("should redirect to Dex on login", async ({ request }) => {
    const response = await request.post("/api/auth/login", {
      maxRedirects: 0,
    });

    expect(response.status()).toBe(302);
    const location = response.headers()["location"];
    expect(location).toContain("localhost");
    expect(location).toContain("client_id=solar-ui");
  });

  test("should clear session on logout", async ({ page }) => {
    await page.goto("/");

    // Hit logout endpoint
    const response = await page.request.get("/api/auth/logout", {
      maxRedirects: 0,
    });

    expect(response.status()).toBe(302);
    expect(response.headers()["location"]).toBe("/");
  });
});
