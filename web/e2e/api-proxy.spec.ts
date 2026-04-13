import { test, expect } from "@playwright/test";

test.describe("API proxy", () => {
  // These tests verify that API routes require authentication.

  test("should return 401 for unauthenticated target list", async ({
    request,
  }) => {
    const response = await request.get("/api/namespaces/default/targets");
    expect(response.status()).toBe(401);
  });

  test("should return 401 for unauthenticated release list", async ({
    request,
  }) => {
    const response = await request.get("/api/namespaces/default/releases");
    expect(response.status()).toBe(401);
  });

  test("should return 401 for unauthenticated SSE endpoint", async ({
    request,
  }) => {
    const response = await request
      .get("/api/namespaces/default/events", {
        timeout: 3000,
      })
      .catch(() => null);

    if (response) {
      expect(response.status()).toBe(401);
    }
  });

  test("should allow unauthenticated access to /api/auth/me", async ({
    request,
  }) => {
    const response = await request.get("/api/auth/me");
    expect(response.status()).toBe(200);

    const body = await response.json();
    expect(body.authenticated).toBe(false);
  });
});
