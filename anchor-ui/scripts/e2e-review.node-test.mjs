import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtemp, readFile, rm, stat } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { chromium } from "playwright";
import { captureReviewCheckpoint } from "../e2e/support/review.ts";

const directory = path.resolve(import.meta.dirname, "..");
const configurationLoader = `
import { createRequire } from "node:module";
import path from "node:path";
const require = createRequire(import.meta.url);
const packageRoot = path.dirname(require.resolve("playwright/package.json"));
const { configLoader } = require(path.join(packageRoot, "lib/common/index.js"));
const loaded = await configLoader.loadConfigFromFile(process.argv[1]);
console.log(JSON.stringify(loaded.config));
`;

function environment(extra = {}) {
	return {
		...process.env,
		E2E_FRONTEND_PORT: undefined,
		E2E_WORKERS: "",
		E2E_SLOW_MO: "",
		E2E_REVIEW_SLOW_MO: "",
		PLAYWRIGHT_JSON_OUTPUT_NAME: undefined,
		PLAYWRIGHT_JSON_OUTPUT_FILE: undefined,
		PLAYWRIGHT_JSON_OUTPUT_DIR: undefined,
		...extra,
	};
}

function loadConfiguration(file, extra = {}) {
	return spawnSync(
		process.execPath,
		["--input-type=module", "--eval", configurationLoader, file],
		{
			cwd: directory,
			encoding: "utf8",
			timeout: 30_000,
			env: environment(extra),
		},
	);
}

function configuration(file, extra = {}) {
	const result = loadConfiguration(file, extra);
	assert.equal(result.status, 0, result.stderr);
	return JSON.parse(result.stdout);
}

test("responsive review isolates evidence while preserving one managed app server", () => {
	const ordinary = configuration("playwright.app.config.ts");
	const review = configuration("playwright.review.config.ts");
	assert.deepEqual(review.webServer, ordinary.webServer);
	assert.equal(Array.isArray(review.webServer), false);
	assert.equal(review.workers, 1);
	assert.equal(review.fullyParallel, false);
	assert.equal(review.forbidOnly, true);
	assert.deepEqual(
		review.projects.map((project) => project.name),
		["bootstrap", "mobile", "tablet", "desktop"],
	);
	const [bootstrap, ...profiles] = review.projects;
	assert.equal(bootstrap.use.headless, true);
	assert.equal(bootstrap.use.launchOptions.slowMo, 0);
	assert.equal(bootstrap.use.video, "off");
	assert.equal(bootstrap.use.screenshot, "off");
	assert.equal(bootstrap.metadata.reviewEvidence, undefined);
	assert.deepEqual(bootstrap.dependencies, []);
	const viewports = [
		{ width: 393, height: 851 },
		{ width: 768, height: 1024 },
		{ width: 1440, height: 900 },
	];
	for (const [index, project] of profiles.entries()) {
		assert.deepEqual(project.dependencies, ["bootstrap"]);
		assert.equal(project.use.browserName, "chromium");
		assert.equal(project.use.colorScheme, "light");
		assert.deepEqual(project.use.viewport, viewports[index]);
		assert.equal(project.use.isMobile, index < 2);
		assert.equal(project.use.hasTouch, index < 2);
		assert.deepEqual(project.use.video, { mode: "on", size: viewports[index] });
		assert.equal(project.use.screenshot, "on");
		assert.equal(project.use.trace, "off");
		assert.equal(project.use.launchOptions.slowMo, 500);
		assert.equal(project.metadata.reviewEvidence, true);
		assert.equal(project.metadata.reviewHoldMs, 1_500);
	}
	for (const project of review.projects) {
		assert.equal(project.retries, 0);
		assert.equal(project.timeout, 180_000);
		assert.equal(
			project.outputDir,
			path.join(directory, ".ui-craft/review/results"),
		);
	}
	assert.deepEqual(review.reporter, [
		["list", null],
		["json", { outputFile: ".ui-craft/review/results.json" }],
		["html", { outputFolder: ".ui-craft/review/report", open: "never" }],
	]);
	for (const project of ordinary.projects) {
		assert.equal(project.use.video, "off");
		assert.equal(project.use.launchOptions.slowMo, 0);
		assert.equal(project.metadata.reviewEvidence, undefined);
		assert.equal(project.outputDir, path.join(directory, "test-results/app"));
	}
});

function discover(file) {
	const result = spawnSync(
		process.execPath,
		[
			"node_modules/playwright/cli.js",
			"test",
			"--config",
			file,
			"--list",
			"--reporter=json",
		],
		{
			cwd: directory,
			encoding: "utf8",
			timeout: 30_000,
			maxBuffer: 4_194_304,
			env: environment(),
		},
	);
	assert.equal(result.status, 0, result.stderr);
	const inventory = JSON.parse(result.stdout);
	const counts = new Map();
	function visit(suite) {
		for (const spec of suite.specs ?? [])
			for (const execution of spec.tests ?? [])
				counts.set(
					execution.projectName,
					(counts.get(execution.projectName) ?? 0) + 1,
				);
		for (const nested of suite.suites ?? []) visit(nested);
	}
	for (const suite of inventory.suites) visit(suite);
	return counts;
}

test("actual discovery runs bootstrap once and retains every feature in each review profile", () => {
	const ordinary = discover("playwright.app.config.ts");
	const review = discover("playwright.review.config.ts");
	assert.equal(ordinary.get("bootstrap"), 1);
	assert.equal(review.get("bootstrap"), 1);
	for (const name of ["mobile", "tablet", "desktop"])
		assert.equal(review.get(name), ordinary.get("chromium"));
	assert.equal(review.size, 4);
});

test("review pacing overrides stay isolated and invalid values fail before startup", () => {
	for (const value of ["0", "125"]) {
		const loaded = configuration("playwright.review.config.ts", {
			E2E_REVIEW_SLOW_MO: value,
		});
		assert.equal(loaded.projects[0].use.launchOptions.slowMo, 0);
		assert.equal(loaded.projects[1].use.launchOptions.slowMo, Number(value));
	}
	for (const value of ["-1", "1.5", "1e2", "NaN", "9007199254740992"]) {
		const loaded = loadConfiguration("playwright.review.config.ts", {
			E2E_REVIEW_SLOW_MO: value,
		});
		assert.notEqual(loaded.status, 0);
		assert.match(
			loaded.stderr,
			/E2E_REVIEW_SLOW_MO must be a non-negative safe integer/,
		);
	}
});

test("ordinary runs never evaluate or capture review checkpoints", async () => {
	const page = {
		evaluate: () => {
			throw new Error("Unexpected review browser action");
		},
	};
	await captureReviewCheckpoint(
		page,
		{ project: { metadata: {} } },
		"../ignored-outside-review",
	);
	const info = { project: { metadata: { reviewEvidence: true } } };
	await assert.rejects(
		captureReviewCheckpoint(page, info, "../escape"),
		/checkpoint names/,
	);
	info.project.metadata.reviewHoldMs = -1;
	await assert.rejects(
		captureReviewCheckpoint(page, info, "safe-name"),
		/reviewHoldMs/,
	);
});

test("a real mobile context produces a viewport PNG attachment and finalized review video", async (t) => {
	const output = await mkdtemp(
		path.join(tmpdir(), "anchor-review-checkpoint-"),
	);
	t.after(() => rm(output, { recursive: true, force: true }));
	const browser = await chromium.launch();
	t.after(() => browser.close());
	const viewport = { width: 393, height: 851 };
	const context = await browser.newContext({
		viewport,
		isMobile: true,
		hasTouch: true,
		deviceScaleFactor: 2.625,
		colorScheme: "light",
		recordVideo: { dir: output, size: viewport },
	});
	const page = await context.newPage();
	await page.setContent(
		'<meta name="viewport" content="width=device-width"><style>body{min-height:2000px;font:24px sans-serif}</style><h1>Review checkpoint</h1><button>Save</button>',
	);
	const attachments = [];
	const info = {
		project: { metadata: { reviewEvidence: true, reviewHoldMs: 100 } },
		outputPath: (name) => path.join(output, name),
		attach: async (name, attachment) =>
			attachments.push({ name, ...attachment }),
	};
	const before = performance.now();
	await captureReviewCheckpoint(page, info, "mobile-ready");
	assert.ok(performance.now() - before >= 100);
	assert.equal(attachments.length, 1);
	assert.equal(attachments[0].name, "checkpoint-mobile-ready");
	assert.equal(attachments[0].contentType, "image/png");
	const screenshot = await readFile(attachments[0].path);
	assert.deepEqual(
		[...screenshot.subarray(0, 8)],
		[137, 80, 78, 71, 13, 10, 26, 10],
	);
	assert.equal(screenshot.readUInt32BE(16), viewport.width);
	assert.equal(screenshot.readUInt32BE(20), viewport.height);
	const recording = page.video();
	assert.ok(recording);
	await context.close();
	assert.ok((await stat(await recording.path())).size > 0);
});

test("custom headless ports stay aligned through app and responsive configurations", () => {
	for (const file of [
		"playwright.app.config.ts",
		"playwright.review.config.ts",
	]) {
		const loaded = configuration(file, { E2E_FRONTEND_PORT: "13015" });
		assert.equal(loaded.webServer.url, "http://127.0.0.1:13015/__e2e/ready");
		assert.equal(
			loaded.webServer.env.VITE_API_BASE_URL,
			"http://127.0.0.1:13015",
		);
		assert.equal(loaded.webServer.reuseExistingServer, false);
		for (const project of loaded.projects)
			assert.equal(project.use.baseURL, "http://127.0.0.1:13015");
	}
});
