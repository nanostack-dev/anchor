import assert from "node:assert/strict";
import test from "node:test";
import { orphanedProjects } from "./e2e-runtime.mjs";

const own = "anchor-e2e-aaaaaaaaaa-";
const select = (projects, overrides = {}) =>
	orphanedProjects({
		projects: new Map(projects),
		ownPrefix: own,
		trackedRunIds: new Set(["11111111"]),
		liveWorktreeHashes: new Set(["aaaaaaaaaa", "bbbbbbbbbb"]),
		worktreeExists: (path) => path === "/live",
		...overrides,
	});

test("this worktree keeps its tracked run and removes untracked leftovers", () => {
	assert.deepEqual(
		select([
			[`${own}11111111`, "/live"],
			[`${own}22222222`, "/live"],
		]),
		[`${own}22222222`],
	);
});

test("a stop with no tracked run removes every run of this worktree", () => {
	assert.deepEqual(
		select([[`${own}11111111`, "/live"]], { trackedRunIds: new Set() }),
		[`${own}11111111`],
	);
});

test("another worktree's runtime is removed only once its directory is gone", () => {
	assert.deepEqual(
		select([
			["anchor-e2e-cccccccccc-33333333", "/live"],
			["anchor-e2e-dddddddddd-44444444", "/deleted"],
		]),
		["anchor-e2e-dddddddddd-44444444"],
	);
});

test("an unlabeled runtime is removed only when no listed worktree owns it", () => {
	assert.deepEqual(
		select([
			["anchor-e2e-bbbbbbbbbb-55555555", ""],
			["anchor-e2e-eeeeeeeeee-66666666", ""],
		]),
		["anchor-e2e-eeeeeeeeee-66666666"],
	);
});

test("projects outside the Anchor E2E naming scheme are never touched", () => {
	assert.deepEqual(
		select([
			["anchor", "/deleted"],
			["echopoint-e2e-eeeeeeeeee-66666666", ""],
			["anchor-e2e-licensing", ""],
		]),
		[],
	);
});
