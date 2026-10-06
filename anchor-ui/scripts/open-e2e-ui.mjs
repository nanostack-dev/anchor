import { spawn } from "node:child_process";
import { createServer } from "node:net";
import path from "node:path";
import { setTimeout as delay } from "node:timers/promises";
import { fileURLToPath } from "node:url";
import { frontendFingerprint } from "../e2e/support/fingerprint.ts";
import { statusRuntime } from "./e2e-runtime.mjs";

const directory = path.resolve(import.meta.dirname, "..");
const frontendURL = "http://127.0.0.1:3015";
const cancellation = new AbortController();
const environment = { ...process.env, E2E_EMAIL: "", E2E_PASSWORD: "" };
let preview;
let ui;
let cleanup;

function launch(script, args = [], env = environment) {
	const child = spawn(process.execPath, [script, ...args], {
		cwd: directory,
		stdio: "inherit",
		detached: true,
		env,
	});
	const owned = { child, exited: false };
	owned.closed = new Promise((resolveClosed) => {
		child.once("error", (error) => {
			owned.exited = true;
			resolveClosed({ code: 1, error });
		});
		child.once("close", (code, signal) => {
			owned.exited = true;
			resolveClosed({ code, signal });
		});
	});
	return owned;
}

async function stop(owned, signal) {
	if (!owned) return;
	if (!owned.exited) owned.child.kill(signal);
	return await owned.closed;
}

function waitForUI() {
	return Promise.race([
		ui.closed,
		...(preview
			? [
					preview.closed.then(() => {
						throw new Error(
							"The owned preview exited while Playwright was running.",
						);
					}),
				]
			: []),
	]);
}

function shutdown() {
	cleanup ??= (async () => {
		await stop(ui, "SIGINT");
		return await stop(preview, "SIGTERM");
	})();
	return cleanup;
}

for (const signal of ["SIGINT", "SIGTERM"])
	process.on(signal, () => {
		cancellation.abort(new Error("Playwright UI interrupted."));
		void shutdown();
	});

async function readReady() {
	let response;
	try {
		response = await fetch(`${frontendURL}/__e2e/ready`, {
			signal: AbortSignal.any([
				cancellation.signal,
				AbortSignal.timeout(2_000),
			]),
		});
	} catch (error) {
		if (error.cause?.code === "ECONNREFUSED") return null;
		throw new Error("Cannot verify the preview on 127.0.0.1:3015.", {
			cause: error,
		});
	}
	if (!response.ok)
		throw new Error(
			`Refusing to reuse 127.0.0.1:3015: readiness returned ${response.status}.`,
		);
	try {
		const ready = await response.json();
		if (!ready || typeof ready !== "object" || Array.isArray(ready))
			throw new Error("Invalid readiness metadata.");
		return ready;
	} catch {
		throw new Error("Refusing to reuse an invalid preview readiness response.");
	}
}

async function verifyPreview(ready) {
	if (!ready) throw new Error("The owned preview is no longer ready.");
	const local = await statusRuntime();
	if (
		!ready ||
		!local?.active ||
		local.frontendURL !== frontendURL ||
		new URL(local.apiURL).hostname !== "127.0.0.1" ||
		ready.runId !== local.runId ||
		ready.apiURL !== local.apiURL ||
		ready.fingerprint !== frontendFingerprint()
	)
		throw new Error(
			"Refusing to reuse a foreign or stale preview. Stop its owner and restart the managed preview.",
		);
}

async function assertUIPortAvailable() {
	const probe = createServer();
	await new Promise((resolveListening, reject) => {
		probe.once("error", reject);
		probe.listen(9351, "127.0.0.1", resolveListening);
	});
	await new Promise((resolveClosed, reject) =>
		probe.close((error) => (error ? reject(error) : resolveClosed())),
	);
}

try {
	if (process.argv.length > 2)
		throw new Error("Usage: pnpm test:e2e:ui (select tests in the UI).");
	await assertUIPortAvailable();
	let ready = await readReady();
	if (!ready) {
		if ((await statusRuntime())?.active)
			throw new Error(
				"An owned backend is already running without a ready preview. Start its managed preview explicitly or stop that backend before opening UI.",
			);
		cancellation.signal.throwIfAborted();
		preview = launch(path.join(directory, "scripts/serve-e2e-full.mjs"));
		const deadline = Date.now() + 180_000;
		while (!ready) {
			cancellation.signal.throwIfAborted();
			if (preview.exited)
				throw new Error("The owned preview exited during startup.");
			if (Date.now() >= deadline)
				throw new Error("Timed out waiting for the owned preview.");
			ready = await readReady();
			if (!ready) await delay(200, undefined, { signal: cancellation.signal });
		}
	}
	await verifyPreview(ready);
	cancellation.signal.throwIfAborted();
	const playwright = path.join(
		path.dirname(fileURLToPath(import.meta.resolve("playwright/package.json"))),
		"cli.js",
	);
	const reuseEnvironment = { ...environment, E2E_REUSE_SERVER: "1" };
	ui = launch(
		playwright,
		["test", "--config", "playwright.app.config.ts", "--project=bootstrap"],
		reuseEnvironment,
	);
	const bootstrap = await waitForUI();
	if (bootstrap.error) throw bootstrap.error;
	if (bootstrap.code !== 0)
		throw new Error("Owned platform bootstrap failed; UI was not opened.");
	await verifyPreview(await readReady());
	cancellation.signal.throwIfAborted();
	ui = launch(
		playwright,
		[
			"test",
			"--config",
			"playwright.app.config.ts",
			"--ui",
			"--ui-host=127.0.0.1",
			"--ui-port=9351",
		],
		reuseEnvironment,
	);
	const result = await waitForUI();
	if (result.error) throw result.error;
	process.exitCode = cancellation.signal.aborted ? 0 : (result.code ?? 1);
} catch (error) {
	if (!cancellation.signal.aborted) {
		console.error(error.message);
		process.exitCode = 1;
	}
} finally {
	const stopped = await shutdown();
	if (stopped && (stopped.error || stopped.code !== 0)) {
		console.error(
			"Owned preview cleanup failed. Inspect its output and run node scripts/e2e-runtime.mjs verify-stopped.",
		);
		process.exitCode = 1;
	}
}
