import { test, expect } from "@playwright/test";

test.describe("Impersonation — unauthenticated access", () => {
  // storageState: undefined overrides the project-level stored session so
  // these contexts have no cookies and are treated as unauthenticated.

  test("GET /api/auth/impersonation-targets returns 401 when not logged in", async ({
    browser,
  }) => {
    const ctx = await browser.newContext({ storageState: undefined });
    const response = await ctx.request.get("/api/auth/impersonation-targets");
    expect(response.status()).toBe(401);
    await ctx.close();
  });

  test("PUT /api/auth/impersonate returns 401 when not logged in", async ({
    browser,
  }) => {
    const ctx = await browser.newContext({ storageState: undefined });
    const response = await ctx.request.put("/api/auth/impersonate", {
      data: { username: "maintainer@solar.local" },
    });
    expect(response.status()).toBe(401);
    await ctx.close();
  });
});

test.describe("Impersonation — admin user", () => {
  // Session from auth.setup.ts: logged in as admin@solar.local (isAdmin=true).

  test.afterEach(async ({ request }) => {
    // Best-effort: ensure no test leaves the shared session impersonating.
    await request.delete("/api/auth/impersonate").catch(() => {});
  });

  test("GET /api/auth/me reports isAdmin=true for admin user", async ({
    request,
  }) => {
    const response = await request.get("/api/auth/me");
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body.authenticated).toBe(true);
    expect(body.username).toBe("admin@solar.local");
    expect(body.isAdmin).toBe(true);
  });

  test("GET /api/auth/impersonation-targets returns available personas", async ({
    request,
  }) => {
    const response = await request.get("/api/auth/impersonation-targets");
    expect(response.status()).toBe(200);
    const targets: { username: string; groups: string[] }[] =
      await response.json();
    expect(Array.isArray(targets)).toBe(true);
    expect(targets.length).toBeGreaterThan(0);

    const usernames = targets.map((t) => t.username);
    expect(usernames).toContain("maintainer@solar.local");
    expect(usernames).toContain("coordinator@solar.local");
  });

  test("can activate impersonation, sees persona permissions, then clear it", async ({
    request,
  }) => {
    // Activate impersonation as coordinator
    const impersonate = await request.put("/api/auth/impersonate", {
      data: { username: "coordinator@solar.local" },
    });
    expect(impersonate.status()).toBe(204);

    // /auth/me should reflect impersonating state
    const meImpersonating = await request.get("/api/auth/me");
    expect(meImpersonating.status()).toBe(200);
    const meBody = await meImpersonating.json();
    expect(meBody.impersonating?.username).toBe("coordinator@solar.local");
    expect(meBody.impersonating?.groups).toContain("coordinator");
    // isAdmin must still reflect the real admin identity, not the persona's
    expect(meBody.isAdmin).toBe(true);

    // Permissions endpoint should return the coordinator's rules only
    const perms = await request.get("/api/namespaces/default/permissions");
    expect(perms.status()).toBe(200);
    const { rules } = await perms.json();
    const solarRules = (
      rules as { apiGroups: string[]; resources: string[]; verbs: string[] }[]
    ).filter((r) => r.apiGroups.includes("solar.opendefense.cloud"));
    // coordinator has get/list/watch on targets, releases, releasebindings, profiles
    expect(
      solarRules.some((r) => r.resources.includes("targets")),
    ).toBe(true);
    // coordinator does NOT have write verbs on any solar resource
    const hasWrite = solarRules.some((r) =>
      r.verbs.some((v) => ["create", "update", "patch", "delete"].includes(v)),
    );
    expect(hasWrite).toBe(false);

    // Clear impersonation
    const clear = await request.delete("/api/auth/impersonate");
    expect(clear.status()).toBe(204);

    // /auth/me should no longer show impersonating
    const meCleared = await request.get("/api/auth/me");
    const meClearedBody = await meCleared.json();
    expect(meClearedBody.impersonating).toBeUndefined();
  });

  test("PUT /api/auth/impersonate rejects unknown username", async ({
    request,
  }) => {
    const response = await request.put("/api/auth/impersonate", {
      data: { username: "nobody@solar.local" },
    });
    expect(response.status()).toBe(400);
  });

  test("PUT /api/auth/impersonate rejects missing username", async ({
    request,
  }) => {
    const response = await request.put("/api/auth/impersonate", {
      data: {},
    });
    expect(response.status()).toBe(400);
  });
});
