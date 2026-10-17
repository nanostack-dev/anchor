import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import {
	mkdirSync,
	mkdtempSync,
	readFileSync,
	renameSync,
	rmSync,
	writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import {
	collectGitChanges,
	planAffectedTests,
	planFromGit,
} from "./e2e-impact.mjs";

const manifest = {
	version: 1,
	domains: {
		auth: ["anchor-ui/e2e/features/auth/session.e2e.ts"],
		platform: ["anchor-ui/e2e/features/platform/products.e2e.ts"],
		access: ["anchor-ui/e2e/features/access/roles.e2e.ts"],
		licensing: ["anchor-ui/e2e/features/licensing/schema.e2e.ts"],
		integrations: ["anchor-ui/e2e/features/integrations/email.e2e.ts"],
	},
	always: [
		"anchor-ui/e2e/features/auth/session.e2e.ts",
		"anchor-ui/e2e/features/platform/products.e2e.ts",
	],
	docs: ["README.md", "docs/**"],
	rules: [
		{
			id: "license",
			paths: ["anchor-ui/src/components/license/**"],
			domains: ["licensing", "access", "integrations"],
			reason: "License changes affect licensed organizations and their events.",
			provenance: ["apps/anchor/internal/service/organization_service.go"],
		},
		{
			id: "role-ui",
			paths: ["anchor-ui/src/components/product/roles/**"],
			domains: ["access"],
			reason: "Roles are covered by the access journey.",
			provenance: [
				"anchor-ui/src/components/product/roles/ProductRoleDialog.tsx",
			],
		},
		{
			id: "email-ui",
			paths: ["anchor-ui/src/components/email/**"],
			domains: ["integrations"],
			reason: "Email editing is covered by the SMTP lifecycle.",
			provenance: ["anchor-ui/e2e/features/integrations/email.e2e.ts"],
		},
	],
	full: [
		{
			id: "shared-ui",
			paths: ["anchor-ui/src/components/common/**"],
			reason: "Shared table behavior affects every feature.",
		},
	],
};

test("an unmapped executable change runs every feature instead of an empty selection", () => {
	const plan = planAffectedTests({
		changes: [
			{
				status: "A",
				paths: ["anchor-ui/src/new-feature.tsx"],
				source: "untracked",
			},
		],
		manifest,
	});
	assert.equal(plan.mode, "full");
	assert.deepEqual(plan.testPaths, [
		"anchor-ui/e2e/features/access/roles.e2e.ts",
		"anchor-ui/e2e/features/auth/session.e2e.ts",
		"anchor-ui/e2e/features/integrations/email.e2e.ts",
		"anchor-ui/e2e/features/licensing/schema.e2e.ts",
		"anchor-ui/e2e/features/platform/products.e2e.ts",
	]);
});

test("a reviewed licensing change includes its tenancy and event consumers plus mandatory journeys", () => {
	const plan = planAffectedTests({
		changes: [
			{
				status: "M",
				paths: ["anchor-ui/src/components/license/LicenseSchemaEditor.tsx"],
				source: "committed",
			},
		],
		manifest,
	});
	assert.equal(plan.mode, "selected");
	assert.deepEqual(plan.domains, [
		"access",
		"auth",
		"integrations",
		"licensing",
		"platform",
	]);
	assert.equal(plan.reasons[0].rule, "license");
	assert.deepEqual(plan.reasons[0].domains, [
		"access",
		"integrations",
		"licensing",
	]);
});

test("malformed manifest domain names fail clearly instead of dropping tests", () => {
	const invalid = structuredClone(manifest);
	invalid.rules[0].domains.push("licenseing");
	assert.throws(
		() => planAffectedTests({ changes: [], manifest: invalid }),
		/Invalid impact manifest.*licenseing/,
	);
});

test("unreviewed current test inventory prevents a docs-only plan from omitting a new feature", () => {
	const plan = planAffectedTests({
		changes: [{ status: "M", paths: ["README.md"], source: "unstaged" }],
		manifest,
		testFiles: [
			...Object.values(manifest.domains).flat(),
			"anchor-ui/e2e/features/access/new-lifecycle.e2e.ts",
		],
	});
	assert.equal(plan.mode, "full");
	assert.ok(
		plan.testPaths.includes(
			"anchor-ui/e2e/features/access/new-lifecycle.e2e.ts",
		),
	);
	assert.equal(plan.allTestPaths.length, 6);
});

function repository(t) {
	const cwd = mkdtempSync(path.join(tmpdir(), "anchor-impact-"));
	t.after(() => rmSync(cwd, { recursive: true, force: true }));
	const git = (...args) => execFileSync("git", args, { cwd, encoding: "utf8" });
	const write = (file, content = "initial\n") => {
		mkdirSync(path.dirname(path.join(cwd, file)), { recursive: true });
		writeFileSync(path.join(cwd, file), content);
	};
	git("init", "-q", "-b", "main");
	git("config", "user.name", "E2E impact fixture");
	git("config", "user.email", "impact@example.invalid");
	for (const spec of Object.values(manifest.domains).flat()) write(spec);
	write("README.md");
	write("anchor-ui/e2e/impact-manifest.json", JSON.stringify(manifest));
	git("add", ".");
	git("commit", "-qm", "base");
	git("branch", "fixture-base");
	return { cwd, git, write };
}

test("Git collection uses the real merge-base and unions staged, unstaged and untracked NUL-safe paths", (t) => {
	const { cwd, git, write } = repository(t);
	git("checkout", "-qb", "feature");
	write("anchor-ui/src/components/license/LicenseSchemaEditor.tsx");
	git("add", ".");
	git("commit", "-qm", "license feature");
	git("checkout", "main");
	write("anchor-ui/src/unrelated-main.tsx");
	git("add", ".");
	git("commit", "-qm", "base advanced");
	git("checkout", "feature");
	write("anchor-ui/src/components/license/staged.tsx");
	git("add", ".");
	write("README.md", "working edit\n");
	write("anchor-ui/src/components/license/space and\nnewline.tsx");
	const collected = collectGitChanges({ cwd, base: "main" });
	assert.equal(collected.gitError, null);
	assert.equal(collected.mergeBase, git("rev-parse", "fixture-base").trim());
	assert.equal(collected.head, git("rev-parse", "HEAD").trim());
	assert.deepEqual(
		collected.changes.map((change) => [change.source, change.paths]),
		[
			[
				"committed",
				["anchor-ui/src/components/license/LicenseSchemaEditor.tsx"],
			],
			["staged", ["anchor-ui/src/components/license/staged.tsx"]],
			["unstaged", ["README.md"]],
			[
				"untracked",
				["anchor-ui/src/components/license/space and\nnewline.tsx"],
			],
		],
	);
});

test("running from anchor-ui still collects staged Go changes and repo-relative untracked files", (t) => {
	const { cwd, git, write } = repository(t);
	write("apps/anchor/internal/license/service/schema_service.go");
	git("add", ".");
	write("anchor-ui/src/new.tsx");
	const collected = collectGitChanges({
		cwd: path.join(cwd, "anchor-ui"),
		base: "fixture-base",
	});
	assert.deepEqual(
		collected.changes.map((change) => change.paths),
		[
			["apps/anchor/internal/license/service/schema_service.go"],
			["anchor-ui/src/new.tsx"],
		],
	);
});

test("CLI plans a real renamed feature conservatively using both paths and no browser runtime", (t) => {
	const { cwd, git, write } = repository(t);
	write(
		"anchor-ui/src/components/license/old.tsx",
		"export const stable = true;\n",
	);
	git("add", ".");
	git("commit", "-qm", "old feature");
	git("branch", "rename-base");
	renameSync(
		path.join(cwd, "anchor-ui/src/components/license/old.tsx"),
		path.join(cwd, "anchor-ui/src/new-executable.tsx"),
	);
	git("add", ".");
	const plan = planFromGit({
		cwd,
		base: "rename-base",
		manifestPath: "anchor-ui/e2e/impact-manifest.json",
	});
	assert.equal(plan.mode, "full");
	assert.deepEqual(plan.changedPaths, [
		"anchor-ui/src/components/license/old.tsx",
		"anchor-ui/src/new-executable.tsx",
	]);
	const cli = JSON.parse(
		execFileSync(
			process.execPath,
			[
				path.resolve(import.meta.dirname, "e2e-impact.mjs"),
				"--cwd",
				cwd,
				"--base",
				"rename-base",
				"--manifest",
				"anchor-ui/e2e/impact-manifest.json",
			],
			{ encoding: "utf8" },
		),
	);
	assert.deepEqual(cli, plan);
	assert.equal(cli.allTestPaths.length, 5);
});

test("a mixed-domain branch selects the union and mandatory journeys while docs add no unrelated suite", (t) => {
	const { cwd, git, write } = repository(t);
	write("anchor-ui/src/components/product/roles/RoleEditor.tsx");
	write("anchor-ui/src/components/email/EmailTemplateBuilder.tsx");
	write("docs/feature.md");
	git("add", ".");
	git("commit", "-qm", "roles and email");
	const plan = planFromGit({
		cwd,
		base: "fixture-base",
		manifestPath: "anchor-ui/e2e/impact-manifest.json",
	});
	assert.equal(plan.mode, "selected");
	assert.deepEqual(plan.testPaths, [
		"anchor-ui/e2e/features/access/roles.e2e.ts",
		"anchor-ui/e2e/features/auth/session.e2e.ts",
		"anchor-ui/e2e/features/integrations/email.e2e.ts",
		"anchor-ui/e2e/features/platform/products.e2e.ts",
	]);
	assert.equal(plan.allTestPaths.length, 5);
});

test("docs-only and unchanged branches have no focused browser run", (t) => {
	const { cwd, write } = repository(t);
	const options = {
		cwd,
		base: "fixture-base",
		manifestPath: "anchor-ui/e2e/impact-manifest.json",
	};
	assert.equal(planFromGit(options).mode, "none");
	write("README.md", "documentation changed\n");
	write("docs/new guide.md");
	const plan = planFromGit(options);
	assert.equal(plan.mode, "none");
	assert.deepEqual(plan.testPaths, []);
	assert.equal(plan.allTestPaths.length, 5);
});

test("an executable file inside a docs directory still triggers the full suite", (t) => {
	const { cwd, write } = repository(t);
	write("docs/run-example.js");
	assert.equal(
		planFromGit({
			cwd,
			base: "fixture-base",
			manifestPath: "anchor-ui/e2e/impact-manifest.json",
		}).mode,
		"full",
	);
});

test("a shared table change runs every current feature", (t) => {
	const { cwd, write } = repository(t);
	write("anchor-ui/src/components/common/datatable/AnchorDataTable.tsx");
	const plan = planFromGit({
		cwd,
		base: "fixture-base",
		manifestPath: "anchor-ui/e2e/impact-manifest.json",
	});
	assert.equal(plan.mode, "full");
	assert.equal(plan.testPaths.length, 5);
	assert.ok(plan.reasons.some((reason) => reason.rule === "shared-ui"));
});

test("missing base, invalid head and Git command errors all produce a full fallback", (t) => {
	const { cwd } = repository(t);
	for (const refs of [
		{ base: "absent-ref" },
		{ base: "fixture-base", head: "absent-head" },
	]) {
		const plan = planFromGit({
			cwd,
			...refs,
			manifestPath: "anchor-ui/e2e/impact-manifest.json",
		});
		assert.equal(plan.mode, "full");
		assert.equal(plan.testPaths.length, 5);
		assert.ok(plan.reasons.some((reason) => reason.rule === "git-error"));
	}
	const collection = collectGitChanges({ cwd: tmpdir(), base: "main" });
	assert.ok(collection.gitError);
	assert.equal(planAffectedTests({ ...collection, manifest }).mode, "full");
});

test("a staged spec deletion runs remaining specs without passing the deleted path to Playwright", (t) => {
	const { cwd, git } = repository(t);
	git("rm", "-q", "anchor-ui/e2e/features/access/roles.e2e.ts");
	const collected = collectGitChanges({ cwd, base: "fixture-base" });
	assert.ok(
		collected.changes.some(
			(change) =>
				change.status === "D" &&
				change.paths[0] === "anchor-ui/e2e/features/access/roles.e2e.ts",
		),
	);
	const plan = planFromGit({
		cwd,
		base: "fixture-base",
		manifestPath: "anchor-ui/e2e/impact-manifest.json",
	});
	assert.equal(plan.mode, "full");
	assert.equal(plan.testPaths.length, 4);
	assert.ok(
		!plan.testPaths.includes("anchor-ui/e2e/features/access/roles.e2e.ts"),
	);
});

test("a new untracked feature spec is included and inventory drift requires every feature", (t) => {
	const { cwd, write } = repository(t);
	write("anchor-ui/e2e/features/access/new-lifecycle.e2e.ts");
	const plan = planFromGit({
		cwd,
		base: "fixture-base",
		manifestPath: "anchor-ui/e2e/impact-manifest.json",
	});
	assert.equal(plan.mode, "full");
	assert.equal(plan.allTestPaths.length, 6);
	assert.ok(
		plan.testPaths.includes(
			"anchor-ui/e2e/features/access/new-lifecycle.e2e.ts",
		),
	);
});

test("a changed existing spec runs its domain and the critical journeys", (t) => {
	const { cwd, write } = repository(t);
	write(
		"anchor-ui/e2e/features/access/roles.e2e.ts",
		"changed role lifecycle\n",
	);
	const plan = planFromGit({
		cwd,
		base: "fixture-base",
		manifestPath: "anchor-ui/e2e/impact-manifest.json",
	});
	assert.equal(plan.mode, "selected");
	assert.deepEqual(plan.testPaths, [
		"anchor-ui/e2e/features/access/roles.e2e.ts",
		"anchor-ui/e2e/features/auth/session.e2e.ts",
		"anchor-ui/e2e/features/platform/products.e2e.ts",
	]);
	assert.ok(plan.reasons.some((reason) => reason.rule === "changed-spec"));
});

test("a staged production deletion selects its reviewed consumers", (t) => {
	const { cwd, git, write } = repository(t);
	write("anchor-ui/src/components/email/OldEditor.tsx");
	git("add", ".");
	git("commit", "-qm", "email editor");
	git("branch", "delete-base");
	git("rm", "-q", "anchor-ui/src/components/email/OldEditor.tsx");
	const plan = planFromGit({
		cwd,
		base: "delete-base",
		manifestPath: "anchor-ui/e2e/impact-manifest.json",
	});
	assert.equal(plan.mode, "selected");
	assert.deepEqual(plan.domains, ["auth", "integrations", "platform"]);
});

test("all matching reviewed rules are unioned and change order cannot change the plan", () => {
	const overlap = structuredClone(manifest);
	overlap.rules.push({
		id: "role-consumer",
		paths: ["anchor-ui/src/components/product/roles/RoleEditor.tsx"],
		domains: ["integrations"],
		reason: "An independently reviewed event consumer.",
		provenance: ["anchor-ui/e2e/features/integrations/integrations.e2e.ts"],
	});
	const changes = [
		{
			status: "M",
			paths: ["anchor-ui/src/components/product/roles/RoleEditor.tsx"],
			source: "committed",
		},
		{ status: "M", paths: ["README.md"], source: "unstaged" },
	];
	const plan = planAffectedTests({ changes, manifest: overlap });
	assert.deepEqual(plan.domains, [
		"access",
		"auth",
		"integrations",
		"platform",
	]);
	assert.deepEqual(
		planAffectedTests({ changes: changes.toReversed(), manifest: overlap }),
		plan,
	);
});

test("empty inventory, unknown patterns, broken JSON and missing mandatory journeys fail clearly", (t) => {
	assert.throws(
		() => planAffectedTests({ changes: [], manifest, testFiles: [] }),
		/empty or invalid.*inventory/,
	);
	const badPattern = structuredClone(manifest);
	badPattern.rules[0].paths = ["anchor-ui/src/**/license/*.tsx"];
	assert.throws(
		() => planAffectedTests({ changes: [], manifest: badPattern }),
		/Invalid impact manifest.*invalid paths/,
	);
	const noCritical = structuredClone(manifest);
	noCritical.always = [];
	assert.throws(
		() => planAffectedTests({ changes: [], manifest: noCritical }),
		/Invalid impact manifest.*always/,
	);
	const { cwd, write } = repository(t);
	write("anchor-ui/e2e/impact-manifest.json", "{broken");
	assert.throws(
		() =>
			planFromGit({
				cwd,
				base: "fixture-base",
				manifestPath: "anchor-ui/e2e/impact-manifest.json",
			}),
		/Cannot load impact manifest/,
	);
});

test("the reviewed Anchor manifest narrows email but broadens licensing consumers and shared providers", () => {
	const actual = JSON.parse(
		readFileSync(
			path.resolve(import.meta.dirname, "../e2e/impact-manifest.json"),
			"utf8",
		),
	);
	const plan = (file) =>
		planAffectedTests({
			manifest: actual,
			changes: [{ status: "M", paths: [file], source: "committed" }],
		});
	const email = plan("apps/anchor/internal/email/service/service.go");
	assert.equal(email.mode, "selected");
	assert.deepEqual(email.testPaths, [
		"anchor-ui/e2e/features/auth/bootstrap.e2e.ts",
		"anchor-ui/e2e/features/auth/session.e2e.ts",
		"anchor-ui/e2e/features/integrations/email.e2e.ts",
		"anchor-ui/e2e/features/integrations/integrations.e2e.ts",
		"anchor-ui/e2e/features/platform/products.e2e.ts",
	]);
	assert.deepEqual(
		plan("apps/anchor/internal/license/service/schema_service.go").domains,
		["access", "auth", "integrations", "licensing", "platform"],
	);
	assert.equal(
		plan("apps/anchor/internal/integration/provider/registry.go").mode,
		"full",
	);
	assert.equal(
		plan("apps/anchor/internal/unknown-new-handler.go").mode,
		"full",
	);
	assert.equal(
		plan("anchor-ui/src/context/product/ProductContext.tsx").mode,
		"full",
	);
});
