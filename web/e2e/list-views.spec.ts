import { test, expect, Page } from "@playwright/test";

// Verifies that each list-view page talks to the backend successfully:
// 1. The authenticated API returns a valid ResourceList.
// 2. The UI resolves without showing an error state.

const RESOURCES = [
  { name: "targets", path: "/targets", heading: "Targets" },
  { name: "releases", path: "/releases", heading: "Releases" },
  { name: "components", path: "/components", heading: "Components" },
  { name: "profiles", path: "/profiles", heading: "Profiles" },
  { name: "registries", path: "/registries", heading: "Registries" },
] as const;

test.describe("List views — API → UI", () => {
  for (const { name, path } of RESOURCES) {
    test(`GET /api/namespaces/default/${name} returns a valid list`, async ({
      request,
    }) => {
      const res = await request.get(`/api/namespaces/default/${name}`);
      expect(res.status()).toBe(200);

      const body = await res.json();
      expect(body).toHaveProperty("items");
      expect(Array.isArray(body.items)).toBe(true);
    });
  }

  for (const { name, path, heading } of RESOURCES) {
    test(`${heading} page loads without error`, async ({ page }) => {
      const listResponse = page.waitForResponse(
        (res) =>
          res.url().includes(`/api/namespaces/default/${name}`) &&
          res.request().method() === "GET",
      );
      await page.goto(path);
      await listResponse;
      await expect(page.getByRole("heading", { name: heading })).toBeVisible();

      // Error state must not be present.
      await expect(
        page.getByText(`Failed to load ${name}`),
      ).not.toBeVisible();
    });
  }
});

// Spot-check: all-namespaces list endpoint also works.
test.describe("List views — all-namespaces variant", () => {
  for (const { name } of RESOURCES) {
    test(`GET /api/${name} returns a valid list`, async ({ request }) => {
      const res = await request.get(`/api/${name}`);
      expect(res.status()).toBe(200);

      const body = await res.json();
      expect(body).toHaveProperty("items");
      expect(Array.isArray(body.items)).toBe(true);
    });
  }
});
