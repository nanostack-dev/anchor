import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import {
	cp,
	mkdir,
	mkdtemp,
	readFile,
	readdir,
	rm,
	writeFile,
} from "node:fs/promises";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import path from "node:path";
import { performance } from "node:perf_hooks";
import { pathToFileURL } from "node:url";
import { chromium } from "playwright";
import { scenarios } from "./e2e-completion.mjs";

const cwd = path.resolve(import.meta.dirname, "..");
const cli = path.join(cwd, "node_modules/playwright/cli.js");
const directory = await mkdtemp(path.join(tmpdir(), "anchor-e2e-artifacts-"));

async function inspectReport(reportDir, evidence) {
	const server = createServer(async (request, response) => {
		const pathname = new URL(request.url, "http://localhost").pathname;
		const file = path.resolve(
			reportDir,
			`.${pathname === "/" ? "/index.html" : pathname}`,
		);
		if (!file.startsWith(`${reportDir}${path.sep}`)) {
			response.writeHead(403).end();
			return;
		}
		try {
			const data = await readFile(file);
			const types = {
				".html": "text/html",
				".js": "text/javascript",
				".css": "text/css",
				".png": "image/png",
				".md": "text/plain",
			};
			response
				.writeHead(200, {
					"Content-Type":
						types[path.extname(file)] ?? "application/octet-stream",
				})
				.end(data);
		} catch {
			response.writeHead(404).end();
		}
	});
	let browser;
	try {
		await new Promise((resolve, reject) => {
			server.once("error", reject);
			server.listen(0, "127.0.0.1", resolve);
		});
		const url = `http://127.0.0.1:${server.address().port}`;
		browser = await chromium.launch();
		const page = await browser.newPage();
		page.setDefaultTimeout(5_000);
		page.setDefaultNavigationTimeout(5_000);
		await page.goto(url);
		await page
			.getByRole("link", {
				name: "intentional failure retains evidence",
				exact: true,
			})
			.click();
		for (const { name, bytes } of evidence) {
			const href = await page
				.getByRole("link", { name, exact: true })
				.getAttribute("href");
			assert.ok(href.startsWith(`${url}/data/`));
			const response = await page.request.get(href, { timeout: 5_000 });
			assert.equal(response.status(), 200, `Report link for ${name} failed.`);
			assert.ok((await response.body()).equals(bytes));
		}
		if (evidence.some(({ name }) => name === "trace")) {
			const viewer = await page
				.getByRole("link", { name: "View Trace", exact: true })
				.getAttribute("href");
			assert.equal(
				(await page.request.get(viewer, { timeout: 5_000 })).status(),
				200,
			);
		}
	} finally {
		try {
			await browser?.close();
		} finally {
			server.closeAllConnections();
			await new Promise((resolve) => server.close(resolve));
		}
	}
}

async function folderBytes(folder) {
	let bytes = 0;
	for (const entry of await readdir(folder, { withFileTypes: true })) {
		const file = path.join(folder, entry.name);
		bytes += entry.isDirectory()
			? await folderBytes(file)
			: (await readFile(file)).length;
	}
	return bytes;
}

function run(config, env) {
	return new Promise((resolve, reject) => {
		let output = "";
		const child = spawn(process.execPath, [cli, "test", "--config", config], {
			cwd,
			env,
			stdio: ["ignore", "pipe", "pipe"],
			timeout: 30_000,
		});
		child.stdout.on("data", (data) => {
			output += data;
		});
		child.stderr.on("data", (data) => {
			output += data;
		});
		child.once("error", reject);
		child.once("close", (code, signal) => resolve({ code, signal, output }));
	});
}

async function verify(trace) {
	const trial = path.join(directory, trace ? "traced" : "normal");
	await mkdir(trial);
	const outputDir = path.join(trial, "app");
	const reportDir = path.join(trial, "report");
	const reportPath = path.join(outputDir, "results.json");
	const config = path.join(trial, "playwright.config.mjs");
	await writeFile(
		config,
		`import base from ${JSON.stringify(pathToFileURL(path.join(cwd, "playwright.app.config.ts")).href)};
export default {
  ...base,
  testDir: ${JSON.stringify(trial)},
  testMatch: '**/*.spec.mjs',
  outputDir: ${JSON.stringify(outputDir)},
  workers: 1,
  webServer: undefined,
  projects: [{ name: 'artifacts' }],
  use: { ...base.use, baseURL: undefined, headless: true, launchOptions: { slowMo: 0 } },
};
`,
	);
	await writeFile(
		path.join(trial, "evidence.spec.mjs"),
		`import { test, expect } from ${JSON.stringify(pathToFileURL(path.join(cwd, "node_modules/playwright/test.mjs")).href)};
test('passing case discards recordings', async ({ page }) => {
  await page.setContent('<h1>Ready</h1>');
  await expect(page.getByRole('heading')).toHaveText('Ready');
});
test('intentional failure retains evidence', async ({ page }) => {
  await page.setContent('<h1>Pending</h1>');
  await expect(page.getByRole('heading')).toHaveText('Complete', { timeout: 100 });
});
`,
	);
	const started = performance.now();
	const result = await run(config, {
		...process.env,
		E2E_TRACE: trace ? "1" : "0",
		E2E_JSON_REPORT: reportPath,
		E2E_HTML_REPORT: reportDir,
		E2E_WORKERS: "1",
		E2E_SLOW_MO: "",
		PLAYWRIGHT_HTML_OPEN: "never",
	});
	assert.equal(result.signal, null, result.output);
	assert.equal(result.code, 1, result.output);
	const report = JSON.parse(await readFile(reportPath, "utf8"));
	assert.equal(report.stats.expected, 1, result.output);
	assert.equal(report.stats.unexpected, 1, result.output);
	assert.equal(report.stats.skipped, 0);
	assert.equal(report.stats.flaky, 0);
	assert.deepEqual(report.errors, []);
	const executions = scenarios(report).map(({ execution }) => execution);
	assert.equal(executions.length, 2);
	for (const execution of executions) {
		assert.equal(execution.results.length, 1);
		assert.equal(execution.results[0].retry, 0);
	}
	const failure = executions.find(
		({ results }) => results[0].status === "failed",
	);
	const passed = executions.find(
		({ results }) => results[0].status === "passed",
	);
	assert.ok(failure);
	assert.ok(passed);
	assert.deepEqual(passed.results[0].attachments, []);
	const attachments = failure.results[0].attachments;
	assert.ok(attachments.some(({ name }) => name === "screenshot"));
	assert.ok(attachments.some(({ name }) => name === "error-context"));
	assert.equal(
		attachments.some(({ name }) => name === "trace"),
		trace,
	);
	assert.ok(attachments.every(({ name }) => name !== "video"));

	const downloaded = path.join(trial, "downloaded");
	await cp(reportDir, path.join(downloaded, "report"), { recursive: true });
	await mkdir(path.join(downloaded, "app"), { recursive: true });
	await cp(reportPath, path.join(downloaded, "app/results.json"));
	for (const attachment of attachments)
		await cp(
			attachment.path,
			path.join(downloaded, "app", path.basename(attachment.path)),
		);
	await rm(outputDir, { recursive: true });
	await rm(reportDir, { recursive: true });
	const html = await readFile(
		path.join(downloaded, "report/index.html"),
		"utf8",
	);
	assert.ok(html.includes("Playwright Test Report"));
	const dataDir = path.join(downloaded, "report/data");
	const packaged = await Promise.all(
		(await readdir(dataDir)).map((file) => readFile(path.join(dataDir, file))),
	);
	const evidence = [];
	for (const attachment of attachments) {
		const bytes = await readFile(
			path.join(downloaded, "app", path.basename(attachment.path)),
		);
		assert.ok(
			packaged.some((item) => item.equals(bytes)),
			`Downloaded HTML report is missing ${attachment.name}.`,
		);
		evidence.push({ name: attachment.name, bytes });
	}
	await inspectReport(path.join(downloaded, "report"), evidence);
	console.log(
		`Failure packaging verified: trace=${trace}, exit=1, screenshot/context retained, HTML links readable, report ${Math.floor((await folderBytes(path.join(downloaded, "report"))) / 1024)} KiB, ${(performance.now() - started).toFixed(0)} ms.`,
	);
}

try {
	await verify(false);
	await verify(true);
} finally {
	await rm(directory, { recursive: true, force: true });
}
