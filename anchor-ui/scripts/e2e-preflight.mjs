import { execFile } from "node:child_process";
import { realpathSync } from "node:fs";
import { access, readFile } from "node:fs/promises";
import { createServer } from "node:net";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

const directory = path.resolve(import.meta.dirname, "..");
const repository = path.resolve(directory, "..");
const moduleDirectory = path.join(repository, "apps/anchor");
const defaultPort = 3015;
const uiPort = 9351;
const identityProvider = false;
const exec = promisify(execFile);
const timeout = 5_000;

function version(value) {
	const match = String(value).match(/(?:^|go|v)(\d+)\.(\d+)(?:\.(\d+))?/);
	return match ? match.slice(1).map((part) => Number(part ?? 0)) : null;
}

function atLeast(actual, required) {
	const a = version(actual);
	const b = version(required);
	if (!a || !b) return false;
	for (let i = 0; i < 3; i++) {
		if (a[i] !== b[i]) return a[i] > b[i];
	}
	return true;
}

async function exists(file) {
	try {
		await access(file);
		return true;
	} catch (error) {
		if (error.code === "ENOENT") return false;
		throw error;
	}
}

export async function portAvailable(port) {
	const server = createServer();
	try {
		await new Promise((resolve, reject) => {
			server.once("error", reject);
			server.listen(port, "127.0.0.1", resolve);
		});
		return true;
	} catch (error) {
		if (error.code === "EADDRINUSE") return false;
		throw error;
	} finally {
		if (server.listening) await new Promise((resolve) => server.close(resolve));
	}
}

export function defaultProbes() {
	let runtime;
	let dependencies;
	const loadRuntime = async () => {
		runtime ??= await import("./e2e-runtime.mjs");
		return runtime;
	};
	return {
		node: process.versions.node,
		async requirements() {
			const pkg = JSON.parse(
				await readFile(path.join(directory, "package.json"), "utf8"),
			);
			const go = await readFile(path.join(moduleDirectory, "go.mod"), "utf8");
			const pnpm = pkg.packageManager?.match(/^pnpm@(\d+\.\d+\.\d+)/)?.[1];
			const toolchain =
				go.match(/^toolchain go([\d.]+)$/m)?.[1] ??
				go.match(/^go ([\d.]+)$/m)?.[1];
			if (!pnpm || !toolchain) throw new Error("Missing tool versions");
			return { pnpm, go: toolchain };
		},
		async command(command, args) {
			const { stdout } = await exec(command, args, {
				cwd: command === "go" ? moduleDirectory : directory,
				timeout,
				maxBuffer: 65_536,
			});
			return stdout.trim();
		},
		async dependencies() {
			await access(path.join(directory, "node_modules/playwright/cli.js"));
			const [playwright] = await Promise.all([
				import("playwright"),
				import("vite"),
			]);
			dependencies = playwright;
		},
		async browser() {
			const browser = await dependencies.chromium.launch({
				headless: true,
				timeout,
			});
			await browser.close();
		},
		async runtime() {
			for (const name of ["startup.json", "startup.lock"])
				if (await exists(path.join(directory, "e2e/runtime/.local", name)))
					return { pending: true };
			return await (await loadRuntime()).statusRuntime();
		},
		async sourceFingerprint() {
			return await (await loadRuntime()).sourceFingerprint();
		},
		async frontendFingerprint() {
			return (
				await import("../e2e/support/fingerprint.ts")
			).frontendFingerprint();
		},
		async credentials() {
			if (identityProvider) await (await loadRuntime()).clerkCredentials();
		},
		portAvailable,
		async ready(port) {
			const response = await fetch(`http://127.0.0.1:${port}/__e2e/ready`, {
				signal: AbortSignal.timeout(2_000),
				redirect: "error",
			});
			if (!response.ok) return null;
			return await response.json();
		},
	};
}

export async function collectPreflight(
	{ mode = "fresh", ui = false, port = defaultPort } = {},
	probes = defaultProbes(),
) {
	const started = performance.now();
	const report = {
		status: "BLOCKED",
		mode,
		worktree: repository,
		port,
		coverage: "prerequisites_only",
	};
	const finish = (reason, detail, next_action) => ({
		...report,
		reason,
		detail,
		next_action,
		elapsed_ms: Math.round(performance.now() - started),
	});
	if (!["fresh", "reuse", "auto"].includes(mode))
		return finish(
			"invalid_mode",
			"Choose fresh start or explicit preview reuse.",
			"Run pnpm test:e2e:preflight --help.",
		);
	if (!Number.isInteger(port) || port < 1024 || port > 65535)
		return finish(
			"invalid_port",
			"The managed preview requires an unprivileged loopback port.",
			"Set E2E_PORT to an integer from 1024 to 65535.",
		);
	if (version(probes.node)?.[0] !== 24)
		return finish(
			"node_version",
			"Managed application tests require Node 24.",
			"Activate Node 24, then rerun the same command.",
		);
	let requirements;
	try {
		requirements = await probes.requirements();
	} catch {
		return finish(
			"tool_configuration",
			"Cannot read the committed tool requirements.",
			"Check package.json and the backend go.mod in this worktree.",
		);
	}
	let pnpm;
	try {
		pnpm = await probes.command("pnpm", ["--version"]);
	} catch {
		return finish(
			"pnpm_unavailable",
			"The required package manager is unavailable.",
			`Activate pnpm ${requirements.pnpm} from package.json.`,
		);
	}
	if (
		!atLeast(pnpm, requirements.pnpm) ||
		version(pnpm)?.[0] !== version(requirements.pnpm)?.[0]
	)
		return finish(
			"pnpm_version",
			"The package manager does not satisfy this worktree's requirements.",
			`Activate pnpm ${requirements.pnpm} or a compatible newer patch/minor in the same major.`,
		);
	try {
		await probes.dependencies();
	} catch {
		return finish(
			"dependency_missing",
			"Playwright or Vite is missing or cannot load in this worktree.",
			"Run pnpm install --frozen-lockfile from the repository's documented install directory.",
		);
	}
	let go;
	try {
		go = await probes.command("go", ["version"]);
	} catch {
		return finish(
			"go_unavailable",
			"The backend Go toolchain is unavailable or did not become ready within five seconds.",
			`Run go version in apps/anchor to prepare Go ${requirements.go}, then rerun.`,
		);
	}
	if (!atLeast(go, requirements.go))
		return finish(
			"go_version",
			"The backend Go toolchain is older than go.mod requires.",
			`Activate Go ${requirements.go} or newer, then rerun.`,
		);
	try {
		await probes.command("docker", ["info", "--format", "{{.ServerVersion}}"]);
	} catch {
		return finish(
			"docker_unavailable",
			"The Docker daemon is unavailable.",
			"Start Docker and verify docker info succeeds.",
		);
	}
	try {
		await probes.command("docker", ["compose", "version", "--short"]);
	} catch {
		return finish(
			"compose_unavailable",
			"Docker Compose is unavailable.",
			"Install the Docker Compose plugin and verify docker compose version.",
		);
	}
	let free;
	let runtime;
	try {
		free = await probes.portAvailable(port);
		if (ui && !(await probes.portAvailable(uiPort)))
			return finish(
				"ui_port_in_use",
				"The Playwright UI port is already occupied.",
				"Close the existing UI through its owning launcher before opening another.",
			);
		runtime = await probes.runtime();
	} catch {
		return finish(
			"ownership_unverified",
			"Cannot establish this worktree's runtime or port ownership.",
			"Inspect the owned runtime metadata and its manager; leave unrelated processes untouched.",
		);
	}
	if (mode === "auto") report.mode = free ? "fresh" : "reuse";
	if (runtime?.pending)
		return finish(
			"startup_pending",
			"This worktree has pending startup metadata or a startup lock.",
			"Wait for the owned manager, or stop that manager and verify owned cleanup before retrying.",
		);
	if (report.mode === "fresh") {
		if (!free)
			return finish(
				"preview_port_in_use",
				"The preview port is already occupied.",
				"Use --reuse only for a verified owned preview, or select another port where supported. Close a runtime through its owner.",
			);
		if (runtime)
			return finish(
				"runtime_exists",
				"This worktree already has runtime metadata; a fresh run requires owned cleanup.",
				"Stop its preview manager, then run node scripts/e2e-runtime.mjs stop and verify-stopped. Use --reuse for a ready preview.",
			);
	} else {
		if (!runtime?.active)
			return finish(
				"runtime_unavailable",
				"No healthy backend owned by this worktree is available for reuse.",
				"Inspect the owned manager's diagnostics, then start a managed preview from this worktree.",
			);
		if (free)
			return finish(
				"preview_unavailable",
				"The owned backend has no managed preview on the requested port.",
				"Start its managed preview explicitly or stop the backend's owner before a fresh run.",
			);
		try {
			if (
				runtime.frontendURL !== `http://127.0.0.1:${port}` ||
				new URL(runtime.apiURL).hostname !== "127.0.0.1"
			)
				return finish(
					"runtime_identity",
					"The runtime endpoints do not match this managed loopback preview.",
					"Use the preview belonging to this worktree and port.",
				);
			if (runtime.fingerprint !== (await probes.sourceFingerprint()))
				return finish(
					"backend_stale",
					"The owned backend no longer matches this worktree's sources or toolchain.",
					"Stop its owning preview manager and rebuild the managed preview.",
				);
			const ready = await probes.ready(port);
			if (!ready)
				return finish(
					"preview_unhealthy",
					"The managed preview or its worker is not ready.",
					"Inspect the owned preview and worker diagnostics before restarting it.",
				);
			if (ready.runId !== runtime.runId || ready.apiURL !== runtime.apiURL)
				return finish(
					"preview_identity",
					"The listener is not the preview associated with this worktree's runtime.",
					"Leave that listener untouched and inspect the owning manager or choose another port where supported.",
				);
			if (ready.fingerprint !== (await probes.frontendFingerprint()))
				return finish(
					"frontend_stale",
					"The managed frontend no longer matches this worktree's sources.",
					"Stop its owning preview manager and rebuild the managed preview.",
				);
		} catch {
			return finish(
				"reuse_unverified",
				"Cannot verify the source fingerprint or preview health for reuse.",
				"Inspect the owned manager and rebuild after correcting its reported failure.",
			);
		}
	}
	try {
		await probes.credentials(report.mode === "reuse" ? runtime : null);
	} catch {
		return finish(
			"development_identity_missing",
			"The documented development identity is unavailable or invalid.",
			report.mode === "reuse"
				? "Stop the owned preview manager and rebuild it with the documented Clerk development identity."
				: "Configure the Clerk development credentials described in e2e/README.md; production credentials are refused.",
		);
	}
	try {
		await probes.browser();
	} catch {
		return finish(
			"browser_unavailable",
			"Headless Chromium cannot start from this worktree's Playwright installation.",
			"Run pnpm exec playwright install --with-deps chromium, then rerun.",
		);
	}
	report.status = "READY";
	return finish(
		"none",
		"Managed test prerequisites are ready; no scenarios have run.",
		"Run the intended managed test selection.",
	);
}

export async function assertPreflight(options = {}) {
	const report = await collectPreflight({
		...options,
		port: options.port ?? defaultPort,
	});
	console.log(JSON.stringify(report));
	if (report.status !== "READY") {
		const error = new Error(
			`E2E preflight blocked: ${report.reason}. ${report.next_action}`,
		);
		error.code = "E2E_PREFLIGHT_BLOCKED";
		throw error;
	}
	return report;
}

export const preflightHelp =
	"Usage: pnpm test:e2e:preflight [--reuse] [--ui]\nChecks managed test prerequisites before execution. Default requires a fresh start; --reuse verifies an existing owned preview and both source fingerprints. --ui also checks the UI port. Starts no application services; briefly opens blank headless Chromium. READY is not test coverage.";
if (
	process.argv[1] &&
	realpathSync(process.argv[1]) === fileURLToPath(import.meta.url)
) {
	try {
		const args = process.argv.slice(2);
		if (args.length === 1 && args[0] === "--help") console.log(preflightHelp);
		else {
			if (
				args.some((arg) => !["--reuse", "--ui"].includes(arg)) ||
				new Set(args).size !== args.length
			)
				throw new Error(preflightHelp);
			await assertPreflight({
				mode: args.includes("--reuse") ? "reuse" : "fresh",
				ui: args.includes("--ui"),
			});
		}
	} catch (error) {
		console.error(error.message);
		process.exitCode = 1;
	}
}
