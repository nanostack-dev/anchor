import { defineConfig, devices } from "playwright/test";
import appConfig from "./playwright.app.config";

const pacing = process.env.E2E_REVIEW_SLOW_MO;
const slowMo = pacing ? Number(pacing) : 500;
if (
	pacing &&
	(!/^\d+$/.test(pacing) || !Number.isSafeInteger(slowMo) || slowMo < 0)
)
	throw new Error("E2E_REVIEW_SLOW_MO must be a non-negative safe integer");

const bootstrap = appConfig.projects?.find(
	(project) => project.name === "bootstrap",
);
const feature = appConfig.projects?.find(
	(project) => project.name === "chromium",
);
if (!bootstrap || !feature)
	throw new Error(
		"Responsive review requires the managed app bootstrap and feature projects",
	);

const profiles = [
	{
		name: "mobile",
		device: devices["Pixel 7"],
		viewport: { width: 393, height: 851 },
	},
	{
		name: "tablet",
		device: devices["iPad Mini"],
		viewport: { width: 768, height: 1024 },
	},
	{
		name: "desktop",
		device: devices["Desktop Chrome"],
		viewport: { width: 1440, height: 900 },
	},
];

export default defineConfig({
	...appConfig,
	fullyParallel: false,
	workers: 1,
	forbidOnly: true,
	retries: 0,
	timeout: 180_000,
	outputDir: ".ui-craft/review/results",
	reporter: [
		["list"],
		["json", { outputFile: ".ui-craft/review/results.json" }],
		["html", { outputFolder: ".ui-craft/review/report", open: "never" }],
	],
	use: {
		...appConfig.use,
		colorScheme: "light",
		trace: "off",
		launchOptions: { slowMo },
		screenshot: "on",
	},
	projects: [
		{
			...bootstrap,
			use: {
				...bootstrap.use,
				colorScheme: "light",
				headless: true,
				launchOptions: { slowMo: 0 },
				video: "off",
				screenshot: "off",
				trace: "off",
			},
		},
		...profiles.map(({ name, device, viewport }) => ({
			...feature,
			name,
			dependencies: ["bootstrap"],
			metadata: { reviewEvidence: true, reviewHoldMs: 1_500 },
			use: {
				...device,
				browserName: "chromium" as const,
				viewport,
				video: { mode: "on" as const, size: viewport },
			},
		})),
	],
});
