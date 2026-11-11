import { readFile } from "node:fs/promises";

// Proves that the shards of a complete run together ran every scenario of the
// inventory exactly once. Each shard's completion.json (run-e2e-affected.mjs
// --complete --shard i/n) already proves its own slice passed in full.
export function verifyShards(proofs) {
	if (!proofs.length) throw new Error("No shard proofs.");
	const total = Number(proofs[0].shard?.split("/")[1]);
	const indexes = proofs.map(({ shard }) => {
		const [index, count] = (shard ?? "").split("/").map(Number);
		if (!index || count !== total)
			throw new Error(
				`Shard proof ${shard ?? "without a shard"} is not one of ${total}.`,
			);
		return index;
	});
	const wanted = Array.from({ length: total }, (_, i) => i + 1);
	if (indexes.toSorted((a, b) => a - b).join() !== wanted.join())
		throw new Error(
			`Expected shards ${wanted.join(", ")} once each, got ${indexes.join(", ")}.`,
		);
	const inventory = proofs[0].inventory ?? [];
	if (!inventory.length) throw new Error("The full app inventory is empty.");
	for (const proof of proofs)
		if (proof.inventory?.join("\n") !== inventory.join("\n"))
			throw new Error(`Shard ${proof.shard} listed a different inventory.`);
	const seen = new Set();
	for (const proof of proofs)
		for (const identity of proof.ran ?? []) {
			if (seen.has(identity))
				throw new Error(`Scenario ran in two shards: ${identity}`);
			seen.add(identity);
		}
	const missing = inventory.filter((identity) => !seen.has(identity));
	const extra = [...seen].filter((identity) => !inventory.includes(identity));
	if (missing.length || extra.length)
		throw new Error(
			`Incomplete sharded run: ${missing.length} missing, ${extra.length} unexpected scenarios.`,
		);
	return { scenarios: seen.size, shards: total };
}

if (import.meta.main) {
	const files = process.argv.slice(2);
	try {
		const proofs = await Promise.all(
			files.map(async (file) => JSON.parse(await readFile(file, "utf8"))),
		);
		const { scenarios, shards } = verifyShards(proofs);
		console.log(
			`Complete coverage verified: ${scenarios} distinct scenarios passed once across ${shards} shards.`,
		);
	} catch (error) {
		console.error(error.message);
		process.exitCode = 1;
	}
}
