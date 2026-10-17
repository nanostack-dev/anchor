import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import path from "node:path";
import test from "node:test";
import { doctorReport } from "./e2e-doctor.mjs";

function fixture() {
	return {
		worktree: "/work/anchor",
		branch: { output: "test/fixture" },
		commit: { output: "abcdef" },
		pnpm: { available: false },
		docker: { available: false },
		runtime: {
			active: true,
			runId: "owned",
			apiURL: "http://127.0.0.1:4000",
			password: "must-not-leak",
			credentials: { token: "must-not-leak" },
		},
		ready: {
			runId: "owned",
			apiURL: "http://127.0.0.1:4000",
			fingerprint: "current",
		},
		fingerprint: "current",
		uiListening: true,
		inventory: { scenarios: 57 },
	};
}

test("a foreign preview cannot appear ready merely because the UI responds", () => {
	const input = fixture();
	input.ready.runId = "another-worktree";
	const report = doctorReport(input);
	assert.equal(report.preview.state, "foreign or inactive runtime");
	assert.equal(report.ui.listening, true);
	assert.match(report.ui.owner, /not verified/);
});

test("stale frontend and inactive backend are distinct from owned readiness", () => {
	const input = fixture();
	assert.equal(doctorReport(input).preview.state, "ready for this worktree");
	input.ready.fingerprint = "previous-build";
	assert.equal(doctorReport(input).preview.state, "stale frontend");
	input.runtime.active = false;
	assert.equal(
		doctorReport(input).preview.state,
		"foreign or inactive runtime",
	);
});

test("status remains useful with no runtime or tools and strips metadata secrets", () => {
	const input = fixture();
	assert.doesNotMatch(JSON.stringify(doctorReport(input)), /must-not-leak/);
	input.runtime = null;
	input.ready = null;
	const report = doctorReport(input);
	assert.equal(report.runtime.active, false);
	assert.equal(report.preview.state, "unreachable");
	assert.equal(report.tools.pnpm, "unavailable");
});

test("help works without pnpm or Docker on PATH", () => {
	const output = execFileSync(
		process.execPath,
		[path.join(import.meta.dirname, "e2e-doctor.mjs"), "--help"],
		{ env: { PATH: "/unavailable" }, encoding: "utf8" },
	);
	assert.match(output, /Starts and stops no services/);
});
