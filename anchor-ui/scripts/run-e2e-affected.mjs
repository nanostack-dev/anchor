import { spawn } from "node:child_process";
import { mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { verifyCompletion } from "./e2e-completion.mjs";
import { planFromGit } from "./e2e-impact.mjs";
import { statusRuntime } from "./e2e-runtime.mjs";

const cwd = path.resolve(import.meta.dirname, "..");
const cli = path.join(cwd, "node_modules/playwright/cli.js");
let child;
let interrupted = false;
function cancel() {
	interrupted = true;
	child?.kill("SIGINT");
}
process.on("SIGINT", cancel);
process.on("SIGTERM", cancel);

function run(args, env, capture = false) {
	if (interrupted) throw new Error("Browser verification was interrupted.");
	return new Promise((resolve, reject) => {
		let output = "";
		child = spawn(process.execPath, [cli, "test", ...args], {
			cwd,
			env,
			stdio: capture ? ["ignore", "pipe", "inherit"] : "inherit",
		});
		if (capture)
			child.stdout.on("data", (data) => {
				output += data;
			});
		child.once("error", reject);
		child.once("close", (code, signal) => {
			child = undefined;
			if (code !== 0 || signal || interrupted)
				reject(new Error(`Playwright exited ${signal ?? code}.`));
			else resolve(output);
		});
	});
}

async function main(args) {
	const options = { cwd };
	let complete = false;
	let list = false;
	for (let i = 0; i < args.length; i++) {
		if (args[i] === "--help") {
			console.log(
				"Usage: node scripts/run-e2e-affected.mjs [--base <ref>] [--head <ref>] [--complete] [--list]\nFocused runs select affected domains. --complete runs and verifies the entire current inventory, affected first. Filtering flags are deliberately unsupported.",
			);
			return;
		}
		if (args[i] === "--complete") complete = true;
		else if (args[i] === "--list") list = true;
		else if (
			["--base", "--head"].includes(args[i]) &&
			args[i + 1] &&
			!args[i + 1].startsWith("--")
		)
			options[args[i].slice(2)] = args[++i];
		else throw new Error(`Unknown option or missing value: ${args[i]}`);
	}
	const plan = planFromGit(options);
	console.log(
		`Browser selection: ${plan.mode}; ${plan.testPaths.length}/${plan.allTestPaths.length} spec files; ${plan.domains.join(", ") || "no feature changes"}.`,
	);
	for (const reason of plan.reasons)
		console.log(
			`  ${JSON.stringify(reason.path)}: ${reason.rule} — ${reason.message}`,
		);
	if (!complete && plan.mode === "none") {
		console.log(
			"No focused browser run needed. Use test:e2e:verify for complete coverage.",
		);
		return;
	}
	if (!list && (await statusRuntime()))
		throw new Error(
			"An Anchor E2E runtime already exists. Stop its owner before a fresh verification; it has been left running.",
		);
	const directory = await mkdtemp(path.join(tmpdir(), "anchor-e2e-selection-"));
	const planPath = path.join(directory, "plan.json");
	const reportPath = path.join(cwd, "test-results/app/results.json");
	const env = {
		...process.env,
		E2E_SELECTION_FILE: planPath,
		E2E_COMPLETE: complete ? "1" : "0",
		E2E_REUSE_SERVER: "0",
		E2E_REQUIRE_FRESH: "1",
		E2E_JSON_REPORT: reportPath,
	};
	const config =
		plan.mode === "selected"
			? "playwright.affected.config.ts"
			: "playwright.app.config.ts";
	try {
		await writeFile(planPath, JSON.stringify(plan), { mode: 0o600 });
		if (list) {
			await run(["--config", config, "--list"], {
				...env,
				E2E_JSON_REPORT: path.join(directory, "listed.json"),
			});
			return;
		}
		await mkdir(path.join(cwd, "test-results"), { recursive: true });
		await rm(path.join(cwd, "test-results/completion.json"), { force: true });
		await writeFile(
			path.join(cwd, "test-results/selection.json"),
			JSON.stringify({ ...plan, complete }, null, 2),
		);
		const inventory = complete
			? JSON.parse(
					await run(
						[
							"--config",
							"playwright.app.config.ts",
							"--list",
							"--reporter=json",
						],
						env,
						true,
					),
				)
			: null;
		await run(["--config", config], env);
		if (complete) {
			const report = JSON.parse(await readFile(reportPath, "utf8"));
			const proof = {
				...verifyCompletion(inventory, report),
				base: plan.base,
				head: plan.head,
				mergeBase: plan.mergeBase,
				mode: plan.mode,
			};
			await writeFile(
				path.join(cwd, "test-results/completion.json"),
				JSON.stringify(proof, null, 2),
			);
			console.log(
				`Complete coverage verified: ${proof.scenarios} distinct scenarios passed once, without skips or retries.`,
			);
		} else
			console.log(
				"Focused browser run passed. Complete coverage requires test:e2e:verify.",
			);
	} finally {
		await rm(directory, { recursive: true, force: true });
	}
}

try {
	await main(process.argv.slice(2));
} catch (error) {
	console.error(error.message);
	process.exitCode = interrupted ? 130 : 1;
}
