import { execFileSync, spawnSync } from "node:child_process";
import { existsSync } from "node:fs";
import path from "node:path";

// Usage: node scripts/changed.mjs <format|lint> [--base REF]
const [tool, flag, baseArg] = process.argv.slice(2);
const base = flag === "--base" ? baseArg : "origin/main";
const commands = {
	format: ["format", "--write"],
	lint: ["lint"],
};
if (!commands[tool]) {
	console.error("Usage: node scripts/changed.mjs <format|lint> [--base REF]");
	process.exit(2);
}

const root = path.resolve(import.meta.dirname, "..");
const git = (...args) =>
	execFileSync("git", args, { cwd: root, encoding: "utf8" })
		.split("\n")
		.filter(Boolean);
const mergeBase = git("merge-base", base, "HEAD")[0];
const files = [
	...new Set([
		// --relative keeps paths inside anchor-ui and relative to it.
		...git("diff", "--name-only", "--relative", "--diff-filter=d", mergeBase),
		...git("ls-files", "--others", "--exclude-standard"),
	]),
].filter(
	(file) => /\.(ts|tsx|mjs)$/.test(file) && existsSync(path.join(root, file)),
);

if (files.length === 0) {
	console.log(`No changed .ts/.tsx/.mjs files since ${base}.`);
	process.exit(0);
}
const biome = path.join(root, "node_modules/@biomejs/biome/bin/biome");
const result = spawnSync(
	process.execPath,
	[biome, ...commands[tool], "--no-errors-on-unmatched", ...files],
	{ cwd: root, stdio: "inherit" },
);
process.exit(result.status ?? 1);
