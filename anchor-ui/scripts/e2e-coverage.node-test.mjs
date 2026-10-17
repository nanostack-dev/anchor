import assert from "node:assert/strict";
import { test } from "node:test";
import {
	checkCoverageDocument,
	renderCoverage,
	summarizeInventory,
	updateCoverageDocument,
} from "./e2e-coverage.mjs";

function spec(title, file) {
	return { title, file, tests: [{ projectName: "chromium" }] };
}

function inventory() {
	return {
		errors: [],
		suites: [
			{
				title: "email.e2e.ts",
				file: "integrations/email.e2e.ts",
				specs: [spec("opens")],
				suites: [
					{
						title: "saved examples",
						file: "integrations/email.e2e.ts",
						specs: [spec("persists")],
					},
					{
						title: "draft examples",
						file: "integrations/email.e2e.ts",
						specs: [spec("persists")],
					},
				],
			},
			{
				title: "bootstrap.e2e.ts",
				file: "auth/bootstrap.e2e.ts",
				specs: [spec("initializes")],
			},
		],
	};
}

test("nested equal scenario titles remain distinct and files sort deterministically", () => {
	const report = inventory();
	const summary = summarizeInventory(report);
	assert.deepEqual(summary, {
		scenarios: 4,
		specs: [
			{ file: "auth/bootstrap.e2e.ts", scenarios: 1 },
			{ file: "integrations/email.e2e.ts", scenarios: 3 },
		],
	});
	report.suites.reverse();
	assert.deepEqual(summarizeInventory(report), summary);
});

test("an added nested scenario fails the count check until regenerated", () => {
	const report = inventory();
	const original = `Manual behavior matrix.\n\n${renderCoverage(summarizeInventory(report))}\n\nHistorical evidence: 3 scenarios passed in 2025.\n`;
	checkCoverageDocument(original, summarizeInventory(report));
	report.suites[0].suites[1].specs.push(spec("preserves saved and new values"));
	const next = summarizeInventory(report);
	assert.equal(next.scenarios, 5);
	assert.equal(next.specs[1].scenarios, 4);
	assert.throws(() => checkCoverageDocument(original, next), /drifted/);
	const updated = updateCoverageDocument(original, next);
	checkCoverageDocument(updated, next);
	assert.match(updated, /\*\*5 scenarios in 2 spec files\*\*/);
	assert.match(
		updated,
		/\| \[integrations\/email\.e2e\.ts\]\(\.\.\/features\/integrations\/email\.e2e\.ts\) \| 4 \|/,
	);
	assert.ok(updated.startsWith("Manual behavior matrix.\n\n"));
	assert.ok(
		updated.endsWith("Historical evidence: 3 scenarios passed in 2025.\n"),
	);
	assert.equal(updateCoverageDocument(updated, next), updated);
});

test("a new spec updates the file count as well as the scenario total", () => {
	const report = inventory();
	report.suites.push({
		title: "roles.e2e.ts",
		file: "access/roles.e2e.ts",
		specs: [spec("persists")],
	});
	const summary = summarizeInventory(report);
	assert.equal(summary.scenarios, 5);
	assert.equal(summary.specs.length, 3);
	assert.deepEqual(summary.specs[0], {
		file: "access/roles.e2e.ts",
		scenarios: 1,
	});
});

test("duplicate discoveries fail instead of inflating the documented count", () => {
	const report = inventory();
	report.suites[0].specs.push(spec("opens"));
	assert.throws(() => summarizeInventory(report), /Duplicate scenario/);
});

test("empty and failed discovery cannot replace valid coverage counts", () => {
	assert.throws(() => summarizeInventory({ suites: [] }), /empty/);
	assert.throws(
		() =>
			summarizeInventory({
				...inventory(),
				errors: [{ message: "invalid test" }],
			}),
		/discovery reported errors/,
	);
});

test("explicit spec files override inherited suite paths and stay within the test root", () => {
	const report = inventory();
	report.config = { rootDir: "/workspace/e2e/features" };
	report.suites[0].specs[0].file =
		"/workspace/e2e/features/access/roles.e2e.ts";
	assert.deepEqual(summarizeInventory(report).specs[0], {
		file: "access/roles.e2e.ts",
		scenarios: 1,
	});
	report.suites[0].specs[0].file = "/workspace/unrelated.e2e.ts";
	assert.throws(() => summarizeInventory(report), /Unexpected app spec file/);
});

test("missing, duplicated or reversed generated markers fail without rewriting prose", () => {
	const summary = summarizeInventory(inventory());
	const generated = renderCoverage(summary);
	for (const document of [
		"Manual text only.",
		`${generated}\n${generated}`,
		"<!-- e2e-coverage:end -->\n<!-- e2e-coverage:start -->",
	])
		assert.throws(
			() => updateCoverageDocument(document, summary),
			/exactly one ordered generated block/,
		);
});
