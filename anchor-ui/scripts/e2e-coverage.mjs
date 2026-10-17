import { spawn } from "node:child_process";
import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { scenarios } from "./e2e-completion.mjs";

const startMarker = "<!-- e2e-coverage:start -->";
const endMarker = "<!-- e2e-coverage:end -->";
const cwd = path.resolve(import.meta.dirname, "..");
const coveragePath = path.join(cwd, "e2e/guide/coverage.md");

export function summarizeInventory(inventory) {
	if (inventory.errors?.length)
		throw new Error("Playwright discovery reported errors.");
	const discovered = scenarios(inventory);
	if (!discovered.length) throw new Error("The full app inventory is empty.");
	const identities = new Set();
	const counts = new Map();
	for (const { identity } of discovered) {
		if (identities.has(identity))
			throw new Error(`Duplicate scenario: ${identity}`);
		identities.add(identity);
		const [file] = JSON.parse(identity);
		if (typeof file !== "string" || !file)
			throw new Error(`Scenario has no spec file: ${identity}`);
		let relative = file;
		if (path.isAbsolute(file)) {
			if (!inventory.config?.rootDir)
				throw new Error("Absolute spec files require the discovery rootDir.");
			relative = path.relative(inventory.config.rootDir, file);
		}
		relative = relative.split(path.sep).join("/");
		if (
			relative.startsWith("../") ||
			path.isAbsolute(relative) ||
			!relative.endsWith(".e2e.ts")
		)
			throw new Error(`Unexpected app spec file: ${file}`);
		counts.set(relative, (counts.get(relative) ?? 0) + 1);
	}
	return {
		scenarios: discovered.length,
		specs: [...counts.entries()]
			.sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
			.map(([file, count]) => ({ file, scenarios: count })),
	};
}

export function renderCoverage(summary) {
	return [
		startMarker,
		"Generated from `playwright.app.config.ts` discovery. Refresh with",
		"`pnpm update:e2e:coverage`; CI checks this block with `pnpm check:e2e:coverage`.",
		"",
		`The managed app suite declares **${summary.scenarios} scenarios in ${summary.specs.length} spec files**.`,
		"",
		"| Spec | Scenarios |",
		"| --- | ---: |",
		...summary.specs.map(
			({ file, scenarios: count }) =>
				`| [${file}](../features/${file}) | ${count} |`,
		),
		endMarker,
	].join("\n");
}

export function updateCoverageDocument(document, summary) {
	const start = document.indexOf(startMarker);
	const end = document.indexOf(endMarker);
	if (
		start < 0 ||
		end < start ||
		document.indexOf(startMarker, start + startMarker.length) !== -1 ||
		document.indexOf(endMarker, end + endMarker.length) !== -1
	)
		throw new Error(
			"Coverage guide must have exactly one ordered generated block.",
		);
	return (
		document.slice(0, start) +
		renderCoverage(summary) +
		document.slice(end + endMarker.length)
	);
}

export function checkCoverageDocument(document, summary) {
	if (updateCoverageDocument(document, summary) !== document)
		throw new Error(
			"Browser coverage counts have drifted. Run pnpm update:e2e:coverage, then review the behavior matrix.",
		);
}

export async function discoverCoverage() {
	const env = {
		...process.env,
		E2E_COMPLETE: "1",
		PLAYWRIGHT_JSON_OUTPUT_NAME: undefined,
		PLAYWRIGHT_JSON_OUTPUT_FILE: undefined,
		PLAYWRIGHT_JSON_OUTPUT_DIR: undefined,
	};
	const inventory = await new Promise((resolve, reject) => {
		const child = spawn(
			process.execPath,
			[
				path.join(cwd, "node_modules/playwright/cli.js"),
				"test",
				"--config",
				"playwright.app.config.ts",
				"--list",
				"--reporter=json",
			],
			{
				cwd,
				env,
				stdio: ["ignore", "pipe", "inherit"],
			},
		);
		let output = "";
		child.stdout.on("data", (data) => {
			output += data;
		});
		child.once("error", reject);
		child.once("close", (code, signal) => {
			if (code !== 0 || signal)
				reject(new Error(`Playwright discovery exited ${signal ?? code}.`));
			else {
				try {
					resolve(JSON.parse(output));
				} catch {
					reject(
						new Error("Playwright discovery did not return a JSON inventory."),
					);
				}
			}
		});
	});
	return summarizeInventory(inventory);
}

async function main(args) {
	if (args.length === 1 && args[0] === "--help") {
		console.log(
			"Usage: node scripts/e2e-coverage.mjs [--check | --update | --json]\nDiscovers the full app suite without starting Docker, browsers or the app. --check is the default; --update refreshes only the generated guide block; --json prints counts for status tooling.",
		);
		return;
	}
	const mode = args[0] ?? "--check";
	if (args.length > 1 || !["--check", "--update", "--json"].includes(mode))
		throw new Error(
			"Expected --check, --update or --json. Use --help for usage.",
		);
	const summary = await discoverCoverage();
	if (mode === "--json") {
		console.log(JSON.stringify(summary, null, 2));
		return;
	}
	const document = await readFile(coveragePath, "utf8");
	if (mode === "--update")
		await writeFile(coveragePath, updateCoverageDocument(document, summary));
	else checkCoverageDocument(document, summary);
	console.log(
		`Browser coverage ${mode === "--update" ? "updated" : "checked"}: ${summary.scenarios} scenarios in ${summary.specs.length} spec files.`,
	);
}

if (
	process.argv[1] &&
	path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
	try {
		await main(process.argv.slice(2));
	} catch (error) {
		console.error(error.message);
		process.exitCode = 1;
	}
}
