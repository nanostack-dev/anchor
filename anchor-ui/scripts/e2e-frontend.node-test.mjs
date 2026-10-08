import assert from "node:assert/strict";
import test from "node:test";
import { e2eFrontend } from "../e2e/support/frontend.ts";

test("headless frontend defaults to the existing loopback port", () => {
	const previous = process.env.E2E_FRONTEND_PORT;
	Reflect.deleteProperty(process.env, "E2E_FRONTEND_PORT");
	try {
		assert.deepEqual(e2eFrontend(), {
			port: 3015,
			url: "http://127.0.0.1:3015",
		});
	} finally {
		if (previous !== undefined) process.env.E2E_FRONTEND_PORT = previous;
	}
});

test("headless frontend accepts a custom port and both valid boundaries", () => {
	for (const port of [1, 13015, 65535])
		assert.deepEqual(e2eFrontend(String(port)), {
			port,
			url: `http://127.0.0.1:${port}`,
		});
});

test("invalid frontend ports fail before any runtime starts", () => {
	for (const value of [
		"",
		"0",
		"65536",
		"-1",
		"1.5",
		"1e4",
		"NaN",
		" 13015 ",
		"9007199254740992",
	])
		assert.throws(
			() => e2eFrontend(value),
			/E2E_FRONTEND_PORT must be a whole port between 1 and 65535/,
		);
});
