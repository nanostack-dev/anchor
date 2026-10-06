import { existsSync } from "node:fs";
import { loadEnvFile } from "node:process";
import { defineConfig, devices } from "playwright/test";

if (existsSync(".env.e2e.local")) {
	loadEnvFile(".env.e2e.local");
}

const baseURL = process.env.E2E_BASE_URL || "http://127.0.0.1:3014";
const apiURL = process.env.E2E_API_URL || "https://apidev.tryanchor.dev";

export default defineConfig({
	testDir: "./e2e/smoke",
	testMatch: "**/*.e2e.ts",
	fullyParallel: false,
	forbidOnly: !!process.env.CI,
	workers: 1,
	timeout: 45_000,
	expect: { timeout: 15_000 },
	reporter: [["list"], ["html", { open: "never" }]],
	use: {
		baseURL,
		trace: "off",
		screenshot: "off",
		video: "off",
	},
	projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
	webServer: process.env.E2E_BASE_URL
		? undefined
		: {
				command: "pnpm build && node scripts/serve-e2e.mjs",
				url: baseURL,
				timeout: 120_000,
				env: {
					VITE_API_BASE_URL: baseURL,
					E2E_API_URL: apiURL,
					E2E_EMAIL: "",
					E2E_PASSWORD: "",
				},
			},
});
