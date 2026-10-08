import assert from "node:assert/strict";
import test from "node:test";
import { seedStripeDemoOrganizations } from "./stripe-prototype-bootstrap.mjs";

const options = () => ({
	productPath: "/v1/products/product-fixture",
	token: "private-owner-bearer",
	fallbackTemplateID: "template-free",
	privateValues: new Set(),
});

test("organization bootstrap exchanges owner authentication for a scoped private product key", async () => {
	const setup = options();
	const calls = [];
	const key = "private-product-key";
	await seedStripeDemoOrganizations(async (path, request) => {
		calls.push({ path, ...request });
		if (path.endsWith("/api-keys")) {
			assert.equal(request.token, setup.token);
			assert.deepEqual(request.body.permissions, ["organization:create"]);
			return { value: key };
		}
		assert.equal(request.key, key);
		assert.equal(request.token, undefined);
		assert.equal(setup.privateValues.has(key), true);
		assert.equal(request.body.license.template_id, setup.fallbackTemplateID);
		return { id: `organization-${calls.length}` };
	}, setup);
	assert.deepEqual(
		calls.map(({ path }) => path),
		[
			`${setup.productPath}/api-keys`,
			`${setup.productPath}/organizations`,
			`${setup.productPath}/organizations`,
		],
	);
	assert.deepEqual(
		calls.slice(1).map(({ body }) => body.name),
		["Maple Studio", "Northstar Labs"],
	);
});

test("missing product key never falls back to owner bearer for organization creation", async () => {
	for (const credential of [undefined, {}, { value: "" }, { value: 123 }]) {
		const setup = options();
		let calls = 0;
		await assert.rejects(
			seedStripeDemoOrganizations(async () => {
				calls += 1;
				return credential;
			}, setup),
			/Anchor did not return a product bootstrap API key/,
		);
		assert.equal(calls, 1);
		assert.equal(setup.privateValues.size, 0);
	}
});

test("organization failure preserves partial data and keeps the key registered for redaction", async () => {
	const setup = options();
	const key = "private-product-key";
	const calls = [];
	await assert.rejects(
		seedStripeDemoOrganizations(async (path) => {
			calls.push(path);
			if (path.endsWith("/api-keys")) return { value: key };
			throw new Error("Organization creation failed");
		}, setup),
		/Organization creation failed/,
	);
	assert.equal(setup.privateValues.has(key), true);
	assert.deepEqual(calls, [
		`${setup.productPath}/api-keys`,
		`${setup.productPath}/organizations`,
	]);
});
