import { spawn } from "node:child_process";
import path from "node:path";
import { assertPreflight } from "./e2e-preflight.mjs";

const directory = path.resolve(import.meta.dirname, "..");
const [suite, ...args] = process.argv.slice(2);
let child;
let interrupted;
for (const signal of ["SIGINT", "SIGTERM"])
	process.on(signal, () => {
		interrupted = signal;
		child?.kill(signal);
	});
try {
	if (!["app", "review"].includes(suite))
		throw new Error(
			"Usage: node scripts/run-e2e.mjs <app|review> [Playwright arguments]",
		);
	if (!args.some((arg) => ["--help", "-h", "--list"].includes(arg)))
		await assertPreflight({
			mode:
				process.env.E2E_REUSE_SERVER === "1" &&
				process.env.E2E_REQUIRE_FRESH !== "1"
					? "reuse"
					: "fresh",
		});
	if (!interrupted) {
		const result = await new Promise((resolve, reject) => {
			child = spawn(
				process.execPath,
				[
					path.join(directory, "node_modules/playwright/cli.js"),
					"test",
					"--config",
					`playwright.${suite}.config.ts`,
					...args,
				],
				{ cwd: directory, env: process.env, stdio: "inherit" },
			);
			child.once("error", reject);
			child.once("close", (code, signal) => resolve({ code, signal }));
		});
		process.exitCode =
			result.code ??
			(result.signal === "SIGINT"
				? 130
				: result.signal === "SIGTERM"
					? 143
					: 1);
	}
} catch (error) {
	console.error(error.message);
	process.exitCode = 1;
}
if (interrupted) process.exitCode = interrupted === "SIGINT" ? 130 : 143;
