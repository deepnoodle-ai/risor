import { defineConfig } from "@playwright/test";

// RISOR_CHROMIUM points at a Chromium to use instead of Playwright's own,
// for a machine that has one and no network.
export default defineConfig({
  testDir: "e2e",
  timeout: 30_000,
  use: {
    baseURL: "http://localhost:5199",
    launchOptions: process.env.RISOR_CHROMIUM ? { executablePath: process.env.RISOR_CHROMIUM } : {},
  },
  webServer: { command: "npx vite --port 5199 --strictPort", url: "http://localhost:5199", reuseExistingServer: true },
});
