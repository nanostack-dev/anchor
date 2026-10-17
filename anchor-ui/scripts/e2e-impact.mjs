import { execFileSync } from "node:child_process";
import { readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import { pathToFileURL } from "node:url";

function git(cwd, args) {
	return execFileSync("git", args, {
		cwd,
		encoding: "utf8",
		maxBuffer: 16 * 1024 * 1024,
		stdio: ["ignore", "pipe", "pipe"],
	});
}

function nameStatus(output, source) {
	const fields = output.split("\0");
	if (fields.pop() !== "")
		throw new Error("Git returned an incomplete NUL-delimited diff");
	const changes = [];
	while (fields.length) {
		const status = fields.shift();
		const count = /^[RC]\d+$/.test(status)
			? 2
			: /^[AMDUTXB]$/.test(status)
				? 1
				: 0;
		if (!count || fields.length < count)
			throw new Error(`Unsupported Git change status: ${status}`);
		const paths = fields.splice(0, count);
		if (paths.some((file) => !file))
			throw new Error("Git returned an empty changed path");
		changes.push({ status, paths, source });
	}
	return changes;
}

export function collectGitChanges({
	cwd = process.cwd(),
	base = process.env.E2E_BASE_REF || "origin/main",
	head = "HEAD",
	includeWorkingTree = true,
} = {}) {
	const result = {
		changes: [],
		base,
		mergeBase: null,
		head: null,
		gitError: null,
		repositoryRoot: null,
	};
	try {
		if (
			typeof base !== "string" ||
			!base.trim() ||
			typeof head !== "string" ||
			!head.trim()
		)
			throw new Error("Base and head must be nonempty Git refs");
		result.repositoryRoot = git(cwd, ["rev-parse", "--show-toplevel"]).trim();
		cwd = result.repositoryRoot;
		const baseCommit = git(cwd, [
			"rev-parse",
			"--verify",
			"--end-of-options",
			`${base}^{commit}`,
		]).trim();
		result.head = git(cwd, [
			"rev-parse",
			"--verify",
			"--end-of-options",
			`${head}^{commit}`,
		]).trim();
		result.mergeBase = git(cwd, ["merge-base", baseCommit, result.head]).trim();
		result.changes.push(
			...nameStatus(
				git(cwd, [
					"diff",
					"--no-relative",
					"--name-status",
					"-z",
					"--find-renames",
					result.mergeBase,
					result.head,
					"--",
				]),
				"committed",
			),
		);
		if (includeWorkingTree) {
			result.changes.push(
				...nameStatus(
					git(cwd, [
						"diff",
						"--cached",
						"--no-relative",
						"--name-status",
						"-z",
						"--find-renames",
						"--",
					]),
					"staged",
				),
			);
			result.changes.push(
				...nameStatus(
					git(cwd, [
						"diff",
						"--no-relative",
						"--name-status",
						"-z",
						"--find-renames",
						"--",
					]),
					"unstaged",
				),
			);
			const untracked = git(cwd, [
				"ls-files",
				"--full-name",
				"--others",
				"--exclude-standard",
				"-z",
			]).split("\0");
			if (untracked.pop() !== "")
				throw new Error("Git returned an incomplete untracked-file list");
			result.changes.push(
				...untracked.map((file) => ({
					status: "A",
					paths: [file],
					source: "untracked",
				})),
			);
		}
	} catch (error) {
		result.gitError = `Cannot safely compare Git base ${JSON.stringify(base)} and head ${JSON.stringify(head)}: ${error.message.split("\n")[0]}`;
	}
	return result;
}

function matches(file, pattern) {
	if (pattern.endsWith("/**")) return file.startsWith(pattern.slice(0, -2));
	if (pattern.startsWith("**/*.")) return file.endsWith(pattern.slice(4));
	return file === pattern;
}

function repositoryPath(value) {
	return (
		typeof value === "string" &&
		value.length > 0 &&
		!value.includes("\0") &&
		!path.posix.isAbsolute(value) &&
		!value
			.split("/")
			.some((part) => part === ".." || part === "." || part === "")
	);
}

function validateManifest(manifest) {
	const fail = (message) => {
		throw new Error(`Invalid impact manifest: ${message}`);
	};
	if (manifest?.version !== 1) fail("version must be 1");
	const domains = ["access", "auth", "integrations", "licensing", "platform"];
	if (
		!manifest.domains ||
		Object.keys(manifest.domains).sort().join() !== domains.join()
	)
		fail("domains must be access, auth, integrations, licensing and platform");
	for (const domain of domains) {
		const specs = manifest.domains[domain];
		if (
			!Array.isArray(specs) ||
			!specs.length ||
			specs.some(
				(spec) =>
					!repositoryPath(spec) ||
					!spec.startsWith(`anchor-ui/e2e/features/${domain}/`) ||
					!spec.endsWith(".e2e.ts"),
			)
		)
			fail(`invalid test paths for ${domain}`);
		if (new Set(specs).size !== specs.length)
			fail(`duplicate test paths for ${domain}`);
	}
	if (
		!Array.isArray(manifest.always) ||
		[
			...manifest.domains.auth,
			"anchor-ui/e2e/features/platform/products.e2e.ts",
		].some((spec) => !manifest.always.includes(spec))
	)
		fail(
			"always must include every auth spec and the critical platform/products spec",
		);
	if (
		manifest.always.some(
			(spec) => !Object.values(manifest.domains).flat().includes(spec),
		)
	)
		fail("always refers to an unknown spec");
	const validPattern = (pattern) => {
		if (typeof pattern !== "string") return false;
		if (/^\*\*\/\*\.(md|txt|rst|adoc)$/.test(pattern)) return true;
		const plain = pattern.endsWith("/**") ? pattern.slice(0, -3) : pattern;
		return repositoryPath(plain) && !/[?*\[\]{}]/.test(plain);
	};
	if (
		!Array.isArray(manifest.docs) ||
		manifest.docs.some((pattern) => !validPattern(pattern))
	)
		fail("invalid documentation patterns");
	const ids = new Set();
	for (const kind of ["full", "rules"]) {
		if (!Array.isArray(manifest[kind])) fail(`${kind} must be an array`);
		for (const rule of manifest[kind]) {
			if (typeof rule?.id !== "string" || !rule.id || ids.has(rule.id))
				fail("rule IDs must be nonempty and unique");
			ids.add(rule.id);
			if (
				!Array.isArray(rule.paths) ||
				!rule.paths.length ||
				rule.paths.some((pattern) => !validPattern(pattern))
			)
				fail(`${rule.id} has invalid paths; use exact files or directory/**`);
			if (typeof rule.reason !== "string" || !rule.reason.trim())
				fail(`${rule.id} needs a review reason`);
			if (kind === "rules") {
				if (
					!Array.isArray(rule.domains) ||
					!rule.domains.length ||
					rule.domains.some((domain) => !domains.includes(domain))
				)
					fail(`${rule.id} has unknown domains: ${rule.domains}`);
				if (
					!Array.isArray(rule.provenance) ||
					!rule.provenance.length ||
					rule.provenance.some((file) => !repositoryPath(file))
				)
					fail(`${rule.id} needs code provenance paths`);
			}
		}
	}
}

export function planAffectedTests({
	changes = [],
	manifest,
	gitError,
	base = null,
	mergeBase = null,
	head = null,
	testFiles,
}) {
	validateManifest(manifest);
	const reviewedTests = Object.values(manifest.domains).flat();
	const allTests = [...new Set(testFiles ?? reviewedTests)].sort();
	if (
		!allTests.length ||
		allTests.some(
			(spec) =>
				!repositoryPath(spec) ||
				!spec.startsWith("anchor-ui/e2e/features/") ||
				!spec.endsWith(".e2e.ts"),
		)
	)
		throw new Error(
			"Cannot plan an empty or invalid current E2E test inventory",
		);
	const chosen = new Set();
	const reasons = [];
	let full = Boolean(gitError);
	let requiresRun = false;
	if (gitError)
		reasons.push({
			path: null,
			status: null,
			rule: "git-error",
			domains: [],
			message: gitError,
		});
	for (const spec of allTests.filter((spec) => !reviewedTests.includes(spec))) {
		full = true;
		reasons.push({
			path: spec,
			status: null,
			rule: "manifest-drift",
			domains: [],
			message:
				"Current test inventory contains an unreviewed spec; run every current test and review the manifest.",
		});
	}
	for (const spec of reviewedTests.filter((spec) => !allTests.includes(spec))) {
		full = true;
		reasons.push({
			path: spec,
			status: null,
			rule: "manifest-drift",
			domains: [],
			message:
				"A reviewed spec is absent from the current inventory; run every remaining test and review the manifest.",
		});
	}
	for (const change of changes) {
		if (!Array.isArray(change?.paths) || !change.paths.length) {
			full = true;
			reasons.push({
				path: null,
				status: change?.status ?? null,
				rule: "invalid-change",
				domains: [],
				message: "An incomplete changed-file record requires every feature.",
			});
			continue;
		}
		for (const file of change.paths) {
			if (!repositoryPath(file)) {
				full = true;
				reasons.push({
					path: file,
					status: change.status,
					rule: "invalid-path",
					domains: [],
					message: "Invalid repository path requires every feature.",
				});
				continue;
			}
			if (
				file.startsWith("anchor-ui/e2e/features/") &&
				file.endsWith(".e2e.ts")
			) {
				requiresRun = true;
				const domain = file.split("/")[3];
				if (manifest.domains[domain]) {
					for (const spec of manifest.domains[domain])
						if (allTests.includes(spec)) chosen.add(spec);
					if (allTests.includes(file)) chosen.add(file);
					reasons.push({
						path: file,
						status: change.status,
						source: change.source,
						rule: "changed-spec",
						domains: [domain],
						message:
							"Changed feature specs run with their owning domain and mandatory journeys.",
					});
				} else {
					full = true;
					reasons.push({
						path: file,
						status: change.status,
						source: change.source,
						rule: "unmapped-spec",
						domains: [],
						message:
							"An unreviewed feature domain requires every current test.",
					});
				}
				continue;
			}
			const shared = manifest.full.filter((rule) =>
				rule.paths.some((pattern) => matches(file, pattern)),
			);
			const rules = manifest.rules.filter((rule) =>
				rule.paths.some((pattern) => matches(file, pattern)),
			);
			if (shared.length) {
				full = true;
				for (const rule of shared)
					reasons.push({
						path: file,
						status: change.status,
						source: change.source,
						rule: rule.id,
						domains: [],
						message: rule.reason,
					});
			} else if (rules.length) {
				requiresRun = true;
				for (const rule of rules) {
					for (const domain of rule.domains)
						for (const spec of manifest.domains[domain])
							if (allTests.includes(spec)) chosen.add(spec);
					reasons.push({
						path: file,
						status: change.status,
						source: change.source,
						rule: rule.id,
						domains: [...rule.domains].sort(),
						provenance: [...rule.provenance].sort(),
						message: rule.reason,
					});
				}
			} else if (
				manifest.docs.some((pattern) => matches(file, pattern)) &&
				/\.(md|txt|rst|adoc)$/.test(file)
			) {
				reasons.push({
					path: file,
					status: change.status,
					source: change.source,
					rule: "documentation",
					domains: [],
					message:
						"Documentation has no focused browser run; the full completion gate remains required.",
				});
			} else {
				full = true;
				reasons.push({
					path: file,
					status: change.status,
					source: change.source,
					rule: "unmapped",
					domains: [],
					message:
						"Unreviewed executable or shared input requires every feature.",
				});
			}
		}
	}
	if (full || requiresRun)
		for (const spec of manifest.always)
			if (allTests.includes(spec)) chosen.add(spec);
	if (requiresRun && !full)
		reasons.push({
			path: null,
			status: null,
			rule: "critical-journeys",
			domains: ["auth", "platform"],
			message:
				"Authentication and the critical product lifecycle run with every selected executable change.",
		});
	const testPaths = full ? allTests : [...chosen].sort();
	return {
		mode: full ? "full" : testPaths.length ? "selected" : "none",
		domains: Object.keys(manifest.domains)
			.filter((domain) =>
				manifest.domains[domain].some((spec) => testPaths.includes(spec)),
			)
			.sort(),
		testPaths,
		changedPaths: [
			...new Set(changes.flatMap((change) => change?.paths ?? [])),
		].sort(),
		allTestPaths: allTests,
		reasons: reasons.sort((left, right) =>
			JSON.stringify(left).localeCompare(JSON.stringify(right), "en"),
		),
		base,
		mergeBase,
		head,
	};
}

function discoverTests(repositoryRoot) {
	const directory = path.join(repositoryRoot, "anchor-ui/e2e/features");
	function visit(current) {
		return readdirSync(current, { withFileTypes: true }).flatMap((entry) => {
			const file = path.join(current, entry.name);
			return entry.isDirectory()
				? visit(file)
				: entry.isFile() && entry.name.endsWith(".e2e.ts")
					? [path.relative(repositoryRoot, file).split(path.sep).join("/")]
					: [];
		});
	}
	return visit(directory).sort();
}

export function planFromGit({
	cwd = process.cwd(),
	base,
	head,
	includeWorkingTree = true,
	manifestPath = path.resolve(
		import.meta.dirname,
		"../e2e/impact-manifest.json",
	),
} = {}) {
	const collected = collectGitChanges({ cwd, base, head, includeWorkingTree });
	const repositoryRoot =
		collected.repositoryRoot ?? path.resolve(import.meta.dirname, "../..");
	let manifest;
	try {
		manifest = JSON.parse(
			readFileSync(path.resolve(repositoryRoot, manifestPath), "utf8"),
		);
	} catch (error) {
		throw new Error(
			`Cannot load impact manifest ${manifestPath}: ${error.message}`,
		);
	}
	return planAffectedTests({
		...collected,
		manifest,
		testFiles: discoverTests(repositoryRoot),
	});
}

function main(args) {
	const options = {};
	const names = {
		"--base": "base",
		"--head": "head",
		"--cwd": "cwd",
		"--manifest": "manifestPath",
	};
	for (let i = 0; i < args.length; i++) {
		if (args[i] === "--help") {
			console.log(
				"Usage: node scripts/e2e-impact.mjs [--base <ref>] [--head <ref>] [--cwd <directory>] [--manifest <file>] [--committed-only]\nEmits a JSON plan; base defaults to E2E_BASE_REF or origin/main. Local staged, unstaged and untracked changes are included by default.",
			);
			return;
		}
		if (args[i] === "--committed-only") options.includeWorkingTree = false;
		else if (names[args[i]] && args[i + 1] && !args[i + 1].startsWith("--"))
			options[names[args[i]]] = args[++i];
		else throw new Error(`Unknown option or missing value: ${args[i]}`);
	}
	console.log(JSON.stringify(planFromGit(options), null, 2));
}

if (
	process.argv[1] &&
	pathToFileURL(path.resolve(process.argv[1])).href === import.meta.url
) {
	try {
		main(process.argv.slice(2));
	} catch (error) {
		console.error(error.message);
		process.exitCode = 1;
	}
}
