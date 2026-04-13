import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  timeout: 60_000,
  expect: {
    timeout: 10_000,
  },
  fullyParallel: false,
  retries: 1,
  reporter: "html",
  use: {
    baseURL: "http://localhost:8090",
    trace: "on-first-retry",
    screenshot: "only-on-failure",
    // Dex uses a self-signed TLS certificate in dev/test
    ignoreHTTPSErrors: true,
    launchOptions: {
      executablePath:
        process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH || undefined,
    },
  },
  projects: [
    {
      name: "setup",
      testMatch: /auth\.setup\.ts/,
      use: {
        ...devices["Desktop Chrome"],
      },
    },
    {
      name: "unauthenticated",
      testMatch: /api-proxy\.spec\.ts|auth\.spec\.ts/,
      use: {
        ...devices["Desktop Chrome"],
      },
    },
    {
      name: "authenticated",
      testMatch: /spa\.spec\.ts/,
      dependencies: ["setup"],
      use: {
        ...devices["Desktop Chrome"],
        storageState: "e2e/.auth/session.json",
      },
    },
    {
      name: "oidc",
      testMatch: /oidc-login\.spec\.ts/,
      use: {
        ...devices["Desktop Chrome"],
      },
    },
  ],
});
