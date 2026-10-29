import assert from "node:assert/strict";
import { spawn, spawnSync } from "node:child_process";
import {
	access,
	cp,
	mkdir,
	mkdtemp,
	readFile,
	rm,
	writeFile,
} from "node:fs/promises";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { collectPreflight, portAvailable } from "./e2e-preflight.mjs";

const previewPort = 3015;
const uiPort = 9351;
function fixture() {
	const calls = [];
	const runtime = {
		active: true,
		runId: "owned",
		apiURL: "http://127.0.0.1:4000",
		frontendURL: `http://127.0.0.1:${previewPort}`,
		fingerprint: "backend-current",
		secret: "must-not-leak",
	};
	return {
		calls,
		runtime,
		probes: {
			node: "24.17.0",
			requirements: async () => ({ pnpm: "11.1.0", go: "1.27.0" }),
			command: async (name, args) => {
				calls.push([name, ...args]);
				return name === "pnpm"
					? "11.1.2"
					: name === "go"
						? "go version go1.27.0 linux/amd64"
						: "installed";
			},
			dependencies: async () => {
				calls.push(["dependencies"]);
			},
			browser: async () => {
				calls.push(["browser"]);
			},
			runtime: async () => null,
			portAvailable: async () => true,
			sourceFingerprint: async () => "backend-current",
			frontendFingerprint: async () => "frontend-current",
			ready: async () => ({
				runId: "owned",
				apiURL: "http://127.0.0.1:4000",
				fingerprint: "frontend-current",
				password: "must-not-leak",
			}),
			credentials: async () => {},
		},
	};
}
function reusable() {
	const input = fixture();
	input.probes.runtime = async () => input.runtime;
	input.probes.portAvailable = async (port) => port !== previewPort;
	return input;
}
async function inspect(input, options = {}) {
	const report = await collectPreflight(options, input.probes);
	assert.equal(report.coverage, "prerequisites_only");
	assert.doesNotMatch(JSON.stringify(report), /must-not-leak/);
	return report;
}

test("a fresh checkout needs no running application and starts no test scenario", async () => {
	const input = fixture();
	const report = await inspect(input);
	assert.equal(report.status, "READY");
	assert.equal(report.mode, "fresh");
	assert.match(report.detail, /no scenarios/);
	assert.deepEqual(
		input.calls.filter(([name]) => name === "browser"),
		[["browser"]],
	);
	assert.ok(
		input.calls.every(
			([name]) => !["build", "up", "stop", "kill", "install"].includes(name),
		),
	);
});

test("an unsupported Node stops before probing tools, dependencies or services", async () => {
	for (const node of ["22.23.2", "25.0.0", "unknown"]) {
		const input = fixture();
		input.probes.node = node;
		assert.equal((await inspect(input)).reason, "node_version");
		assert.deepEqual(input.calls, []);
	}
});

test("missing and incompatible package managers return a specific corrective action", async () => {
	for (const value of ["10.0.0", "11.0.0", "12.0.0", "unknown", null]) {
		const input = fixture();
		input.probes.command = async () => {
			if (value === null) throw new Error("must-not-leak");
			return value;
		};
		const report = await inspect(input);
		assert.equal(
			report.reason,
			value === null ? "pnpm_unavailable" : "pnpm_version",
		);
		assert.match(report.next_action, /11.1.0/);
	}
});

test("missing project dependencies block before any Docker or browser startup", async () => {
	const input = fixture();
	input.probes.dependencies = async () => {
		throw new Error("must-not-leak");
	};
	const report = await inspect(input);
	assert.equal(report.status, "BLOCKED");
	assert.equal(report.reason, "dependency_missing");
	assert.match(report.next_action, /install --frozen-lockfile/);
	assert.ok(
		!input.calls.some(([name]) => ["docker", "browser"].includes(name)),
	);
});

test("unavailable or old backend toolchains do not reach Docker", async () => {
	for (const go of ["go version go1.26.5 linux/amd64", null]) {
		const input = fixture();
		const command = input.probes.command;
		input.probes.command = async (name, args) => {
			if (name !== "go") return command(name, args);
			if (go === null) throw new Error("must-not-leak");
			return go;
		};
		assert.equal(
			(await inspect(input)).reason,
			go === null ? "go_unavailable" : "go_version",
		);
		assert.ok(!input.calls.some(([name]) => name === "docker"));
	}
});

test("Docker, Compose, identity and browser failures retain distinct safe reasons", async () => {
	for (const [failure, reason] of [
		["daemon", "docker_unavailable"],
		["compose", "compose_unavailable"],
		["credentials", "development_identity_missing"],
		["browser", "browser_unavailable"],
	]) {
		const input = fixture();
		const command = input.probes.command;
		input.probes.command = async (name, args) => {
			if (
				name === "docker" &&
				args[0] ===
					(failure === "daemon"
						? "info"
						: failure === "compose"
							? "compose"
							: "never")
			)
				throw new Error("must-not-leak");
			return command(name, args);
		};
		if (["credentials", "browser"].includes(failure))
			input.probes[failure] = async () => {
				throw new Error("must-not-leak");
			};
		const report = await inspect(input);
		assert.equal(report.status, "BLOCKED");
		assert.equal(report.reason, reason);
	}
});

test("fresh runs refuse occupied preview ports and existing runtime metadata", async () => {
	const input = fixture();
	input.probes.portAvailable = async () => false;
	assert.equal((await inspect(input)).reason, "preview_port_in_use");
	input.probes.portAvailable = async () => true;
	input.probes.runtime = async () => input.runtime;
	assert.equal((await inspect(input)).reason, "runtime_exists");
	input.runtime.active = false;
	assert.equal((await inspect(input)).reason, "runtime_exists");
});

test("pending startup and invalid ownership block without attempting cleanup", async () => {
	const input = fixture();
	input.probes.runtime = async () => ({ pending: true });
	assert.equal((await inspect(input)).reason, "startup_pending");
	input.probes.runtime = async () => {
		throw new Error("must-not-leak");
	};
	assert.equal((await inspect(input)).reason, "ownership_unverified");
});

test("reuse requires a live owned backend and its requested loopback endpoints", async () => {
	const input = reusable();
	input.runtime.active = false;
	assert.equal(
		(await inspect(input, { mode: "reuse" })).reason,
		"runtime_unavailable",
	);
	input.runtime.active = true;
	input.runtime.apiURL = "https://foreign.invalid";
	assert.equal(
		(await inspect(input, { mode: "reuse" })).reason,
		"runtime_identity",
	);
});

test("a healthy frontend cannot conceal stale backend sources", async () => {
	const input = reusable();
	input.runtime.fingerprint = "previous-backend";
	assert.equal(
		(await inspect(input, { mode: "reuse" })).reason,
		"backend_stale",
	);
});

test("reuse rejects missing or unhealthy previews, including latched worker failures", async () => {
	const input = reusable();
	input.probes.portAvailable = async () => true;
	assert.equal(
		(await inspect(input, { mode: "reuse" })).reason,
		"preview_unavailable",
	);
	input.probes.portAvailable = async (port) => port !== previewPort;
	input.probes.ready = async () => null;
	assert.equal(
		(await inspect(input, { mode: "reuse" })).reason,
		"preview_unhealthy",
	);
	input.probes.ready = async () => {
		throw new Error("must-not-leak");
	};
	assert.equal(
		(await inspect(input, { mode: "reuse" })).reason,
		"reuse_unverified",
	);
});

test("an unrelated listener and an outdated frontend never pass reuse", async () => {
	const input = reusable();
	const ready = input.probes.ready;
	input.probes.ready = async () => ({ ...(await ready()), runId: "foreign" });
	assert.equal(
		(await inspect(input, { mode: "reuse" })).reason,
		"preview_identity",
	);
	input.probes.ready = async () => ({
		...(await ready()),
		fingerprint: "previous-frontend",
	});
	assert.equal(
		(await inspect(input, { mode: "reuse" })).reason,
		"frontend_stale",
	);
	input.probes.ready = async () => ({
		...(await ready()),
		apiURL: "http://127.0.0.1:4001",
	});
	assert.equal(
		(await inspect(input, { mode: "reuse" })).reason,
		"preview_identity",
	);
});

test("verified reuse passes while the UI launcher safely chooses fresh or reuse", async () => {
	const input = reusable();
	assert.equal((await inspect(input, { mode: "reuse" })).status, "READY");
	assert.equal(
		(await inspect(input, { mode: "auto", ui: true })).mode,
		"reuse",
	);
	assert.equal(
		(await inspect(fixture(), { mode: "auto", ui: true })).mode,
		"fresh",
	);
	input.probes.portAvailable = async () => false;
	assert.equal(
		(await inspect(input, { mode: "auto", ui: true })).reason,
		"ui_port_in_use",
	);
});

test("invalid preview ports and modes fail before any tool or network access", async () => {
	for (const port of [0, 80, 65536, Number.NaN, 3016.1]) {
		const input = fixture();
		assert.equal((await inspect(input, { port })).reason, "invalid_port");
		assert.deepEqual(input.calls, []);
	}
	assert.equal(
		(await inspect(fixture(), { mode: "unknown" })).reason,
		"invalid_mode",
	);
});

test("the real port probe detects an existing listener without closing or replacing it", async () => {
	const server = createServer();
	await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
	const port = server.address().port;
	try {
		assert.equal(await portAvailable(port), false);
		assert.equal(server.listening, true);
	} finally {
		await new Promise((resolve) => server.close(resolve));
	}
	assert.equal(await portAvailable(port), true);
});

async function cliFixture(t) {
	const root = await mkdtemp(path.join(tmpdir(), "e2e-preflight-test-"));
	t.after(() => rm(root, { recursive: true, force: true }));
	const app = path.join(root, "anchor-ui");
	const scripts = path.join(app, "scripts");
	await mkdir(scripts, { recursive: true });
	for (const file of ["e2e-preflight.mjs", "run-e2e.mjs"])
		await cp(path.join(import.meta.dirname, file), path.join(scripts, file));
	const module = path.join(root, "apps/anchor");
	await mkdir(module, { recursive: true });
	await writeFile(
		path.join(module, "go.mod"),
		"module fixture\ngo 1.27\ntoolchain go1.27.0\n",
	);
	await writeFile(
		path.join(app, "package.json"),
		JSON.stringify({ type: "module", packageManager: "pnpm@11.1.0" }),
	);
	const bin = path.join(root, "bin");
	await mkdir(bin);
	await writeFile(path.join(bin, "pnpm"), "#!/bin/sh\nprintf '11.1.0\\n'\n", {
		mode: 0o755,
	});
	const marker = path.join(root, "launched");
	const cli = path.join(app, "node_modules/playwright/cli.js");
	await mkdir(path.dirname(cli), { recursive: true });
	await writeFile(
		cli,
		`const fs = require('node:fs');fs.writeFileSync(${JSON.stringify(marker)}, JSON.stringify(process.argv.slice(2)));console.log('CLI forwarded');if(process.argv.includes('--wait'))setInterval(()=>{},1000);else process.exit(process.argv.includes('--list') ? 7 : 0);`,
	);
	const run = (file, args = []) =>
		spawnSync(process.execPath, [path.join(scripts, file), ...args], {
			cwd: root,
			encoding: "utf8",
			timeout: 10_000,
			env: { PATH: bin },
		});
	return { marker, run, scripts, bin, root };
}

test("the real app and review commands block before launching Playwright when installation is incomplete", async (t) => {
	const f = await cliFixture(t);
	for (const suite of ["app", "review"]) {
		const result = f.run("run-e2e.mjs", [suite, "a spec with spaces.e2e.ts"]);
		assert.equal(result.status, 1, result.stderr);
		assert.equal(JSON.parse(result.stdout).reason, "dependency_missing");
		await assert.rejects(access(f.marker));
	}
});

test("listing and help remain available without Docker and preserve CLI arguments and exit status", async (t) => {
	const f = await cliFixture(t);
	const listed = f.run("run-e2e.mjs", [
		"app",
		"--list",
		"a spec with spaces.e2e.ts",
	]);
	assert.equal(listed.status, 7, listed.stderr);
	assert.deepEqual(JSON.parse(await readFile(f.marker, "utf8")), [
		"test",
		"--config",
		"playwright.app.config.ts",
		"--list",
		"a spec with spaces.e2e.ts",
	]);
	const help = f.run("e2e-preflight.mjs", ["--help"]);
	assert.equal(help.status, 0, help.stderr);
	assert.match(help.stdout, /READY is not test coverage/);
	const invalid = f.run("e2e-preflight.mjs", ["--reuse=maybe"]);
	assert.equal(invalid.status, 1);
	assert.match(invalid.stderr, /Usage:/);
});

test("interrupting the wrapper terminates and awaits only its owned Playwright child", async (t) => {
	const f = await cliFixture(t);
	const child = spawn(
		process.execPath,
		[path.join(f.scripts, "run-e2e.mjs"), "app", "--list", "--wait"],
		{
			cwd: f.root,
			env: { PATH: f.bin },
			stdio: ["ignore", "pipe", "pipe"],
		},
	);
	const closed = new Promise((resolve, reject) => {
		child.once("error", reject);
		child.once("close", (code, signal) => resolve({ code, signal }));
	});
	let timer;
	try {
		await new Promise((resolve, reject) => {
			child.stdout.once("data", resolve);
			child.once("error", reject);
			timer = setTimeout(
				() => reject(new Error("The fixture child did not start")),
				5_000,
			);
		});
		clearTimeout(timer);
		child.kill("SIGTERM");
		const result = await closed;
		assert.deepEqual(result, { code: 143, signal: null });
	} finally {
		clearTimeout(timer);
		child.kill("SIGTERM");
		await closed;
	}
});

test("reuse validates the owned runtime identity instead of requiring bootstrap credentials again", async () => {
	const input = reusable();
	input.probes.credentials = async (runtime) => {
		assert.equal(runtime, input.runtime);
	};
	assert.equal((await inspect(input, { mode: "reuse" })).status, "READY");
	const fresh = fixture();
	fresh.probes.credentials = async (runtime) => {
		assert.equal(runtime, null);
	};
	assert.equal((await inspect(fresh)).status, "READY");
});
