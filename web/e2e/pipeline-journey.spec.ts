import { test, expect, type Page } from "@playwright/test";

const NS = "default";
const NOW = "2026-01-15T10:00:00.000Z";

const MOCK_AUTH = {
  authenticated: true,
  username: "admin@solar.local",
  groups: ["system:masters"],
  canImpersonate: false,
  canListAllNamespaces: false, // forces namespace selector to pick first ns
};

const MOCK_NAMESPACES = { items: [{ metadata: { name: NS } }] };

const PROFILE = {
  metadata: { name: "production", namespace: NS, creationTimestamp: NOW },
  spec: {
    releaseRef: { name: "profile-ocm-demo-release" },
    targetSelector: { matchLabels: { env: "prod" } },
  },
  status: { matchedTargets: 1, conditions: [] },
};

const RELEASE = {
  metadata: {
    name: "profile-ocm-demo-release",
    namespace: NS,
    creationTimestamp: NOW,
  },
  spec: { componentVersionRef: { name: "opendefense-cloud-ocm-demo-v26-4-2" } },
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
    effectiveUniqueName: "opendefense-cloud-ocm-demo-profile",
  },
};

// The target controller creates one RenderTask per (release, target) pair.
// ownerKind is always "Target"; the release is encoded in spec.repository as
// the last segment: "{targetNs}/{relNs}/release-{relName}".
const RENDER_TASK = {
  metadata: {
    name: "render-rel-profile-ocm-demo-release-abc12345",
    namespace: NS,
    creationTimestamp: NOW,
  },
  spec: {
    baseURL: "deploy-registry.solar.local",
    repository: `${NS}/${NS}/release-${RELEASE.metadata.name}`,
    tag: "v0.0.1",
    ownerKind: "Target",
    ownerName: "cluster-1",
    ownerNamespace: NS,
  },
  status: {
    conditions: [
      {
        type: "JobScheduled",
        status: "True",
        lastTransitionTime: NOW,
        reason: "JobScheduled",
        message: "",
      },
      {
        type: "JobSucceeded",
        status: "True",
        lastTransitionTime: NOW,
        reason: "JobComplete",
        message: "",
      },
    ],
    chartURL: `oci://deploy-registry.solar.local/${NS}/${NS}/release-${RELEASE.metadata.name}:v0.0.1`,
  },
};

const TARGET = {
  metadata: {
    name: "cluster-1",
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
    name: "production-cluster-1-binding",
    namespace: NS,
    creationTimestamp: NOW,
    ownerReferences: [
      {
        apiVersion: "solar.opendefense.cloud/v1alpha1",
        kind: "Profile",
        name: PROFILE.metadata.name,
        uid: "00000000-0000-0000-0000-0000000000p1",
        controller: true,
      },
    ],
  },
  spec: {
    releaseRef: { name: RELEASE.metadata.name },
    targetRef: { name: TARGET.metadata.name },
  },
  status: { conditions: [] },
};

const REGISTRY = {
  metadata: { name: "deploy-registry", namespace: NS, creationTimestamp: NOW },
  spec: { baseURL: "deploy-registry.solar.local", type: "oci", https: true },
};

function list<T>(items: T[]) {
  return { items };
}

async function setupMocks(page: Page) {
  // Captured POST bodies for the Target create test.
  const postedTargets: unknown[] = [];

  // Set namespace before page JS runs so NamespaceProvider.loadInitial() reads "default".
  await page.addInitScript(() => {
    localStorage.setItem("solar-ui-selected-namespace", "default");
  });

  // Use a URL predicate instead of a glob — glob patterns with **/api/**
  // are unreliable against full URLs containing "://" in Playwright 1.61.
  await page.route(
    (url) => url.pathname.startsWith("/api/"),
    (route) => {
      const { pathname: p } = new URL(route.request().url());
      const ns = NS;

      // Auth + infra
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
      if (p === `/api/namespaces/${ns}/profiles/${PROFILE.metadata.name}`)
        return route.fulfill({ json: PROFILE });
      if (p === `/api/namespaces/${ns}/releases/${RELEASE.metadata.name}`)
        return route.fulfill({ json: RELEASE });
      if (p === `/api/namespaces/${ns}/targets/${TARGET.metadata.name}`)
        return route.fulfill({ json: TARGET });
      if (
        p ===
        `/api/namespaces/${ns}/rendertasks/${RENDER_TASK.metadata.name}`
      )
        return route.fulfill({ json: RENDER_TASK });

      // Namespace-scoped list routes
      if (p === `/api/namespaces/${ns}/profiles`)
        return route.fulfill({ json: list([PROFILE]) });
      if (p === `/api/namespaces/${ns}/releases`)
        return route.fulfill({ json: list([RELEASE]) });
      // The create under test: capture the POST body, return the created target.
      if (p === `/api/namespaces/${ns}/targets` && route.request().method() === "POST") {
        postedTargets.push(route.request().postDataJSON());
        return route.fulfill({ json: TARGET });
      }
      if (p === `/api/namespaces/${ns}/targets`)
        return route.fulfill({ json: list([TARGET]) });
      if (p === `/api/namespaces/${ns}/registries`)
        return route.fulfill({ json: list([REGISTRY]) });
      if (p === `/api/namespaces/${ns}/releasebindings`)
        return route.fulfill({ json: list([RELEASE_BINDING]) });
      if (p === `/api/namespaces/${ns}/rendertasks`)
        return route.fulfill({ json: list([RENDER_TASK]) });

      if (p === "/api/targets") return route.fulfill({ json: list([TARGET]) });

      if (
        p === "/api/profiles" ||
        p === "/api/releases" ||
        p === "/api/releasebindings" ||
        p === "/api/rendertasks"
      )
        return route.fulfill({ status: 403, body: "Forbidden" });

      // Catch-all: fail loudly so regressions surface immediately
      return route.fulfill({ status: 500, body: `Unexpected route: ${p}` });
    },
  );

  return { postedTargets };
}

test.describe("Profile → Release → Target click journey", () => {
  test("full deployment-cycle navigation with mocked data", async ({ page }) => {
    await setupMocks(page);

    // ── Step 1: Profile list ──────────────────────────────────────────────

    await test.step("profile list shows the mocked profile", async () => {
      await page.goto("/profiles");
      await expect(
        page.getByRole("heading", { name: "Profiles" })
      ).toBeVisible();

      // Card title
      await expect(
        page.getByRole("heading", { name: "production", level: 3 })
      ).toBeVisible();

      // Matched-targets count badge
      await expect(page.getByText("1 matched")).toBeVisible();
    });

    // ── Step 2: Profile detail ────────────────────────────────────────────

    await test.step("clicking profile card opens profile detail", async () => {
      await page.getByRole("heading", { name: "production", level: 3 }).click();
      await expect(page).toHaveURL(/\/profiles\/default\/production/);

      // Hero heading
      await expect(
        page.getByRole("heading", { name: "production" })
      ).toBeVisible();

      // Release ref shown
      await expect(
        page.getByText("profile-ocm-demo-release").first()
      ).toBeVisible();

      // Target selector labels
      await expect(page.getByText(/env\s*=\s*prod/)).toBeVisible();

      // Matched target appears in the target list
      await expect(
        page.getByRole("link", { name: "cluster-1" })
      ).toBeVisible();
    });

    // ── Step 3: Release detail ────────────────────────────────────────────

    await test.step("clicking release link opens release detail", async () => {
      await page
        .getByRole("link", { name: "profile-ocm-demo-release" })
        .click();
      await expect(page).toHaveURL(
        /\/releases\/default\/profile-ocm-demo-release/
      );

      // Hero heading
      await expect(
        page.getByRole("heading", { name: "profile-ocm-demo-release" })
      ).toBeVisible();

      // ComponentVersion ref in facts grid
      await expect(
        page.getByText("opendefense-cloud-ocm-demo-v26-4-2").first()
      ).toBeVisible();

      // Render task section: succeeded phase dot + task name
      await expect(page.getByText("render-rel-profile-ocm-demo-release-abc12345")).toBeVisible();
      await expect(page.getByText("succeeded")).toBeVisible();

      // Chart URL shown
      await expect(
        page.getByText(/release-profile-ocm-demo-release:v0\.0\.1/)
      ).toBeVisible();

      // Deployed-on-targets: cluster-1 link
      await expect(
        page.getByRole("link", { name: "cluster-1" })
      ).toBeVisible();
    });

    // ── Step 4: Target detail ─────────────────────────────────────────────

    await test.step("clicking target link opens target detail", async () => {
      await page.getByRole("link", { name: "cluster-1" }).click();
      await expect(page).toHaveURL(/\/targets\/default\/cluster-1/);

      // Hero heading
      await expect(
        page.getByRole("heading", { name: "cluster-1" })
      ).toBeVisible();

      // Health badge
      await expect(page.getByText("Healthy")).toBeVisible();

      // Registry fact
      await expect(page.getByText("deploy-registry").first()).toBeVisible();

      // Bound release shown with link back to release detail
      await expect(
        page.getByRole("link", { name: "profile-ocm-demo-release" })
      ).toBeVisible();

      // Conditions table
      await expect(page.getByText("ReleasesRendered")).toBeVisible();
      await expect(page.getByText("BootstrapReady")).toBeVisible();
    });

    // ── Step 5: Back navigation ───────────────────────────────────────────

    await test.step("back button returns to target list", async () => {
      await page.getByRole("button", { name: /Back to Targets/i }).click();
      await expect(page).toHaveURL(/\/targets$/);
      await expect(
        page.getByRole("heading", { name: "Targets" })
      ).toBeVisible();
    });
  });
});

test.describe("Target create", () => {
  test("trims whitespace from name and registry before creating", async ({
    page,
  }) => {
    const { postedTargets } = await setupMocks(page);

    await page.goto("/targets");
    await expect(page.getByRole("heading", { name: "Targets" })).toBeVisible();

    await page.getByRole("button", { name: "New Target" }).click();
    await expect(
      page.getByRole("heading", { name: "New Target" })
    ).toBeVisible();

    // Name is a plain <input>; the registry field is an <input list>, which
    // gets the implicit ARIA role "combobox".
    await page.getByRole("textbox").first().fill(`  ${TARGET.metadata.name}  `);
    await page.getByRole("combobox").fill(`  ${REGISTRY.metadata.name}  `);

    await page.getByRole("button", { name: "Create" }).click();

    // Success toast reports the trimmed name.
    await expect(page.getByRole("status")).toContainText(
      `Target "${TARGET.metadata.name}" created.`
    );

    // The POST body carries the trimmed values, not the raw padded input.
    expect(postedTargets).toHaveLength(1);
    const body = postedTargets[0] as {
      metadata: { name: string };
      spec: { renderRegistryRef: { name: string } };
    };
    expect(body.metadata.name).toBe(TARGET.metadata.name);
    expect(body.spec.renderRegistryRef.name).toBe(REGISTRY.metadata.name);

    // Dialog closes after success.
    await expect(
      page.getByRole("heading", { name: "New Target" })
    ).toBeHidden();
  });
});
