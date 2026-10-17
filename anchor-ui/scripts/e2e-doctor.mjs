import { execFile } from "node:child_process";
import path from "node:path";
import { pathToFileURL } from "node:url";
import { promisify } from "node:util";
import { frontendFingerprint } from "../e2e/support/fingerprint.ts";
import { discoverCoverage } from "./e2e-coverage.mjs";
import { statusRuntime } from "./e2e-runtime.mjs";

const exec = promisify(execFile);
const directory = path.resolve(import.meta.dirname, "..");
const frontendURL = "http://127.0.0.1:3015";
const uiURL = "http://127.0.0.1:9351";

async function command(binary, args) {
	try {
		const { stdout } = await exec(binary, args, {
			cwd: directory,
			timeout: 10_000,
			maxBuffer: 1_048_576,
		});
		return { available: true, output: stdout.trim() };
	} catch {
		return { available: false };
	}
}

async function probe(url, json = false) {
	try {
		const response = await fetch(url, {
			signal: AbortSignal.timeout(2_000),
		});
		if (!response.ok) return null;
		return json ? await response.json() : true;
	} catch {
		return null;
	}
}

export function doctorReport({
	worktree,
	branch,
	commit,
	pnpm,
	docker,
	runtime,
	runtimeError,
	ready,
	fingerprint,
	uiListening,
	inventory,
}) {
	const matches =
		runtime?.active &&
		ready?.runId === runtime.runId &&
		ready?.apiURL === runtime.apiURL;
	return {
		worktree,
		branch: branch.output ?? null,
		commit: commit.output ?? null,
		tools: {
			node: process.version,
			pnpm: pnpm.output ?? "unavailable",
			docker: docker.output ?? "unavailable or daemon unreachable",
		},
		runtime: runtime
			? {
					owner: "this worktree",
					runId: runtime.runId,
					composeProject: runtime.composeProject,
					pid: runtime.pid,
					active: runtime.active,
				}
			: { owner: null, active: false, error: runtimeError },
		preview: {
			url: frontendURL,
			state: !ready
				? "unreachable"
				: !matches
					? "foreign or inactive runtime"
					: ready.fingerprint === fingerprint
						? "ready for this worktree"
						: "stale frontend",
		},
		ui: {
			url: uiURL,
			listening: Boolean(uiListening),
			owner: "not verified; confirm the UI uses this worktree",
		},
		inventory: inventory ?? { error: "Playwright discovery unavailable" },
	};
}

export async function collectDoctorReport() {
	const [branch, commit, pnpm, docker, local, ready, ui, inventory] =
		await Promise.all([
			command("git", ["branch", "--show-current"]),
			command("git", ["rev-parse", "HEAD"]),
			command("pnpm", ["--version"]),
			command("docker", ["version", "--format", "{{.Server.Version}}"]),
			statusRuntime().catch(() => ({ unreadable: true })),
			probe(`${frontendURL}/__e2e/ready`, true),
			probe(uiURL),
			discoverCoverage().catch(() => null),
		]);
	return doctorReport({
		worktree: path.resolve(directory, ".."),
		branch,
		commit,
		pnpm,
		docker,
		runtime: local?.unreadable ? null : local,
		runtimeError: local?.unreadable ? "unreadable ownership metadata" : null,
		ready,
		fingerprint: frontendFingerprint(),
		uiListening: ui,
		inventory,
	});
}

if (
	process.argv[1] &&
	pathToFileURL(path.resolve(process.argv[1])).href === import.meta.url
) {
	try {
		if (process.argv.slice(2).length === 0)
			console.log(JSON.stringify(await collectDoctorReport(), null, 2));
		else if (process.argv.length === 3 && process.argv[2] === "--help")
			console.log(
				"Usage: pnpm test:e2e:status\nRead-only worktree, tools, runtime ownership/health, URLs and Playwright inventory. Starts and stops no services; UI filters require a browser check.",
			);
		else throw new Error("Usage: pnpm test:e2e:status [--help]");
	} catch (error) {
		console.error(error.message);
		process.exitCode = 1;
	}
}
