import { spawn } from "node:child_process";
import { randomBytes, randomUUID } from "node:crypto";
import { createWriteStream } from "node:fs";
import {
	chmod,
	mkdir,
	readFile,
	readlink,
	rename,
	rm,
	writeFile,
} from "node:fs/promises";
import { createServer } from "node:net";
import { dirname, join, resolve } from "node:path";
import { setTimeout as delay } from "node:timers/promises";
import { fileURLToPath } from "node:url";
import { startRuntime, statusRuntime, stopRuntime } from "./e2e-runtime.mjs";
import { seedStripeDemoOrganizations } from "./stripe-prototype-bootstrap.mjs";

const scriptPath = fileURLToPath(import.meta.url);
const uiDirectory = resolve(dirname(scriptPath), "..");
const scratchDirectory = join(uiDirectory, ".ui-craft/stripe-prototype");
const sessionPath = join(scratchDirectory, "session.json");
const runtimeOwnerPath = join(scratchDirectory, "runtime-owner.json");
const lockDirectory = join(scratchDirectory, "launcher.lock");
const launcherPath = join(lockDirectory, "owner.json");
const frontendURL = "http://127.0.0.1:3307";
const events = [
	"checkout.session.completed",
	"customer.subscription.created",
	"customer.subscription.updated",
	"customer.subscription.deleted",
	"invoice.paid",
	"invoice.payment_failed",
	"invoice.payment_action_required",
	"radar.early_fraud_warning.created",
	"radar.early_fraud_warning.updated",
	"refund.created",
	"refund.updated",
	"refund.failed",
	"charge.dispute.created",
	"charge.dispute.updated",
].join(",");
const cancellation = new AbortController();
const children = new Set();
const processGroups = new Map();
const privateValues = new Set();
let runtime;
let ownsRuntime = false;
let ownsLock = false;
let stopping = false;
let logDirectory;
let launcherStartedAt;
let finish;
const finished = new Promise((done) => {
	finish = done;
});

function privateEnvironment() {
	return Object.fromEntries(
		Object.entries(process.env).filter(([name]) => name !== "STRIPE_API_KEY"),
	);
}

function scrub(value) {
	let result = String(value).replace(
		/\b(?:whsec_[A-Za-z0-9]+|oak_[A-Za-z0-9]+|(?:sk|rk|pk)_(?:test|live)_[A-Za-z0-9]+)\b/g,
		"[redacted]",
	);
	for (const secret of privateValues) {
		if (secret) result = result.replaceAll(secret, "[redacted]");
	}
	return result;
}

async function readJSON(path) {
	try {
		return JSON.parse(await readFile(path, "utf8"));
	} catch (error) {
		if (error.code === "ENOENT") return null;
		throw new Error("A private prototype metadata file is invalid.");
	}
}

async function writePrivateJSON(path, data) {
	const temporary = `${path}.${randomUUID()}.tmp`;
	await writeFile(temporary, `${JSON.stringify(data, null, 2)}\n`, {
		mode: 0o600,
	});
	await rename(temporary, path);
	await chmod(path, 0o600);
}

async function command(
	commandName,
	args,
	{ cwd = uiDirectory, timeout = 120_000, allowDuringShutdown = false } = {},
) {
	if (!allowDuringShutdown) cancellation.signal.throwIfAborted();
	return new Promise((complete, reject) => {
		const child = spawn(commandName, args, {
			cwd,
			env: privateEnvironment(),
			signal: allowDuringShutdown ? undefined : cancellation.signal,
			timeout,
			stdio: ["ignore", "pipe", "pipe"],
		});
		children.add(child);
		let output = "";
		child.stdout.on("data", (chunk) => {
			output = (output + chunk.toString()).slice(-1_048_576);
		});
		child.stderr.on("data", () => {});
		child.once("error", () =>
			reject(
				new Error(
					`${commandName} could not run. Check its installation and local login.`,
				),
			),
		);
		child.once("close", (code) => {
			children.delete(child);
			if (code === 0) complete(output.trim());
			else
				reject(
					new Error(
						`${commandName} failed. Check its installation and local sandbox login.`,
					),
				);
		});
	});
}

async function isLauncher(owner) {
	if (
		!Number.isInteger(owner?.pid) ||
		owner.pid <= 1 ||
		owner.script !== scriptPath
	)
		return false;
	try {
		const running = await inspectProcess(owner.pid);
		if (!running || (await processDirectory(owner.pid)) !== uiDirectory)
			return false;
		if (owner.started_at && owner.started_at !== running.startedAt)
			return false;
		const scriptArgument = running.command.trim().split(/\s+/)[1];
		return (
			scriptArgument === scriptPath ||
			scriptArgument === "scripts/stripe-prototype.mjs" ||
			scriptArgument === "./scripts/stripe-prototype.mjs"
		);
	} catch {
		return false;
	}
}

function parseProcesses(output) {
	return output.split("\n").flatMap((line) => {
		const matched = line.match(
			/^\s*(\d+)\s+(\d+)\s+(\S+\s+\S+\s+\d+\s+\d{2}:\d{2}:\d{2}\s+\d{4})\s+(.*)$/,
		);
		return matched
			? [
					{
						pid: Number(matched[1]),
						group: Number(matched[2]),
						startedAt: matched[3],
						command: matched[4],
					},
				]
			: [];
	});
}

async function inspectProcess(pid) {
	try {
		return parseProcesses(
			await command(
				"ps",
				["-p", String(pid), "-o", "pid=,pgid=,lstart=,command="],
				{ allowDuringShutdown: true },
			),
		)[0];
	} catch {
		return undefined;
	}
}

async function inspectGroup(group) {
	return parseProcesses(
		await command("ps", ["-axo", "pid=,pgid=,lstart=,command="], {
			allowDuringShutdown: true,
		}),
	).filter((entry) => entry.group === group);
}

async function processDirectory(pid) {
	if (process.platform === "linux") return readlink(`/proc/${pid}/cwd`);
	const output = await command(
		"lsof",
		["-a", "-p", String(pid), "-d", "cwd", "-Fn"],
		{ allowDuringShutdown: true },
	);
	return output
		.split("\n")
		.find((line) => line.startsWith("n"))
		?.slice(1);
}

async function persistLauncher() {
	if (!ownsLock) return;
	await writePrivateJSON(launcherPath, {
		pid: process.pid,
		script: scriptPath,
		started_at: launcherStartedAt,
		groups: [...processGroups.values()],
		owned_runtime_id: ownsRuntime ? runtime?.runId : null,
	});
}

async function verifiedRecordedGroup(record) {
	if (
		!Number.isInteger(record?.id) ||
		record.id <= 1 ||
		!Array.isArray(record.members)
	)
		throw new Error("Private prototype process ownership metadata is invalid.");
	const members = await inspectGroup(record.id);
	if (!members.length) return [];
	const knownMember = members.some((member) =>
		record.members.some(
			(saved) =>
				saved.pid === member.pid && saved.startedAt === member.startedAt,
		),
	);
	if (!knownMember)
		throw new Error(
			"A recorded process group has changed ownership; it was left running.",
		);
	for (const member of members) {
		if ((await processDirectory(member.pid)) !== record.cwd)
			throw new Error(
				"A recorded process group has changed directories; it was left running.",
			);
	}
	return members;
}

async function stopRecorded(owner) {
	if (owner?.script !== scriptPath) return;
	const groups = owner.groups ?? [];
	for (const record of groups) await verifiedRecordedGroup(record);
	for (const record of groups) {
		if (!(await verifiedRecordedGroup(record)).length) continue;
		process.kill(-record.id, "SIGTERM");
	}
	for (const record of groups) {
		for (let attempt = 0; attempt < 20; attempt++) {
			if (!(await inspectGroup(record.id)).length) break;
			await delay(250);
		}
		if ((await verifiedRecordedGroup(record)).length) {
			process.kill(-record.id, "SIGKILL");
			for (let attempt = 0; attempt < 20; attempt++) {
				if (!(await inspectGroup(record.id)).length) break;
				await delay(100);
			}
			if ((await inspectGroup(record.id)).length)
				throw new Error(
					"An owned process group is still exiting; private ownership metadata was kept for recovery.",
				);
		}
	}
}

async function teardownOwnedRuntime() {
	const owner = await readJSON(runtimeOwnerPath);
	const current = await statusRuntime();
	if (!current) return;
	if (
		owner?.script !== scriptPath ||
		owner.runtime_id !== current.runId ||
		owner.directory !== current.directory
	)
		throw new Error(
			"This launcher does not own the current Anchor runtime; it was left running.",
		);
	await stopRuntime(current);
	await rm(runtimeOwnerPath, { force: true });
}

async function claimLauncher() {
	try {
		await mkdir(lockDirectory, { mode: 0o700 });
	} catch (error) {
		if (error.code !== "EEXIST") throw error;
		const owner = await readJSON(launcherPath);
		if (await isLauncher(owner))
			throw new Error(
				"This worktree already has a running Stripe prototype. Stop it with pnpm stripe:prototype --stop.",
			);
		if (!owner)
			throw new Error(
				"A prototype launcher is starting. Retry after it finishes.",
			);
		await stopRecorded(owner);
		await rm(lockDirectory, { recursive: true, force: true });
		await mkdir(lockDirectory, { mode: 0o700 });
	}
	launcherStartedAt = (await inspectProcess(process.pid))?.startedAt;
	ownsLock = true;
	await persistLauncher();
}

async function assertPortFree(port) {
	const server = createServer();
	await new Promise((ready, reject) => {
		server.once("error", () =>
			reject(
				new Error(
					`Loopback port ${port} is already occupied. Stop its owner before launching this prototype.`,
				),
			),
		);
		server.listen(port, "127.0.0.1", ready);
	});
	await new Promise((closed, reject) =>
		server.close((error) => (error ? reject(error) : closed())),
	);
}

function requireLoopbackOrigin(value) {
	const url = new URL(value);
	if (
		url.protocol !== "http:" ||
		url.hostname !== "127.0.0.1" ||
		url.username ||
		url.password ||
		url.pathname !== "/" ||
		url.search ||
		url.hash
	)
		throw new Error("The managed Anchor runtime must use loopback origins.");
	return url.origin;
}

async function anchorRequest(path, { method = "GET", body, token, key } = {}) {
	const headers = { Accept: "application/json" };
	if (body !== undefined) headers["Content-Type"] = "application/json";
	if (token) headers.Authorization = `Bearer ${token}`;
	if (key) headers["X-Product-API-Key"] = key;
	let response;
	try {
		response = await fetch(`${runtime.apiURL}${path}`, {
			method,
			headers,
			body: body === undefined ? undefined : JSON.stringify(body),
			signal: AbortSignal.any([
				cancellation.signal,
				AbortSignal.timeout(30_000),
			]),
			redirect: "error",
		});
	} catch {
		throw new Error("The managed Anchor API could not be reached.");
	}
	if (!response.ok)
		throw new Error(
			`Anchor bootstrap ${method} ${path} failed with HTTP ${response.status}.`,
		);
	try {
		return await response.json();
	} catch {
		throw new Error(
			"The managed Anchor API returned an invalid bootstrap response.",
		);
	}
}

async function bootstrap(accountID) {
	const previous = await readJSON(sessionPath);
	if (
		previous?.runtime_id === runtime.runId &&
		previous.account_id !== accountID
	)
		throw new Error(
			"This product belongs to another Stripe sandbox. Select its saved sandbox before restarting.",
		);
	const ownerPath = join(runtime.directory, "owner.json");
	let owner = await readJSON(ownerPath);
	const health = await anchorRequest("/health");
	if (!owner && health.tenant_initialized)
		throw new Error(
			"This existing Anchor runtime has no saved owner credentials. Keep its current work intact and launch with a fresh managed runtime after its owner stops it.",
		);
	if (!owner) {
		owner = {
			email: `stripe-owner-${runtime.runId}@example.test`,
			password: `LocalA1!${randomBytes(24).toString("hex")}`,
		};
		await writePrivateJSON(ownerPath, owner);
	}
	privateValues.add(owner.password);
	if (!health.tenant_initialized)
		await anchorRequest("/v1/auth/register", {
			method: "POST",
			body: { ...owner, tenant_name: "Local Stripe billing prototype" },
		});
	const authentication = await anchorRequest("/v1/auth/login", {
		method: "POST",
		body: owner,
	});
	const token = authentication.accessToken;
	if (typeof token !== "string" || !token)
		throw new Error("Anchor did not return a bootstrap access token.");
	privateValues.add(token);
	if (previous?.runtime_id === runtime.runId) {
		await anchorRequest(`/v1/products/${previous.product_id}`, { token });
		return { session: previous, token, ownerPath };
	}
	const product = await anchorRequest("/v1/products", {
		method: "POST",
		token,
		body: {
			name: `Stripe Sandbox ${randomUUID().slice(0, 8)}`,
			description: "Local Stripe billing prototype",
		},
	});
	const productPath = `/v1/products/${product.id}`;
	await anchorRequest(`${productPath}/licensing/schema`, {
		method: "POST",
		token,
		body: {
			description:
				"Organization entitlements used by the Stripe sandbox prototype",
			fields: [
				{ name: "seats", type: "NUMBER", rules: { min: 1, max: 10000 } },
				{
					name: "monthly_runs",
					type: "LIMIT",
					usage_shape: "WINDOWED_COUNTER",
					rules: { min: 0, max: 10000000 },
				},
				{ name: "sso", type: "BOOLEAN" },
			],
		},
	});
	let fallbackID;
	for (const template of [
		{ name: "Free", values: { seats: 3, monthly_runs: 1000, sso: false } },
		{ name: "Pro", values: { seats: 15, monthly_runs: 50000, sso: false } },
		{
			name: "Business",
			values: { seats: 100, monthly_runs: 500000, sso: true },
		},
	]) {
		const created = await anchorRequest(`${productPath}/licensing/templates`, {
			method: "POST",
			token,
			body: template,
		});
		if (template.name === "Free") fallbackID = created.id;
	}
	await seedStripeDemoOrganizations(anchorRequest, {
		productPath,
		token,
		fallbackTemplateID: fallbackID,
		privateValues,
	});
	const session = {
		runtime_id: runtime.runId,
		account_id: accountID,
		product_id: product.id,
		product_name: product.name,
		fallback_template_id: fallbackID,
		state_path: join(
			scratchDirectory,
			`state-${runtime.runId}-${product.id}.json`,
		),
	};
	await writePrivateJSON(sessionPath, session);
	return { session, token, ownerPath };
}

async function configureStripe(session, token, webhookSecret) {
	const productPath = `/v1/products/${session.product_id}`;
	const instances = await anchorRequest(`${productPath}/integrations`, {
		token,
	});
	const existing = instances.items?.find(
		(instance) => instance.provider_type === "STRIPE",
	);
	if (
		(existing?.public_config?.account_id &&
			existing.public_config.account_id !== session.account_id) ||
		(existing?.public_config?.auth_method &&
			existing.public_config.auth_method !== "LOCAL_CLI")
	)
		throw new Error(
			"The product's Stripe configuration has changed. Review it in Anchor before using the local CLI launcher.",
		);
	const config = {
		auth_method: "LOCAL_CLI",
		account_id: session.account_id,
		webhook_secret: webhookSecret,
		return_url: frontendURL,
	};
	const instance = await anchorRequest(
		`${productPath}/integrations${existing ? `/${existing.id}` : ""}`,
		{
			method: existing ? "PUT" : "POST",
			token,
			body: existing
				? { is_enabled: true, config }
				: { provider_type: "STRIPE", config },
		},
	);
	await waitReady(async () => {
		const current = await anchorRequest(
			`${productPath}/integrations/${instance.id}`,
			{ token },
		);
		return current.is_enabled && current.status === "ACTIVE";
	}, "The native Stripe integration");
	const state = await anchorRequest(`${productPath}/billing/stripe`, {
		token,
	});
	if (!state.settings?.fallback_template_id)
		await anchorRequest(`${productPath}/billing/stripe/settings`, {
			method: "PUT",
			token,
			body: { fallback_template_id: session.fallback_template_id },
		});
	if (state.prices?.length === 0) {
		for (const [name, amount] of [
			["Pro", 2900],
			["Business", 9900],
		]) {
			const template = state.templates?.find(
				(candidate) => candidate.name === name && !candidate.archived,
			);
			if (!template)
				throw new Error(`The demo needs an active ${name} license template.`);
			await anchorRequest(`${productPath}/billing/stripe/prices`, {
				method: "POST",
				token,
				body: {
					name: `${name} monthly`,
					template_id: template.id,
					amount,
					currency: "usd",
					interval: "month",
				},
			});
		}
	}
	return instance.id;
}

async function launch(
	commandName,
	args,
	label,
	cwd = uiDirectory,
	environment = {},
) {
	cancellation.signal.throwIfAborted();
	const path = join(logDirectory, `${label}.log`);
	await writeFile(path, "", { mode: 0o600 });
	await chmod(path, 0o600);
	cancellation.signal.throwIfAborted();
	const log = createWriteStream(path, { flags: "a", mode: 0o600 });
	const child = spawn(commandName, args, {
		cwd,
		env: { ...privateEnvironment(), ...environment },
		detached: true,
		stdio: ["ignore", "pipe", "pipe"],
	});
	child.prototypeProcessGroup = true;
	children.add(child);
	let recent = "";
	for (const stream of [child.stdout, child.stderr]) {
		let pending = "";
		const write = (line) => {
			const output = scrub(line);
			recent = (recent + output).slice(-8192);
			log.write(output);
		};
		stream.on("data", (chunk) => {
			pending += chunk.toString();
			const lines = pending.split(/\r?\n|\r/);
			pending = lines.pop();
			for (const line of lines) write(`${line}\n`);
		});
		stream.once("end", () => {
			if (pending) write(pending);
		});
	}
	child.once("error", () => {
		if (!stopping)
			void shutdown(
				new Error(
					`${label} could not start. See private logs in ${logDirectory}.`,
				),
			);
	});
	child.once("close", (code) => {
		children.delete(child);
		log.end();
		if (!stopping)
			void shutdown(
				new Error(
					`${label} stopped (${code}). See private logs in ${logDirectory}.`,
				),
			);
	});
	await new Promise((spawned, reject) => {
		child.once("spawn", spawned);
		child.once("error", () => reject(new Error(`${label} could not start.`)));
	});
	processGroups.set(child.pid, {
		id: child.pid,
		cwd,
		members: await inspectGroup(child.pid),
	});
	await persistLauncher();
	return { child, output: () => recent };
}

async function waitReady(check, description, timeout = 60_000) {
	const deadline = Date.now() + timeout;
	while (Date.now() < deadline) {
		cancellation.signal.throwIfAborted();
		if (await check()) return;
		await delay(200, undefined, { signal: cancellation.signal });
	}
	throw new Error(
		`${description} did not become ready. See private logs in ${logDirectory}.`,
	);
}

async function probe(url) {
	try {
		return (await fetch(url, { signal: AbortSignal.timeout(2_000) })).ok;
	} catch {
		return false;
	}
}

async function shutdown(error) {
	if (stopping) return finished;
	stopping = true;
	if (error) {
		console.error(scrub(error.message));
		process.exitCode = 1;
	}
	cancellation.abort();
	const ownedChildren = [...children];
	let cleanupSucceeded = true;
	try {
		for (const record of processGroups.values()) {
			const members = await verifiedRecordedGroup(record);
			if (members.length) record.members = members;
		}
		await persistLauncher();
	} catch {
		cleanupSucceeded = false;
		process.exitCode = 1;
		console.error(
			"Process ownership inspection failed; private ownership metadata will be kept for recovery.",
		);
	}
	const stopChild = (child, signal) => {
		try {
			if (child.prototypeProcessGroup && child.pid)
				process.kill(-child.pid, signal);
			else child.kill(signal);
		} catch (failure) {
			if (failure.code !== "ESRCH") throw failure;
		}
	};
	for (const child of ownedChildren) stopChild(child, "SIGTERM");
	await Promise.all(
		ownedChildren.map(
			(child) =>
				new Promise((closed) => {
					if (child.exitCode !== null || child.signalCode !== null)
						return closed();
					const force = setTimeout(() => {
						stopChild(child, "SIGKILL");
						closed();
					}, 5000);
					child.once("close", () => {
						clearTimeout(force);
						closed();
					});
				}),
		),
	);
	try {
		await stopRecorded({
			script: scriptPath,
			groups: [...processGroups.values()],
		});
	} catch {
		cleanupSucceeded = false;
		process.exitCode = 1;
		console.error(
			"Owned process-group cleanup failed; private ownership metadata will be kept for recovery.",
		);
	}
	if (ownsLock && cleanupSucceeded) {
		const owner = await readJSON(launcherPath);
		if (owner?.pid === process.pid)
			await rm(lockDirectory, { recursive: true, force: true });
	}
	finish();
}

async function main() {
	if (process.argv.includes("--help")) {
		console.log(
			"Usage: pnpm stripe:prototype [--stop [--teardown]]\nStarts the regular Anchor app with its native Stripe sandbox integration and signed webhook listener. Ctrl-C or --stop stops owned UI/listener processes and preserves Anchor data. --stop --teardown additionally removes only a runtime created by this launcher.",
		);
		return;
	}
	await mkdir(scratchDirectory, { recursive: true, mode: 0o700 });
	await chmod(scratchDirectory, 0o700);
	if (process.argv.includes("--teardown") && !process.argv.includes("--stop"))
		throw new Error("Use --stop --teardown to remove an owned demo runtime.");
	if (process.argv.includes("--stop")) {
		const owner = await readJSON(launcherPath);
		if (!owner && process.argv.includes("--teardown")) {
			await teardownOwnedRuntime();
			console.log("Removed this launcher's disposable Anchor runtime.");
			return;
		}
		if (owner?.script !== scriptPath)
			throw new Error(
				"No owned Stripe prototype launcher is running in this worktree.",
			);
		if (await isLauncher(owner)) {
			process.kill(owner.pid, "SIGTERM");
			for (let attempt = 0; attempt < 30; attempt++) {
				if (!(await isLauncher(owner))) break;
				await delay(500);
			}
			if (await isLauncher(owner))
				throw new Error(
					"The owned launcher is still shutting down. Retry --stop shortly.",
				);
		}
		await stopRecorded(owner);
		const currentOwner = await readJSON(launcherPath);
		if (currentOwner?.pid === owner.pid && currentOwner.script === owner.script)
			await rm(lockDirectory, { recursive: true, force: true });
		if (process.argv.includes("--teardown")) await teardownOwnedRuntime();
		console.log("Stopped this worktree's Stripe prototype processes.");
		return;
	}
	await claimLauncher();
	process.on("SIGINT", () => void shutdown());
	process.on("SIGTERM", () => void shutdown());
	process.on("SIGHUP", () => void shutdown());
	await assertPortFree(3307);
	console.log(
		"Verifying Stripe sandbox login and preparing the local prototype…",
	);
	let account;
	try {
		account = JSON.parse(
			await command("stripe", ["whoami", "--format", "json", "--color", "off"]),
		);
	} catch {
		throw new Error(
			"Stripe CLI is not logged in. Run stripe login and select your sandbox account.",
		);
	}
	if (
		!/^acct_[A-Za-z0-9]+$/.test(account.account_id ?? "") ||
		account.mode !== "test"
	)
		throw new Error(
			"The prototype requires Stripe sandbox mode. Live mode is refused.",
		);
	const existing = await statusRuntime();
	if (existing?.active) {
		runtime = existing;
		console.log(
			"Reusing the running Anchor backend in this worktree; its owner keeps control of shutdown.",
		);
	} else {
		if (existing)
			throw new Error(
				"An inactive managed Anchor runtime remains. Have its owner clean it up before launching a fresh prototype.",
			);
		runtime = await startRuntime({
			frontendURL,
			requireFresh: true,
			signal: cancellation.signal,
		});
		ownsRuntime = true;
		await writePrivateJSON(runtimeOwnerPath, {
			script: scriptPath,
			runtime_id: runtime.runId,
			directory: runtime.directory,
		});
		if (stopping) {
			return;
		}
		await persistLauncher();
	}
	runtime.apiURL = requireLoopbackOrigin(runtime.apiURL);
	runtime.frontendURL = requireLoopbackOrigin(runtime.frontendURL);
	const { session, token, ownerPath } = await bootstrap(account.account_id);
	const secretOutput = await command("stripe", [
		"listen",
		"--print-secret",
		"--color",
		"off",
		"--skip-update",
	]);
	const webhookSecret = secretOutput.match(/\bwhsec_[A-Za-z0-9]+\b/)?.[0];
	if (!webhookSecret)
		throw new Error("Stripe did not return a sandbox webhook signing secret.");
	privateValues.add(webhookSecret);
	logDirectory = join(scratchDirectory, "runs", randomUUID());
	await mkdir(logDirectory, { recursive: true, mode: 0o700 });
	const integrationID = await configureStripe(session, token, webhookSecret);
	await writePrivateJSON(sessionPath, {
		...session,
		integration_instance_id: integrationID,
	});
	const webhookURL = `${runtime.apiURL}/v1/products/${session.product_id}/billing/stripe/webhook`;
	const listener = await launch(
		"stripe",
		[
			"listen",
			`--events=${events}`,
			"--events-from=@self",
			`--forward-to=${webhookURL}`,
			"--color",
			"off",
			"--skip-update",
		],
		"stripe-listener",
	);
	await waitReady(
		() => /Ready!|ready to receive/i.test(listener.output()),
		"The Stripe webhook listener",
	);
	await launch(
		"pnpm",
		["exec", "vite", "--host", "127.0.0.1", "--port", "3307", "--strictPort"],
		"frontend",
		uiDirectory,
		{
			VITE_API_BASE_URL: frontendURL,
			ANCHOR_API_PROXY_TARGET: runtime.apiURL,
			ANCHOR_API_PROXY_ORIGIN: runtime.frontendURL,
		},
	);
	await waitReady(() => probe(`${frontendURL}/login`), "The regular Anchor UI");
	for (const record of processGroups.values())
		record.members = await inspectGroup(record.id);
	await persistLauncher();
	console.log(
		`Anchor with Stripe sandbox ready: ${frontendURL}\nSign in with the local owner credentials saved privately in ${ownerPath}. Select product ${session.product_name}, then open Integrations → Stripe or Pricing.\nCtrl-C stops the UI and webhook listener. Anchor will remain running to preserve the demo data.`,
	);
	await finished;
}

try {
	await main();
} catch (error) {
	await shutdown(error);
}
