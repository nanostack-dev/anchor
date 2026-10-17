import { execFileSync, spawn } from "node:child_process";
import { createHash } from "node:crypto";
import {
	createWriteStream,
	mkdirSync,
	readFileSync,
	readdirSync,
	writeFileSync,
} from "node:fs";
import os from "node:os";
import path from "node:path";
import { parseArgs } from "node:util";

const { values } = parseArgs({
	options: {
		workers: { type: "string", default: "1,2,4" },
		runs: { type: "string", default: "3" },
	},
});
const workers = values.workers.split(",").map(Number);
const repetitions = Number(values.runs);
if (
	![...workers, repetitions].every(
		(value) => Number.isInteger(value) && value > 0,
	)
)
	throw new Error("Workers and runs must be positive integers");
const root = path.resolve(import.meta.dirname, "..");
function scenarioFingerprint() {
	const hash = createHash("sha256");
	function visit(relative) {
		for (const entry of readdirSync(path.join(root, relative), {
			withFileTypes: true,
		}).sort((a, b) => a.name.localeCompare(b.name))) {
			const file = path.join(relative, entry.name);
			if (entry.isDirectory()) visit(file);
			else if (entry.isFile()) {
				hash.update(file);
				hash.update(readFileSync(path.join(root, file)));
			}
		}
	}
	visit("e2e/features");
	visit("e2e/support");
	for (const file of ["playwright.app.config.ts", "package.json"]) {
		hash.update(file);
		hash.update(readFileSync(path.join(root, file)));
	}
	return hash.digest("hex");
}
const scenarios = scenarioFingerprint();
const directory = path.join(
	root,
	".ui-craft/performance",
	new Date().toISOString().replaceAll(":", "-"),
);
mkdirSync(directory, { recursive: true });
const ready = await fetch("http://127.0.0.1:3015/__e2e/ready").then(
	(response) => {
		if (!response.ok)
			throw new Error("Start the owned managed preview before benchmarking");
		return response.json();
	},
);
const environment = {
	date: new Date().toISOString(),
	commit: execFileSync("git", ["rev-parse", "HEAD"], {
		cwd: root,
		encoding: "utf8",
	}).trim(),
	fingerprint: ready.fingerprint,
	scenarioFingerprint: scenarios,
	node: process.version,
	platform: process.platform,
	arch: process.arch,
	cpu: os.cpus()[0].model,
	logicalCpus: os.cpus().length,
	memoryGiB: Math.round(os.totalmem() / 1024 ** 3),
	runtimeMs: ready.runtimeMs,
	frontendBuildMs: ready.frontendBuildMs,
	startupMs: ready.startupMs,
};
const runs = [];
let identities;
let child;
process.on("SIGINT", () => child?.kill("SIGTERM"));
process.on("SIGTERM", () => child?.kill("SIGTERM"));
for (const count of workers) {
	for (let repeat = 1; repeat <= repetitions; repeat++) {
		const output = path.join(directory, `${count}-${repeat}.json`);
		const log = createWriteStream(
			path.join(directory, `${count}-${repeat}.log`),
		);
		const started = performance.now();
		const code = await new Promise((resolve, reject) => {
			child = spawn("pnpm", ["test:e2e:app"], {
				cwd: root,
				env: {
					...process.env,
					E2E_WORKERS: String(count),
					E2E_REUSE_SERVER: "1",
					E2E_TRACE: "0",
					E2E_JSON_REPORT: output,
				},
				stdio: ["ignore", "pipe", "pipe"],
			});
			child.stdout.pipe(log, { end: false });
			child.stderr.pipe(log, { end: false });
			child.once("error", reject);
			child.once("close", (status) => {
				log.end();
				resolve(status);
			});
		});
		if (code !== 0)
			throw new Error(
				`Worker ${count}, run ${repeat} failed; inspect ${directory}`,
			);
		const report = JSON.parse(readFileSync(output, "utf8"));
		if (scenarioFingerprint() !== scenarios)
			throw new Error(
				"Tests, helpers or configuration changed; restart comparison",
			);
		const tests = [];
		function collect(suite) {
			for (const spec of suite.specs ?? [])
				for (const test of spec.tests ?? [])
					tests.push({
						id: `${suite.file}:${spec.title}:${test.projectName}`,
						durationMs: test.results.at(-1).duration,
					});
			for (const nested of suite.suites ?? []) collect(nested);
		}
		report.suites.forEach(collect);
		const current = tests
			.map((test) => test.id)
			.sort()
			.join("\n");
		identities ??= current;
		if (
			identities !== current ||
			report.stats.skipped ||
			report.stats.flaky ||
			report.stats.unexpected
		)
			throw new Error(
				"Benchmark scenario set changed or contains excluded/flaky cases",
			);
		const durations = tests
			.map((test) => test.durationMs)
			.sort((a, b) => a - b);
		const run = {
			workers: count,
			repeat,
			tests: tests.length,
			wallMs: Math.round(performance.now() - started),
			runnerMs: Math.round(report.stats.duration),
			p95Ms: durations[Math.ceil(durations.length * 0.95) - 1],
			slowest: tests.sort((a, b) => b.durationMs - a.durationMs).slice(0, 5),
			failed: 0,
			flaky: 0,
			skipped: 0,
		};
		runs.push(run);
		writeFileSync(
			path.join(directory, "summary.json"),
			JSON.stringify({ environment, runs }, null, 2),
		);
		console.log(
			`${count} workers, run ${repeat}: ${tests.length} passed, ${(run.runnerMs / 1000).toFixed(1)} s runner, ${(run.wallMs / 1000).toFixed(1)} s wall, p95 ${(run.p95Ms / 1000).toFixed(1)} s`,
		);
	}
}
console.log(`Comparable reports saved locally: ${directory}`);
