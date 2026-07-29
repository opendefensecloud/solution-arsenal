// web/e2e/detail-views.spec.ts
import { test, expect, type Page } from "@playwright/test";

const NS = "default";
const NOW = "2026-01-15T10:00:00.000Z";

const MOCK_AUTH = {
  authenticated: true,
  username: "admin@solar.local",
  groups: ["system:masters"],
  canImpersonate: false,
  canListAllNamespaces: false,
};

const MOCK_NAMESPACES = { items: [{ metadata: { name: NS } }] };

const RELEASE = {
  metadata: { name: "e2e-release", namespace: NS, creationTimestamp: NOW },
  spec: { componentVersionRef: { name: "e2e-component-v1" } },
  status: {
    conditions: [
      {
        type: "ComponentVersionResolved",
        status: "True",
        lastTransitionTime: NOW,
        reason: "Resolved",
        message: "",
      },
    ],
  },
};

const TARGET = {
  metadata: {
    name: "e2e-target",
    namespace: NS,
    creationTimestamp: NOW,
    labels: { env: "prod" },
  },
  spec: { renderRegistryRef: { name: "deploy-registry" } },
  status: {
    conditions: [
      {
        type: "ReleasesRendered",
        status: "True",
        lastTransitionTime: NOW,
        reason: "Rendered",
        message: "",
      },
      {
        type: "BootstrapReady",
        status: "True",
        lastTransitionTime: NOW,
        reason: "Ready",
        message: "",
      },
    ],
  },
};

const RELEASE_BINDING = {
  metadata: {
    name: "e2e-target-e2e-release",
    namespace: NS,
    creationTimestamp: NOW,
  },
  spec: {
    releaseRef: { name: RELEASE.metadata.name },
    targetRef: { name: TARGET.metadata.name },
  },
  status: { conditions: [] },
};

function list<T>(items: T[]) {
  return { items };
}

async function seedMocks(page: Page) {
  await page.addInitScript(() => {
    localStorage.setItem("solar-ui-selected-namespace", "default");
  });

  await page.route(
    (url) => url.pathname.startsWith("/api/"),
    (route) => {
      const { pathname: p } = new URL(route.request().url());

      if (p === "/api/auth/me") return route.fulfill({ json: MOCK_AUTH });
      if (p === "/api/namespaces")
        return route.fulfill({ json: MOCK_NAMESPACES });

      // SSE — return an empty stream so EventSource connects cleanly
      if (p.endsWith("/events"))
        return route.fulfill({
          status: 200,
          headers: {
            "Content-Type": "text/event-stream",
            "Cache-Control": "no-cache",
          },
          body: "",
        });

      // Detail routes
      if (p === `/api/namespaces/${NS}/targets/${TARGET.metadata.name}`)
        return route.fulfill({ json: TARGET });
      if (p === `/api/namespaces/${NS}/releases/${RELEASE.metadata.name}`)
        return route.fulfill({ json: RELEASE });

      // Namespace-scoped list routes
      if (p === `/api/namespaces/${NS}/targets`)
        return route.fulfill({ json: list([TARGET]) });
      if (p === `/api/namespaces/${NS}/releasebindings`)
        return route.fulfill({ json: list([RELEASE_BINDING]) });
      if (p === `/api/namespaces/${NS}/rendertasks`)
        return route.fulfill({ json: list([]) });
      if (p === `/api/namespaces/${NS}/registrybindings`)
        return route.fulfill({ json: list([]) });

      // All-namespace list routes → 403: the UI must pick a namespace first.
      if (
        p === "/api/targets" ||
        p === "/api/releases" ||
        p === "/api/releasebindings" ||
        p === "/api/rendertasks"
      )
        return route.fulfill({ status: 403, body: "Forbidden" });

      // Catch-all: fail loudly so missing fixtures surface immediately.
      return route.fulfill({ status: 500, body: `Unexpected route: ${p}` });
    },
  );
}

test.describe("resource detail views", () => {
  test("target detail shows conditions and navigates to its bound release", async ({
    page,
  }) => {
    await seedMocks(page);

    await page.goto("/targets");
    await expect(page.getByRole("heading", { name: "Targets" })).toBeVisible();

    // The seeded target must be present, missing fixture fails here.
    const rows = page.locator("main button.bg-card");
    await expect(rows).toHaveCount(1);
    await rows.first().click();

    // Landed on the target detail page.
    await expect(page).toHaveURL(/\/targets\/default\/e2e-target/);
    await expect(
      page.getByRole("heading", { name: "Conditions" }),
    ).toBeVisible();

    // The seeded target has conditions, so the table renders unconditionally.
    await expect(
      page.getByRole("columnheader", { name: "Last Transition" }),
    ).toBeVisible();
    await expect(page.getByText("ReleasesRendered")).toBeVisible();

    // The bound release link navigates to the release detail page.
    const releaseLink = page.locator('a[href^="/releases/"]').first();
    await expect(releaseLink).toBeVisible();
    await releaseLink.click();
    await expect(page).toHaveURL(/\/releases\/default\/e2e-release/);
    await expect(
      page.getByRole("heading", { name: "e2e-release" }),
    ).toBeVisible();
  });
});
