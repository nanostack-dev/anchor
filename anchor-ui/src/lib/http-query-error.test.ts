import { describe, expect, it } from "vitest";
import { createClient } from "../client/client";
import { unwrapQuery } from "./http-query-error";

describe("unwrapQuery", () => {
	it("preserves a generated client's transport error without a response", async () => {
		const failure = new TypeError("connection unavailable");
		const client = createClient({
			baseUrl: "https://example.test",
			fetch: async () => {
				throw failure;
			},
		});
		await expect(unwrapQuery(client.get({ url: "/licenses" }))).rejects.toBe(
			failure,
		);
	});

	it("retains HTTP status and error body for an actual response", async () => {
		const body = { code: "NOT_FOUND" };
		await expect(
			unwrapQuery(
				Promise.resolve({
					error: body,
					response: new Response(null, { status: 404 }),
				}),
			),
		).rejects.toMatchObject({ status: 404, body, name: "HttpQueryError" });
	});

	it("returns successful data", async () => {
		const data = { id: "license-1" };
		await expect(
			unwrapQuery(Promise.resolve({ data, response: new Response() })),
		).resolves.toBe(data);
	});
});
