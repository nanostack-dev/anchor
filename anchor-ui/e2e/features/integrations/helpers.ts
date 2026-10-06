import { createHmac, randomBytes } from "node:crypto";
import { type IncomingHttpHeaders, createServer } from "node:http";
import type { Page } from "playwright/test";
import type { IntegrationInstanceResponse } from "../../../src/client";
import { type World, expect } from "../../support/fixtures";

export async function integrations(world: World) {
	return (
		await world.api.get<{ items: IntegrationInstanceResponse[] }>(
			`${world.productPath}/integrations`,
		)
	).items;
}

export async function configureSMTP(world: World) {
	const instance = await world.api.post<IntegrationInstanceResponse>(
		`${world.productPath}/integrations`,
		{
			provider_type: "SMTP",
			config: {
				host: "127.0.0.1",
				port: world.smtpPort,
				encryption: "NONE",
				auth_method: "PLAIN",
				username: "local-e2e",
				password: "local-e2e-password",
				from_address: "anchor@example.test",
				from_name: "Anchor E2E",
			},
		},
	);
	await expect
		.poll(
			async () =>
				(await integrations(world)).find((item) => item.id === instance.id)
					?.status,
		)
		.toBe("ACTIVE");
	return instance;
}

export async function fillSMTP(page: Page, world: World) {
	await page.getByLabel("Host *", { exact: true }).fill("127.0.0.1");
	await page.getByLabel("Port *", { exact: true }).fill(String(world.smtpPort));
	await page.getByRole("combobox", { name: "Encryption", exact: true }).click();
	await page
		.getByRole("option", { name: "None (dev only)", exact: true })
		.click();
	await page.getByLabel("Username *", { exact: true }).fill("local-e2e");
	await page
		.getByLabel("Password *", { exact: true })
		.fill("local-e2e-password");
	await page
		.getByLabel("From Address *", { exact: true })
		.fill("anchor@example.test");
	await page.getByLabel("From Name", { exact: true }).fill("Anchor E2E");
}

export function webhookSecret() {
	return `whsec_${randomBytes(32).toString("base64")}`;
}

export function signature(
	secret: string,
	id: string,
	timestamp: string,
	body: string,
) {
	return `v1,${createHmac(
		"sha256",
		Buffer.from(secret.replace(/^whsec_/, ""), "base64"),
	)
		.update(`${id}.${timestamp}.${body}`)
		.digest("base64")}`;
}

export async function eventReceiver() {
	const deliveries: { body: string; headers: IncomingHttpHeaders }[] = [];
	const server = createServer(async (request, response) => {
		let body = "";
		for await (const chunk of request) body += chunk.toString();
		deliveries.push({ body, headers: request.headers });
		response.writeHead(204).end();
	});
	await new Promise<void>((ready, reject) => {
		server.once("error", reject);
		server.listen(0, "127.0.0.1", ready);
	});
	const address = server.address();
	if (!address || typeof address === "string")
		throw new Error("Receiver did not bind TCP.");
	return {
		url: `http://127.0.0.1:${address.port}/events`,
		deliveries,
		async close() {
			await new Promise<void>((closed, reject) =>
				server.close((error) => (error ? reject(error) : closed())),
			);
		},
	};
}
