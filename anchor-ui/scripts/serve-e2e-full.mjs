import { spawn } from "node:child_process";
import { randomBytes } from "node:crypto";
import { existsSync, writeFileSync } from "node:fs";
import path from "node:path";
import { preview } from "vite";
import { frontendFingerprint } from "../e2e/support/fingerprint.ts";
import { startRuntime, stopRuntime } from "./e2e-runtime.mjs";

const frontendURL = "http://127.0.0.1:3015";
const launchedAt = performance.now();
const cancellation = new AbortController();
let runtime;
let build;
let server;
let shuttingDown = false;
async function shutdown() {
	if (shuttingDown) return;
	shuttingDown = true;
	cancellation.abort();
	if (server) {
		// Release Vite's own signal handler before awaiting backend cleanup.
		const closing = server.close();
		server.httpServer.closeAllConnections();
		await closing;
	}
	if (!runtime && startup) {
		try {
			runtime = await startup;
		} catch {
			/* Runtime startup cleans its partial resources. */
		}
	}
	build?.kill("SIGTERM");
	if (runtime) await stopRuntime(runtime);
	process.exit(0);
}
process.on("SIGINT", shutdown);
process.on("SIGTERM", shutdown);
const startup = startRuntime({
	reuse: true,
	frontendURL,
	signal: cancellation.signal,
});
runtime = await startup;
const runtimeMs = performance.now() - launchedAt;
const ownerPath = path.join(runtime.directory, "owner.json");
if (!existsSync(ownerPath)) {
	writeFileSync(
		ownerPath,
		JSON.stringify({
			email: `owner-${runtime.runId}@example.test`,
			password: `LocalA1!${randomBytes(18).toString("hex")}`,
		}),
		{ mode: 0o600 },
	);
}

try {
	const buildStartedAt = performance.now();
	await new Promise((resolve, reject) => {
		build = spawn("pnpm", ["build"], {
			stdio: "inherit",
			env: {
				...process.env,
				VITE_API_BASE_URL: frontendURL,
				E2E_EMAIL: "",
				E2E_PASSWORD: "",
			},
		});
		build.once("error", reject);
		build.once("exit", (code) =>
			code === 0
				? resolve()
				: reject(new Error(`Frontend build exited ${code}`)),
		);
	});
	const fingerprint = frontendFingerprint();
	const frontendBuildMs = performance.now() - buildStartedAt;
	server = await preview({
		plugins: [
			{
				name: "e2e-ready",
				configurePreviewServer(previewServer) {
					server = previewServer;
					previewServer.middlewares.use(
						"/__e2e/ready",
						(_request, response) => {
							response.setHeader("Content-Type", "application/json");
							response.end(
								JSON.stringify({
									runId: runtime.runId,
									apiURL: runtime.apiURL,
									fingerprint,
									runtimeMs,
									frontendBuildMs,
									startupMs,
								}),
							);
						},
					);
				},
			},
		],
		preview: {
			host: "127.0.0.1",
			port: 3015,
			strictPort: true,
			proxy: {
				"/health": { target: runtime.apiURL, changeOrigin: true },
				"/v1": {
					target: runtime.apiURL,
					changeOrigin: true,
					cookieDomainRewrite: "",
				},
			},
		},
	});
	const startupMs = performance.now() - launchedAt;
	console.log(
		`Disposable Anchor browser runtime ready at ${frontendURL} (${Math.round(startupMs)} ms; backend ${Math.round(runtimeMs)} ms, frontend ${Math.round(frontendBuildMs)} ms)`,
	);
} catch (error) {
	await stopRuntime(runtime);
	throw error;
}
