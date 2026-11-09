import { availableParallelism } from "node:os";
import { defineConfig, devices } from "playwright/test";

const workers = process.env.E2E_WORKERS
	? Number(process.env.E2E_WORKERS)
	: process.env.CI
		? 2
		: Math.min(4, availableParallelism());
if (!Number.isInteger(workers) || workers < 1)
	throw new Error("E2E_WORKERS must be a positive integer");
const slowMo = process.env.E2E_SLOW_MO ? Number(process.env.E2E_SLOW_MO) : 0;
if (process.env.E2E_SLOW_MO && (!Number.isSafeInteger(slowMo) || slowMo < 1))
	throw new Error("E2E_SLOW_MO must be a positive safe integer");

export default defineConfig({
	testDir: "./e2e/features",
	testMatch: "**/*.e2e.ts",
	fullyParallel: true,
	workers,
	forbidOnly: Boolean(process.env.CI) || process.env.E2E_COMPLETE === "1",
	retries: 0,
	timeout: 30_000,
	expect: { timeout: 8_000 },
	outputDir: "test-results/app",
	reporter: [
		["list"],
		[
			"json",
			{
				outputFile:
					process.env.E2E_JSON_REPORT ?? "test-results/app/results.json",
			},
		],
		[
			"html",
			{
				outputFolder: process.env.E2E_HTML_REPORT ?? "test-results/report",
				open: "never",
			},
		],
	],
	use: {
		baseURL: "http://127.0.0.1:3015",
		launchOptions: { slowMo },
		actionTimeout: 8_000,
		navigationTimeout: 15_000,
		trace: process.env.E2E_TRACE === "1" ? "retain-on-failure" : "off",
		screenshot: "only-on-failure",
		video: "off",
	},
	projects: [
		{
			name: "bootstrap",
			testMatch: "**/bootstrap.e2e.ts",
			use: { ...devices["Desktop Chrome"] },
		},
		{
			name: "chromium",
			testIgnore: "**/bootstrap.e2e.ts",
			dependencies: ["bootstrap"],
			use: { ...devices["Desktop Chrome"] },
		},
	],
	webServer: {
		command: "node scripts/serve-e2e-full.mjs",
		url: "http://127.0.0.1:3015/__e2e/ready",
		// Covers two serial phases: the backend (Postgres + `go build ./cmd`, 33-165 s on
		// GitHub runners depending on the Go build cache) and `vite build`. 180 s timed out
		// on 4 branches (docs/postmortems/2026-11-09-e2e-app-startup-budget.md).
		timeout: 420_000,
		gracefulShutdown: { signal: "SIGTERM", timeout: 30_000 },
		reuseExistingServer: process.env.E2E_REUSE_SERVER === "1",
		env: {
			E2E_EMAIL: "",
			E2E_PASSWORD: "",
			VITE_API_BASE_URL: "http://127.0.0.1:3015",
		},
	},
});
