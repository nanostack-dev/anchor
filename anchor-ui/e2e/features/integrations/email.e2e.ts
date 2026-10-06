import type {
	EmailSendRecordResponse,
	EmailTemplateResponse,
	EmailTemplateVersionResponse,
} from "../../../src/client";
import { expect, test } from "../../support/fixtures";
import { selectProduct } from "../../support/ui";
import { configureSMTP } from "./helpers";

// Covers: EMAIL_TEMPLATES, EMAIL_TEMPLATE_BUILDER, EMAIL_SENDS
test("email builder saves content, variables and examples, then publishes and sends through SMTP", async ({
	page,
	world,
}) => {
	const externalRequests: string[] = [];
	page.on("request", (request) => {
		if (new URL(request.url()).hostname !== "127.0.0.1")
			externalRequests.push(request.url());
	});
	await configureSMTP(world);
	await selectProduct(page, world.product);
	await page.goto("/products/email/templates");
	await expect(
		page.getByText("No templates yet", { exact: true }),
	).toBeVisible();
	const creation = page.waitForResponse(
		(response) =>
			response.request().method() === "POST" &&
			new URL(response.url()).pathname ===
				`${world.productPath}/email/templates`,
	);
	await page.getByRole("button", { name: "New Template", exact: true }).click();
	const template = (await (await creation).json()) as EmailTemplateResponse;
	await expect(page).toHaveURL(`/products/email/templates/${template.id}`);
	const templatePath = `${world.productPath}/email/templates/${template.id}`;
	const name = world.name("Welcome email");
	await page
		.getByRole("textbox", { name: "Template name", exact: true })
		.fill(name);
	await page.getByLabel("Subject", { exact: true }).fill("Welcome {{ .name }}");
	const body = "<h1>Hello {{ .name }}</h1><p>Sent by Anchor E2E.</p>";
	const editorRegion = page.getByRole("region", {
		name: "HTML editor",
		exact: true,
	});
	const editor = editorRegion.getByRole("textbox");
	await editorRegion.click();
	// Desktop Chrome emulates Windows; Monaco derives its shortcut from that UA.
	await editor.press("Control+A");
	await editor.press("Backspace");
	await page.keyboard.insertText(body);
	await expect
		.poll(
			async () =>
				(
					await world.api.get<EmailTemplateVersionResponse>(
						`${templatePath}/draft`,
					)
				).body_html,
		)
		.toBe(body);
	await expect
		.poll(
			async () =>
				(await world.api.get<EmailTemplateResponse>(templatePath)).name,
		)
		.toBe(name);
	await page.context().grantPermissions(["clipboard-read", "clipboard-write"]);
	await page.getByRole("button", { name: "Copy HTML", exact: true }).click();
	await expect
		.poll(() => page.evaluate(() => navigator.clipboard.readText()))
		.toBe(body);
	await page.getByRole("tab", { name: /^Variables \(/ }).click();
	await page.getByRole("button", { name: "Push all (1)", exact: true }).click();
	await expect(
		page.getByRole("textbox", { name: "Variable name 1", exact: true }),
	).toHaveValue("name");
	await page.getByRole("checkbox", { name: "Req", exact: true }).check();
	await expect
		.poll(
			async () =>
				(
					await world.api.get<EmailTemplateVersionResponse>(
						`${templatePath}/draft`,
					)
				).variables,
		)
		.toEqual([{ name: "name", type: "STRING", required: true }]);
	await page.getByRole("tab", { name: "Examples", exact: true }).click();
	await page.getByRole("button", { name: "New Example", exact: true }).click();
	await page
		.getByPlaceholder("Example name", { exact: true })
		.fill("Friendly reader");
	await page.getByRole("button", { name: "Raw", exact: true }).click();
	const example = page.getByPlaceholder(
		'{"userName": "Alice", "orderTotal": 99.99}',
		{ exact: true },
	);
	await example.fill("[]");
	await expect(
		page.getByText("Must be a JSON object { ... }", { exact: true }),
	).toBeVisible();
	await example.fill('{"name":"Ada"}');
	await page
		.getByRole("button", { name: "Save Examples", exact: true })
		.click();
	await expect
		.poll(
			async () =>
				(
					await world.api.get<{
						examples: { name: string; variables: object }[];
					}>(`${templatePath}/examples`)
				).examples,
		)
		.toEqual([
			expect.objectContaining({
				name: "Friendly reader",
				variables: { name: "Ada" },
			}),
		]);
	await expect(page.getByText("Welcome Ada", { exact: true })).toBeVisible();
	await expect(
		page
			.getByTitle("Email Preview", { exact: true })
			.contentFrame()
			.getByRole("heading", { name: "Hello Ada", exact: true }),
	).toBeVisible();
	const refreshedPreview = page.waitForResponse(
		(response) =>
			response.request().method() === "POST" &&
			new URL(response.url()).pathname === `${templatePath}/preview`,
	);
	await page
		.getByRole("button", { name: "Refresh Preview", exact: true })
		.click();
	expect((await refreshedPreview).status()).toBe(200);
	await page.getByRole("button", { name: "Publish", exact: true }).click();
	await expect(page.getByText("Published", { exact: true })).toBeVisible();
	await page.reload();
	await expect(
		page.getByRole("textbox", { name: "Template name", exact: true }),
	).toHaveValue(name);
	await expect(page.getByLabel("Subject", { exact: true })).toHaveValue(
		"Welcome {{ .name }}",
	);
	await page.getByRole("tab", { name: "Examples", exact: true }).click();
	await expect(
		page.getByPlaceholder("Example name", { exact: true }),
	).toHaveValue("Friendly reader");
	await page.getByRole("button", { name: "Send Test", exact: true }).click();
	const dialog = page.getByRole("dialog", {
		name: "Send Test Email",
		exact: true,
	});
	const recipient = `${world.name("recipient")}@example.test`;
	await dialog.getByLabel("Recipient", { exact: true }).fill(recipient);
	await dialog.getByPlaceholder("required", { exact: true }).fill("Ada");
	await dialog.getByRole("button", { name: "Send", exact: true }).click();
	await expect(dialog.getByText(/Sent — status:/)).toBeVisible();
	await expect
		.poll(async () => {
			const messages = (await fetch(`${world.mailpitURL}/api/v1/messages`).then(
				(response) => response.json(),
			)) as {
				messages: { Subject: string; To: { Address: string }[] }[];
			};
			return messages.messages.find((message) =>
				message.To.some((address) => address.Address === recipient),
			)?.Subject;
		})
		.toBe("Welcome Ada");
	await page.keyboard.press("Escape");
	await page.goto("/products/email/sends");
	const row = page.getByRole("row", { name: new RegExp(recipient) });
	await expect(row).toContainText("Welcome Ada");
	await expect(row).toContainText("SENT");
	await page.getByRole("button", { name: "Status", exact: true }).click();
	await page.getByRole("option", { name: "Failed", exact: true }).click();
	await expect(row).toHaveCount(0);
	await expect(page.getByText(/^No sends match your filters$/)).toBeVisible();
	await page.getByRole("button", { name: "Clear all", exact: true }).click();
	await expect(row).toBeVisible();
	expect(
		externalRequests,
		"email editor assets must load from the app",
	).toEqual([]);
});

// Covers: EMAIL_TEMPLATE_BUILDER
test("email variable schemas and example forms preserve primitive, object and list values", async ({
	page,
	world,
}) => {
	const template = await world.api.post<EmailTemplateResponse>(
		`${world.productPath}/email/templates`,
		{
			slug: world.name("typed-email"),
			name: world.name("Typed email"),
			subject: "Invoice",
			body_html:
				'<h1>Invoice {{ printf "%.2f" .count }}</h1><p>{{ if .enabled }}Enabled{{ else }}Disabled{{ end }}</p><p>{{ .address.city }}</p><ul>{{ range .items }}<li>{{ .sku }}: {{ .amount }}</li>{{ end }}</ul><ul>{{ range .tags }}<li>{{ . }}</li>{{ end }}</ul>',
			variables: ["count", "enabled", "address", "items", "tags"].map(
				(name) => ({
					name,
					type: "STRING",
				}),
			),
		},
	);
	const templatePath = `${world.productPath}/email/templates/${template.id}`;
	await selectProduct(page, world.product);
	await page.goto(`/products/email/templates/${template.id}`);
	await page.getByRole("tab", { name: /^Variables \(/ }).click();
	const variables = page.getByRole("tabpanel", { name: /^Variables \(/ });
	for (const [name, type] of [
		["count", "NUMBER"],
		["enabled", "BOOL"],
		["address", "OBJECT"],
		["items", "LIST"],
		["tags", "LIST"],
	]) {
		await variables
			.getByRole("combobox", { name: `Type for variable ${name}`, exact: true })
			.click();
		await page.getByRole("option", { name: type, exact: true }).click();
	}
	await variables.getByRole("button", { name: "+ field", exact: true }).click();
	await variables
		.getByRole("textbox", { name: "Property name 1", exact: true })
		.fill("city");
	await variables.getByRole("button", { name: "+ field", exact: true }).click();
	await variables
		.getByRole("textbox", { name: "Property name 2", exact: true })
		.fill("remove-field");
	await variables
		.getByRole("button", { name: "Remove field remove-field", exact: true })
		.click();
	await variables
		.getByRole("combobox", {
			name: "Item type for variable items",
			exact: true,
		})
		.click();
	await page.getByRole("option", { name: "OBJECT", exact: true }).click();
	// Repeated nested-property controls follow the displayed variable order.
	await variables
		.getByRole("button", { name: "+ field", exact: true })
		.nth(1)
		.click();
	await variables
		.getByRole("textbox", { name: "Property name 1", exact: true })
		.nth(1)
		.fill("sku");
	await variables
		.getByRole("button", { name: "+ field", exact: true })
		.nth(1)
		.click();
	await variables
		.getByRole("textbox", { name: "Property name 2", exact: true })
		.fill("amount");
	await variables
		.getByRole("combobox", { name: "Type for property amount", exact: true })
		.click();
	await page.getByRole("option", { name: "NUMBER", exact: true }).click();
	await variables
		.getByRole("combobox", { name: "Item type for variable tags", exact: true })
		.click();
	await page.getByRole("option", { name: "NUMBER", exact: true }).click();
	await variables
		.getByRole("combobox", { name: "Item type for variable tags", exact: true })
		.click();
	await page.getByRole("option", { name: "STRING", exact: true }).click();
	await variables
		.getByRole("button", { name: "+ Add Variable", exact: true })
		.click();
	await variables
		.getByRole("textbox", { name: "Variable name 6", exact: true })
		.fill("remove-variable");
	await variables
		.getByRole("button", {
			name: "Remove variable remove-variable",
			exact: true,
		})
		.click();
	await expect
		.poll(
			async () =>
				(
					await world.api.get<EmailTemplateVersionResponse>(
						`${templatePath}/draft`,
					)
				).variables,
		)
		.toEqual([
			{ name: "count", type: "NUMBER" },
			{ name: "enabled", type: "BOOL" },
			{
				name: "address",
				type: "OBJECT",
				properties: [{ name: "city", type: "STRING" }],
			},
			{
				name: "items",
				type: "LIST",
				items: {
					type: "OBJECT",
					properties: [
						{ name: "sku", type: "STRING" },
						{ name: "amount", type: "NUMBER" },
					],
				},
			},
			{ name: "tags", type: "LIST", items: { type: "STRING" } },
		]);
	await page.getByRole("tab", { name: "Examples", exact: true }).click();
	const examples = page.getByRole("tabpanel", {
		name: "Examples",
		exact: true,
	});
	await examples
		.getByRole("button", { name: "New Example", exact: true })
		.click();
	await examples
		.getByPlaceholder("Example name", { exact: true })
		.fill("Typed example");
	await expect(
		examples.getByPlaceholder("value for .2f", { exact: true }),
	).toHaveCount(0);
	await examples
		.getByPlaceholder("value for .count", { exact: true })
		.fill("12.5");
	await examples
		.getByPlaceholder("value for .enabled", { exact: true })
		.fill("false");
	await examples
		.getByPlaceholder('["value1", "value2"]', { exact: true })
		.fill('["alpha","beta"]');
	await examples.getByPlaceholder("—", { exact: true }).fill("Montréal");
	await examples
		.getByRole("button", { name: "+ Add row", exact: true })
		.click();
	await examples
		.getByPlaceholder("—", { exact: true })
		.nth(0)
		.fill("invoice-line");
	await examples.getByPlaceholder("—", { exact: true }).nth(1).fill("7");
	await examples
		.getByRole("button", { name: "+ Add row", exact: true })
		.click();
	await examples
		.getByRole("button", { name: "Remove row 2", exact: true })
		.click();
	await examples
		.getByRole("button", { name: "Save Examples", exact: true })
		.click();
	await expect
		.poll(
			async () =>
				(
					await world.api.get<{ examples: { variables: object }[] }>(
						`${templatePath}/examples`,
					)
				).examples[0]?.variables,
		)
		.toEqual({
			count: "12.5",
			enabled: "false",
			address: '{"city":"Montréal"}',
			items: '[{"sku":"invoice-line","amount":"7"}]',
			tags: '["alpha","beta"]',
		});
	const preview = page
		.getByTitle("Email Preview", { exact: true })
		.contentFrame();
	await expect(
		preview.getByRole("heading", { name: "Invoice 12.50", exact: true }),
	).toBeVisible();
	await expect(preview.getByText("Disabled", { exact: true })).toBeVisible();
	await expect(preview.getByText("Montréal", { exact: true })).toBeVisible();
	await expect(preview.getByRole("listitem")).toHaveText([
		"invoice-line: 7",
		"alpha",
		"beta",
	]);
	await page.reload();
	await page.getByRole("tab", { name: "Examples", exact: true }).click();
	await expect(
		page.getByPlaceholder("value for .enabled", { exact: true }),
	).toHaveValue("false");
	await page
		.getByRole("button", { name: "Delete example", exact: true })
		.click();
	await expect(
		page.getByText("No examples yet", { exact: true }),
	).toBeVisible();
	await page
		.getByRole("button", { name: "Save Examples", exact: true })
		.click();
	await expect
		.poll(
			async () =>
				(
					await world.api.get<{ examples: object[] }>(
						`${templatePath}/examples`,
					)
				).examples,
		)
		.toEqual([]);
});

// Covers: EMAIL_TEMPLATES, EMAIL_SENDS
test("email lists paginate templates and delivered send records", async ({
	page,
	world,
}) => {
	await configureSMTP(world);
	const templates: EmailTemplateResponse[] = [];
	for (let index = 0; index < 11; index++) {
		templates.push(
			await world.api.post<EmailTemplateResponse>(
				`${world.productPath}/email/templates`,
				{
					slug: world.name(`page-${index}`),
					name: world.name(`Page ${index}`),
					subject: `Page ${index}`,
					body_html: `<p>Page ${index}</p>`,
				},
			),
		);
	}
	await world.api.post(
		`${world.productPath}/email/templates/${templates[0].id}/publish`,
		{},
		200,
	);
	const records: EmailSendRecordResponse[] = [];
	for (let index = 0; index < 11; index++) {
		records.push(
			await world.api.post<EmailSendRecordResponse>(
				`${world.productPath}/email/sends`,
				{
					template_id: templates[0].id,
					to_address: `${world.name(`page-${index}`)}@example.test`,
					variables: {},
				},
			),
		);
	}
	await expect
		.poll(
			async () =>
				(
					await world.api.get<{ items: EmailSendRecordResponse[] }>(
						`${world.productPath}/email/sends`,
					)
				).items.filter((record) => record.status === "SENT").length,
		)
		.toBe(11);
	await selectProduct(page, world.product);
	for (const [route, first, last] of [
		["templates", templates[10].name, templates[0].name],
		["sends", records[10].to_address, records[0].to_address],
	]) {
		await page.goto(`/products/email/${route}`);
		await page
			.getByRole("combobox", { name: "Rows per page", exact: true })
			.click();
		await page.getByRole("option", { name: "Show 10", exact: true }).click();
		await expect(page.getByText("11 total", { exact: true })).toBeVisible();
		await expect(
			page.getByRole("row", { name: new RegExp(first) }),
		).toBeVisible();
		await expect(page.getByRole("row", { name: new RegExp(last) })).toHaveCount(
			0,
		);
		await page.getByRole("button", { name: "Next", exact: true }).click();
		await expect(
			page.getByRole("row", { name: new RegExp(last) }),
		).toBeVisible();
		await expect(
			page.getByRole("row", { name: new RegExp(first) }),
		).toHaveCount(0);
		await expect(
			page.getByRole("button", { name: "Next", exact: true }),
		).toBeDisabled();
		await page.getByRole("button", { name: "Previous", exact: true }).click();
		await expect(
			page.getByRole("row", { name: new RegExp(first) }),
		).toBeVisible();
		await expect(
			page.getByRole("button", { name: "Previous", exact: true }),
		).toBeDisabled();
	}
});

// Covers: EMAIL_TEMPLATES
test("email template list deletes one row and only the selected rows in bulk", async ({
	page,
	world,
}) => {
	const templates: EmailTemplateResponse[] = [];
	for (const prefix of [
		"delete-one",
		"bulk-first",
		"bulk-second",
		"keep-template",
	]) {
		templates.push(
			await world.api.post<EmailTemplateResponse>(
				`${world.productPath}/email/templates`,
				{
					slug: world.name(prefix),
					name: world.name(prefix),
					subject: "Fixture",
					body_html: "<p>Fixture</p>",
				},
			),
		);
	}
	await selectProduct(page, world.product);
	await page.goto("/products/email/templates");
	const first = page.getByRole("row", { name: new RegExp(templates[0].name) });
	await first.getByRole("button", { name: "Delete", exact: true }).click();
	await expect(first).toHaveCount(0);
	for (const template of templates.slice(1, 3)) {
		await page
			.getByRole("row", { name: new RegExp(template.name) })
			.getByRole("checkbox", { name: "Select row", exact: true })
			.check();
	}
	await page.getByRole("button", { name: "Bulk actions", exact: true }).click();
	await page
		.getByRole("menuitem", { name: "Delete selected", exact: true })
		.click();
	await page
		.getByRole("alertdialog")
		.getByRole("button", { name: "Delete selected", exact: true })
		.click();
	await expect(
		page.getByText("2 succeeded. 0 failed.", { exact: true }),
	).toBeVisible();
	for (const template of templates.slice(0, 3))
		await expect(
			page.getByRole("row", { name: new RegExp(template.name) }),
		).toHaveCount(0);
	await expect(
		page.getByRole("row", { name: new RegExp(templates[3].name) }),
	).toBeVisible();
	const remaining = await world.api.get<{ items: EmailTemplateResponse[] }>(
		`${world.productPath}/email/templates`,
	);
	expect(remaining.items.map((template) => template.id)).toEqual([
		templates[3].id,
	]);
});
