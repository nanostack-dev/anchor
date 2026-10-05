import { spawn } from "node:child_process";
import { createHash, randomBytes, randomUUID } from "node:crypto";
import {
	access,
	mkdir,
	open,
	readFile,
	readdir,
	rename,
	rm,
	writeFile,
} from "node:fs/promises";
import { createServer } from "node:net";
import { dirname, join, resolve } from "node:path";
import { setTimeout as delay } from "node:timers/promises";
import { fileURLToPath, pathToFileURL } from "node:url";

const uiDirectory = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const repositoryDirectory = resolve(uiDirectory, "..");
const appDirectory = join(repositoryDirectory, "apps", "anchor");
const runtimeDirectory = join(uiDirectory, "e2e", "runtime");
const localDirectory = join(runtimeDirectory, ".local");
const metadataPath = join(localDirectory, "runtime.json");
const pendingMetadataPath = join(localDirectory, "startup.json");
const composePath = join(runtimeDirectory, "docker-compose.yaml");
const projectPrefix = `anchor-e2e-${createHash("sha256").update(repositoryDirectory).digest("hex").slice(0, 10)}-`;

function run(command, args, options = {}) {
	return new Promise((resolveResult, reject) => {
		const child = spawn(command, args, {
			cwd: repositoryDirectory,
			timeout: 120_000,
			...options,
		});
		let output = "";
		for (const stream of [child.stdout, child.stderr]) {
			stream?.on("data", (chunk) => {
				output = (output + chunk.toString()).slice(-32_768);
			});
		}
		child.once("error", reject);
		child.once("close", (code, signal) => {
			if (code === 0) resolveResult(output.trim());
			else {
				reject(
					new Error(
						`${command} ${args.join(" ")} failed (${signal ?? code}).\n${output}`,
					),
				);
			}
		});
	});
}

function compose(metadata, args, options = {}) {
	return run(
		"docker",
		[
			"compose",
			"--project-name",
			metadata.composeProject,
			"--file",
			composePath,
			...args,
		],
		{
			...options,
			env: {
				...process.env,
				ANCHOR_E2E_RUN_ID: metadata.runId,
				ANCHOR_E2E_RUN_DIRECTORY: metadata.directory,
			},
		},
	);
}

async function readMetadata(path = metadataPath) {
	try {
		return JSON.parse(await readFile(path, "utf8"));
	} catch (error) {
		if (error.code === "ENOENT") return null;
		throw error;
	}
}

async function writeMetadata(metadata, path = metadataPath) {
	const temporaryPath = `${path}.${metadata.runId}.tmp`;
	await writeFile(temporaryPath, `${JSON.stringify(metadata, null, 2)}\n`, {
		mode: 0o600,
	});
	await rename(temporaryPath, path);
}

async function sourceFingerprint() {
	const hash = createHash("sha256");
	hash.update(`${process.platform}/${process.arch}`);
	hash.update(
		await run("go", ["env", "GOVERSION", "GOOS", "GOARCH"], {
			cwd: appDirectory,
		}),
	);
	async function visit(directory) {
		const entries = (await readdir(directory, { withFileTypes: true })).sort(
			(a, b) => a.name.localeCompare(b.name),
		);
		for (const entry of entries) {
			const path = join(directory, entry.name);
			if (entry.isDirectory()) {
				if (![".git", "node_modules", "worktrees"].includes(entry.name))
					await visit(path);
			} else if (
				entry.isFile() &&
				/(?:\.go|\.sql|\.ya?ml|go\.mod|go\.sum)$/.test(entry.name) &&
				!entry.name.endsWith("_test.go")
			) {
				hash.update(path);
				hash.update(await readFile(path));
			}
		}
	}
	await visit(appDirectory);
	await visit(join(repositoryDirectory, "clients", "go"));
	hash.update(await readFile(composePath));
	hash.update(await readFile(join(runtimeDirectory, "application.yaml")));
	return hash.digest("hex").slice(0, 20);
}

async function allocatePort() {
	const server = createServer();
	await new Promise((resolveReady, reject) => {
		server.once("error", reject);
		server.listen(0, "127.0.0.1", resolveReady);
	});
	const port = server.address().port;
	await new Promise((resolveClosed, reject) =>
		server.close((error) => (error ? reject(error) : resolveClosed())),
	);
	return port;
}

function assertOwned(metadata) {
	if (
		!/^\w{8}-\w{4}-\w{4}-\w{4}-\w{12}$/.test(metadata?.runId ?? "") ||
		metadata.composeProject !==
			`${projectPrefix}${metadata.runId.slice(0, 8)}` ||
		metadata.directory !== join(localDirectory, "runs", metadata.runId) ||
		(metadata.executable &&
			!metadata.executable.startsWith(join(localDirectory, "bin", "anchor-")))
	) {
		throw new Error(
			"Refusing to manage an Anchor runtime that does not belong to this worktree.",
		);
	}
}

async function ownsBackend(metadata) {
	if (!metadata.pid || !metadata.executable) return false;
	try {
		const command = await run("ps", [
			"-p",
			String(metadata.pid),
			"-o",
			"command=",
		]);
		return command === metadata.executable;
	} catch {
		return false;
	}
}

async function probe(url) {
	try {
		const response = await fetch(url, { signal: AbortSignal.timeout(2_000) });
		return response.ok;
	} catch {
		return false;
	}
}

async function poll(check, description, timeout = 60_000, signal = undefined) {
	const deadline = Date.now() + timeout;
	while (Date.now() < deadline) {
		signal?.throwIfAborted();
		if (await check()) return;
		await delay(200, undefined, { signal });
	}
	throw new Error(`Timed out waiting for ${description}.`);
}

async function mappedPort(metadata, service, port) {
	const address = await compose(metadata, ["port", service, String(port)]);
	const parsed = Number(address.split(":").at(-1));
	if (!Number.isInteger(parsed) || parsed < 1)
		throw new Error(`Invalid Docker port for ${service}: ${address}`);
	return parsed;
}

export async function statusRuntime() {
	const metadata = await readMetadata();
	if (!metadata) return null;
	assertOwned(metadata);
	return {
		...metadata,
		active:
			(await ownsBackend(metadata)) &&
			(await probe(`${metadata.apiURL}/health`)),
	};
}

export async function stopRuntime(metadata) {
	const target =
		metadata ??
		(await readMetadata()) ??
		(await readMetadata(pendingMetadataPath));
	if (!target) return;
	assertOwned(target);
	if (await ownsBackend(target)) {
		process.kill(target.pid, "SIGTERM");
		try {
			await poll(
				async () => !(await ownsBackend(target)),
				"owned Anchor API shutdown",
				10_000,
			);
		} catch {
			if (await ownsBackend(target)) process.kill(target.pid, "SIGKILL");
		}
	}
	await compose(target, [
		"down",
		"--volumes",
		"--remove-orphans",
		"--timeout",
		"5",
	]);
	const current = await readMetadata();
	if (current?.runId === target.runId) await rm(metadataPath, { force: true });
	const pending = await readMetadata(pendingMetadataPath);
	if (pending?.runId === target.runId)
		await rm(pendingMetadataPath, { force: true });
	await Promise.all(
		["postgres-password", "jwt-secret", "encryption-key", "owner.json"].map(
			(name) => rm(join(target.directory, name), { force: true }),
		),
	);
}

export async function startRuntime({
	reuse = true,
	frontendURL = "http://127.0.0.1:3015",
	signal,
} = {}) {
	if (new URL(frontendURL).hostname !== "127.0.0.1")
		throw new Error("The E2E frontend must use 127.0.0.1.");
	await mkdir(localDirectory, { recursive: true, mode: 0o700 });
	const lockDirectory = join(localDirectory, "startup.lock");
	try {
		await mkdir(lockDirectory);
	} catch (error) {
		if (error.code !== "EEXIST") throw error;
		const owner = await readMetadata(join(lockDirectory, "owner.json"));
		let ownerRunning = true;
		if (Number.isInteger(owner?.pid)) {
			try {
				process.kill(owner.pid, 0);
			} catch (probeError) {
				if (probeError.code === "ESRCH") ownerRunning = false;
			}
		}
		if (ownerRunning)
			throw new Error(
				`Anchor E2E runtime startup is already in progress (${lockDirectory}).`,
			);
		await rm(lockDirectory, { recursive: true, force: true });
		await mkdir(lockDirectory);
	}
	await writeFile(
		join(lockDirectory, "owner.json"),
		JSON.stringify({ pid: process.pid }),
		{ mode: 0o600 },
	);
	let metadata;
	try {
		signal?.throwIfAborted();
		const pending = await readMetadata(pendingMetadataPath);
		if (pending) await stopRuntime(pending);
		const fingerprint = await sourceFingerprint();
		const existing = await statusRuntime();
		if (
			existing?.active &&
			reuse &&
			existing.fingerprint === fingerprint &&
			existing.frontendURL === frontendURL
		) {
			const { active: _, ...reused } = existing;
			return reused;
		}
		if (existing) await stopRuntime(existing);
		await run("docker", ["info", "--format", "{{.ServerVersion}}"], { signal });
		const runId = randomUUID();
		metadata = {
			runId,
			composeProject: `${projectPrefix}${runId.slice(0, 8)}`,
			directory: join(localDirectory, "runs", runId),
			frontendURL,
			fingerprint,
			startedAt: new Date().toISOString(),
		};
		await mkdir(metadata.directory, { recursive: true, mode: 0o700 });
		for (const [name, value] of [
			["postgres-password", randomBytes(24).toString("hex")],
			["jwt-secret", randomBytes(32).toString("base64")],
			["encryption-key", randomBytes(32).toString("base64")],
		])
			await writeFile(join(metadata.directory, name), value, { mode: 0o600 });
		await writeMetadata(metadata, pendingMetadataPath);
		await compose(
			metadata,
			["up", "--detach", "--wait", "--wait-timeout", "90"],
			{ signal },
		);
		metadata.postgresPort = await mappedPort(metadata, "postgres", 5432);
		metadata.redisPort = await mappedPort(metadata, "redis", 6379);
		metadata.smtpPort = await mappedPort(metadata, "mailpit", 1025);
		metadata.mailpitURL = `http://127.0.0.1:${await mappedPort(metadata, "mailpit", 8025)}`;
		await poll(
			() => probe(`${metadata.mailpitURL}/api/v1/messages`),
			"Mailpit HTTP API",
			60_000,
			signal,
		);
		const binDirectory = join(localDirectory, "bin");
		await mkdir(binDirectory, { recursive: true });
		metadata.executable = join(binDirectory, `anchor-${fingerprint}`);
		try {
			await access(metadata.executable);
		} catch {
			await run("go", ["build", "-o", metadata.executable, "./cmd"], {
				cwd: appDirectory,
				timeout: 300_000,
				signal,
				env: {
					...process.env,
					GOCACHE: process.env.GOCACHE || join(localDirectory, "go-build"),
				},
			});
		}
		const apiPort = await allocatePort();
		metadata.apiURL = `http://127.0.0.1:${apiPort}`;
		const logFile = await open(join(metadata.directory, "api.log"), "a", 0o600);
		try {
			const env = Object.fromEntries(
				["PATH", "HOME", "TMPDIR", "TMP", "TEMP", "LANG", "LC_ALL", "TZ"]
					.filter((name) => process.env[name] !== undefined)
					.map((name) => [name, process.env[name]]),
			);
			Object.assign(env, {
				CONFIG_PATH: join(runtimeDirectory, "application.yaml"),
				ANCHOR_E2E_MIGRATIONS_PATH: join(appDirectory, "migrations"),
				POSTGRES_PORT: String(metadata.postgresPort),
				POSTGRES_PASSWORD_FILE: join(metadata.directory, "postgres-password"),
				ADMIN_JWT_SECRET_FILE: join(metadata.directory, "jwt-secret"),
				APP_ENCRYPTION_KEY_FILE: join(metadata.directory, "encryption-key"),
				REDIS_ADDRESS: `127.0.0.1:${metadata.redisPort}`,
				SERVER_PORT: String(apiPort),
				ALLOWED_ORIGIN: frontendURL,
				ENVIRONMENT: "development",
				LOG_LEVEL: "warn",
			});
			const child = spawn(metadata.executable, [], {
				cwd: appDirectory,
				env,
				detached: true,
				stdio: ["ignore", logFile.fd, logFile.fd],
			});
			await new Promise((resolveSpawned, reject) => {
				child.once("spawn", resolveSpawned);
				child.once("error", reject);
			});
			metadata.pid = child.pid;
			child.unref();
		} finally {
			await logFile.close();
		}
		await writeMetadata(metadata, pendingMetadataPath);
		await poll(
			async () => {
				if (!(await ownsBackend(metadata))) {
					throw new Error(
						`Anchor API exited during startup. Inspect ${join(metadata.directory, "api.log")}.`,
					);
				}
				return probe(`${metadata.apiURL}/health`);
			},
			"Anchor API and database migrations",
			120_000,
			signal,
		);
		await writeMetadata(metadata);
		await rm(pendingMetadataPath, { force: true });
		return metadata;
	} catch (error) {
		if (metadata) {
			try {
				await stopRuntime(metadata);
			} catch (cleanupError) {
				error.message += `\nRuntime cleanup failed: ${cleanupError.message}`;
			}
		}
		throw error;
	} finally {
		await rm(lockDirectory, { recursive: true, force: true });
	}
}

if (
	process.argv[1] &&
	pathToFileURL(resolve(process.argv[1])).href === import.meta.url
) {
	try {
		const command = process.argv[2] ?? "start";
		if (command === "stop") {
			await stopRuntime();
			console.log("Stopped this worktree's Anchor E2E runtime.");
		} else if (command === "status") {
			console.log(JSON.stringify(await statusRuntime(), null, 2));
		} else if (command === "verify-stopped") {
			for (const name of ["runtime.json", "startup.json", "startup.lock"]) {
				try {
					await access(join(localDirectory, name));
				} catch (error) {
					if (error.code === "ENOENT") continue;
					throw error;
				}
				throw new Error(
					`Anchor E2E cleanup left ${name}. Run the owned runtime stop command.`,
				);
			}
			const projects = await run("docker", [
				"ps",
				"-a",
				"--filter",
				"label=com.docker.compose.project",
				"--format",
				'{{.Label "com.docker.compose.project"}}',
			]);
			if (
				projects
					.split("\n")
					.some((project) => project.startsWith(projectPrefix))
			)
				throw new Error(
					"Anchor E2E cleanup left containers owned by this worktree.",
				);
			console.log("Anchor E2E runtime cleanup verified.");
		} else if (command === "start") {
			const controller = new AbortController();
			const cancelStartup = () =>
				controller.abort(new Error("Runtime startup interrupted."));
			process.once("SIGINT", cancelStartup);
			process.once("SIGTERM", cancelStartup);
			let metadata;
			try {
				metadata = await startRuntime({
					reuse: !process.argv.includes("--fresh"),
					signal: controller.signal,
				});
			} finally {
				process.removeListener("SIGINT", cancelStartup);
				process.removeListener("SIGTERM", cancelStartup);
			}
			console.log(JSON.stringify(metadata, null, 2));
			if (!process.argv.includes("--detach")) {
				await new Promise((resolveStopped) => {
					let stopping = false;
					const shutdown = async () => {
						if (stopping) return;
						stopping = true;
						try {
							await stopRuntime(metadata);
						} catch (error) {
							console.error(error.message);
							process.exitCode = 1;
						}
						clearInterval(keepAlive);
						resolveStopped();
					};
					process.once("SIGINT", shutdown);
					process.once("SIGTERM", shutdown);
					const keepAlive = setInterval(() => {}, 60_000);
				});
			}
		} else
			throw new Error(
				"Usage: node scripts/e2e-runtime.mjs start [--fresh] [--detach] | stop | status | verify-stopped",
			);
	} catch (error) {
		console.error(error.message);
		process.exitCode = 1;
	}
}
