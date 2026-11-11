import assert from "node:assert/strict";
import test from "node:test";
import { verifyShards } from "./e2e-shards.mjs";

const inventory = ["a", "b", "c"];
const proof = (shard, ran) => ({ shard, ran, inventory });

test("shards that together ran every scenario once pass", () => {
	assert.deepEqual(
		verifyShards([proof("2/2", ["c"]), proof("1/2", ["a", "b"])]),
		{
			scenarios: 3,
			shards: 2,
		},
	);
});

test("a missing shard fails", () => {
	assert.throws(
		() => verifyShards([proof("1/2", ["a", "b"])]),
		/Expected shards 1, 2/,
	);
});

test("a scenario in no shard fails", () => {
	assert.throws(
		() => verifyShards([proof("1/2", ["a"]), proof("2/2", ["c"])]),
		/1 missing/,
	);
});

test("a dependency that ran in every shard counts once", () => {
	assert.deepEqual(
		verifyShards([proof("1/2", ["a", "b"]), proof("2/2", ["a", "c"])]),
		{ scenarios: 3, shards: 2 },
	);
});

test("a scenario in some but not all shards twice fails", () => {
	assert.throws(
		() =>
			verifyShards([
				{ shard: "1/3", ran: ["a", "b"], inventory },
				{ shard: "2/3", ran: ["b"], inventory },
				{ shard: "3/3", ran: ["c"], inventory },
			]),
		/ran in 2 of 3 shards: b/,
	);
});

test("shards that listed different inventories fail", () => {
	assert.throws(
		() =>
			verifyShards([
				proof("1/2", ["a", "b"]),
				{ shard: "2/2", ran: ["c"], inventory: ["a", "b", "c", "d"] },
			]),
		/different inventory/,
	);
});
