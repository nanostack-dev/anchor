import assert from "node:assert/strict";
import { test } from "node:test";
import { verifyCompletion } from "./e2e-completion.mjs";

function report(groups) {
	return {
		errors: [],
		stats: { unexpected: 0, flaky: 0, skipped: 0 },
		suites: groups.map(([title, names]) => ({
			title,
			file: "sample.e2e.ts",
			specs: names.map((name) => ({
				title: name,
				file: "sample.e2e.ts",
				tests: [
					{
						expectedStatus: "passed",
						results: [{ status: "passed", retry: 0 }],
					},
				],
			})),
		})),
	};
}

test("a complete run preserves equal titles in different feature groups", () => {
	const inventory = report([
		["roles", ["persists"]],
		["permissions", ["persists"]],
	]);
	assert.equal(verifyCompletion(inventory, inventory).scenarios, 2);
});

test("a passing subset cannot satisfy the full completion gate", () => {
	const inventory = report([["roles", ["creates", "deletes"]]]);
	assert.throws(
		() => verifyCompletion(inventory, report([["roles", ["creates"]]])),
		/missing/i,
	);
});

test("duplicating a passing scenario cannot replace a missing scenario", () => {
	const inventory = report([["roles", ["creates", "deletes"]]]);
	assert.throws(
		() =>
			verifyCompletion(inventory, report([["roles", ["creates", "creates"]]])),
		/duplicate/i,
	);
});

test("an expected failure is not verified app behavior", () => {
	const inventory = report([["roles", ["creates"]]]);
	const actual = structuredClone(inventory);
	actual.suites[0].specs[0].tests[0].expectedStatus = "failed";
	actual.suites[0].specs[0].tests[0].results[0].status = "failed";
	assert.throws(() => verifyCompletion(inventory, actual), /passed/i);
});

test("a retried pass cannot satisfy the zero-retry completion gate", () => {
	const inventory = report([["roles", ["creates"]]]);
	const actual = structuredClone(inventory);
	actual.suites[0].specs[0].tests[0].results[0].retry = 1;
	assert.throws(() => verifyCompletion(inventory, actual), /retry/i);
});

test("skipped or globally failed runs cannot satisfy the gate", () => {
	const inventory = report([["roles", ["creates"]]]);
	for (const key of ["skipped", "flaky", "unexpected"]) {
		const actual = structuredClone(inventory);
		actual.stats[key] = 1;
		assert.throws(() => verifyCompletion(inventory, actual));
	}
	const actual = structuredClone(inventory);
	actual.errors.push({ message: "web server crashed" });
	assert.throws(() => verifyCompletion(inventory, actual));
});
