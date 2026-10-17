import { expect, test } from "../../support/fixtures";
import { chooseOption, openLicensing } from "./helpers";

// Covers: PRODUCT_LICENSE_SCHEMA, PRODUCT_LICENSE_TEMPLATES, ORGANIZATION_LICENSE
test("missing schemas guide the operator from templates and organization licenses to declaration", async ({
	page,
	world,
}) => {
	await openLicensing(page, world, "/products/licensing/templates");
	await expect(
		page.getByText("No license schema declared yet", { exact: true }),
	).toBeVisible();
	await page
		.getByRole("link", { name: "Go to License Schema", exact: true })
		.click();
	await expect(
		page.getByText("No license schema declared", { exact: true }),
	).toBeVisible();
	await page
		.getByRole("button", { name: "Create Schema", exact: true })
		.click();
	const dialog = page.getByRole("dialog", {
		name: "Create License Schema",
		exact: true,
	});
	await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
	await expect(dialog).not.toBeVisible();
	await page.goto("/organizations/license");
	await expect(
		page.getByText("No license schema declared yet", { exact: true }),
	).toBeVisible();
	await expect(
		page.getByRole("link", { name: "Go to License Schema", exact: true }),
	).toBeVisible();
});

// Covers: PRODUCT_LICENSE_SCHEMA
test("visual schema declares every field type and preserves rules through text conversion and editing", async ({
	page,
	world,
}) => {
	await openLicensing(page, world, "/products/licensing/schema");
	await page
		.getByRole("button", { name: "Create Schema", exact: true })
		.click();
	const dialog = page.getByRole("dialog", {
		name: "Create License Schema",
		exact: true,
	});
	await dialog
		.getByRole("textbox", { name: "Schema description", exact: true })
		.fill("Browser-created license contract");

	for (const field of [
		{ name: "webhook_url", type: "String" },
		{ name: "seats", type: "Number" },
		{ name: "sso", type: "Boolean" },
		{ name: "tier", type: "Enum" },
		{ name: "max_flows", type: "Limit", shape: "Gauge" },
		{ name: "monthly_runs", type: "Limit", shape: "Windowed counter" },
	]) {
		if (field.name === "seats")
			await dialog
				.getByRole("textbox", { name: "Name", exact: true })
				.press("Enter");
		else if (field.name !== "webhook_url")
			await dialog
				.getByRole("button", { name: "Add field", exact: true })
				.click();
		await expect(
			dialog.getByRole("textbox", { name: "Name", exact: true }),
		).toHaveCount(1);
		await dialog
			.getByRole("group", { name: "Field New field", exact: true })
			.getByRole("textbox", { name: "Name", exact: true })
			.fill(field.name);
		const row = dialog.getByRole("group", {
			name: `Field ${field.name}`,
			exact: true,
		});
		await chooseOption(
			page,
			row.getByRole("combobox", { name: "Type", exact: true }),
			field.type,
		);
		if (field.type === "String") {
			await row
				.getByRole("textbox", {
					name: "Pattern (regular expression)",
					exact: true,
				})
				.fill("^https://.*$");
			await row
				.getByRole("spinbutton", { name: "Min length", exact: true })
				.fill("1");
			await row
				.getByRole("spinbutton", { name: "Max length", exact: true })
				.fill("2048");
		} else if (field.type === "Enum") {
			await row
				.getByRole("textbox", {
					name: "Allowed values (comma-separated)",
					exact: true,
				})
				.fill("free, pro, enterprise");
		} else if (field.type === "Number" || field.type === "Limit") {
			await row
				.getByRole("spinbutton", { name: "Min", exact: true })
				.fill(field.type === "Number" ? "1" : "0");
			await row
				.getByRole("spinbutton", { name: "Max", exact: true })
				.fill("1000");
			if (field.shape)
				await chooseOption(
					page,
					row.getByRole("combobox", { name: "Usage shape", exact: true }),
					field.shape,
				);
		}
	}
	await dialog.getByRole("button", { name: "Text", exact: true }).click();
	await expect(
		dialog.getByRole("textbox", { name: "Fields", exact: true }),
	).toHaveValue(/max_flows:\s+limit gauge 0\.\.1000/);
	await expect(
		dialog.getByRole("textbox", { name: "Fields", exact: true }),
	).toHaveValue(/monthly_runs:\s+limit windowed_counter 0\.\.1000/);
	await dialog.getByRole("button", { name: "Visual", exact: true }).click();
	await expect(dialog.getByText("6 fields", { exact: true })).toBeVisible();
	await dialog
		.getByRole("button", { name: "Create Schema", exact: true })
		.click();
	await expect(dialog).not.toBeVisible();
	await expect(
		page.getByRole("row").filter({ hasText: "webhook_url" }),
	).toContainText("max length 2048");
	await expect(page.getByRole("row").filter({ hasText: "sso" })).toContainText(
		"No rules apply",
	);

	await page.getByRole("button", { name: "Edit Schema", exact: true }).click();
	const edit = page.getByRole("dialog", {
		name: "Edit License Schema",
		exact: true,
	});
	await edit
		.getByRole("textbox", { name: "Schema description", exact: true })
		.fill("Revised through the browser");
	await edit.getByRole("button", { name: /^seats Number/ }).click();
	await edit.getByRole("spinbutton", { name: "Max", exact: true }).fill("500");
	await edit.getByRole("button", { name: "Remove sso", exact: true }).click();
	await edit.getByRole("button", { name: "Save Schema", exact: true }).click();
	await expect(edit).not.toBeVisible();
	await page.reload();
	await expect(
		page.getByText("Revised through the browser", { exact: true }),
	).toBeVisible();
	await expect(
		page.getByRole("row").filter({ hasText: "seats" }),
	).toContainText("max 500");
	await expect(page.getByRole("row").filter({ hasText: "sso" })).toHaveCount(0);
});

// Covers: PRODUCT_LICENSE_SCHEMA
test("text schemas refuse invalid source, focus its diagnostic, and save a corrected declaration", async ({
	page,
	world,
}) => {
	await openLicensing(page, world, "/products/licensing/schema");
	await page
		.getByRole("button", { name: "Create Schema", exact: true })
		.click();
	const dialog = page.getByRole("dialog", {
		name: "Create License Schema",
		exact: true,
	});
	await dialog.getByRole("button", { name: "Text", exact: true }).click();
	const source = dialog.getByRole("textbox", { name: "Fields", exact: true });
	await source.fill("max_flows: limit 0..100");
	await expect(
		dialog.getByRole("button", { name: "Create Schema", exact: true }),
	).toBeDisabled();
	await dialog
		.getByRole("button", { name: /A limit needs.*gauge.*windowed_counter/ })
		.click();
	await expect(source).toBeFocused();
	await source.fill(
		"max_flows: limit gauge 0..100 # Current flows\nmonthly_runs: limit windowed_counter 0..1000\nseats: number 1..100\nsso: boolean\ntier: enum free | pro\nwebhook_url: string /^https:\\/\\/.*$/ len 1..2048",
	);
	await expect(
		dialog.getByText("6 fields parsed.", { exact: true }),
	).toBeVisible();
	await expect(
		dialog.getByRole("button", { name: "Create Schema", exact: true }),
	).toBeEnabled();
	await dialog.getByRole("button", { name: "Visual", exact: true }).click();
	await dialog.getByRole("button", { name: /^max_flows Limit/ }).click();
	await expect(
		dialog.getByRole("combobox", { name: "Usage shape", exact: true }),
	).toContainText("Gauge");
	await expect(
		dialog.getByRole("textbox", { name: "Description", exact: true }),
	).toHaveValue("Current flows");
	await dialog
		.getByRole("button", { name: "Create Schema", exact: true })
		.click();
	await expect(dialog).not.toBeVisible();
	await page.reload();
	await expect(
		page.getByRole("row").filter({ hasText: "max_flows" }),
	).toContainText("Current flows");
	await expect(page.getByRole("row").filter({ hasText: "tier" })).toContainText(
		"values: free, pro",
	);
});

// Covers: PRODUCT_LICENSE_SCHEMA
test("visual schema validation keeps invalid names, ranges and missing usage shapes from being submitted", async ({
	page,
	world,
}) => {
	await openLicensing(page, world, "/products/licensing/schema");
	await page
		.getByRole("button", { name: "Create Schema", exact: true })
		.click();
	const dialog = page.getByRole("dialog", {
		name: "Create License Schema",
		exact: true,
	});
	const save = dialog.getByRole("button", {
		name: "Create Schema",
		exact: true,
	});
	await save.click();
	await expect(
		dialog.getByText("Name is required.", { exact: true }),
	).toBeVisible();
	await dialog
		.getByRole("textbox", { name: "Name", exact: true })
		.fill("flows");
	await chooseOption(
		page,
		dialog.getByRole("combobox", { name: "Type", exact: true }),
		"Limit",
	);
	await save.click();
	await expect(
		dialog.getByText("Choose a usage shape for this limit.", { exact: true }),
	).toBeVisible();
	await chooseOption(
		page,
		dialog.getByRole("combobox", { name: "Usage shape", exact: true }),
		"Gauge",
	);
	await dialog.getByRole("spinbutton", { name: "Min", exact: true }).fill("10");
	await dialog.getByRole("spinbutton", { name: "Max", exact: true }).fill("1");
	await save.click();
	await expect(
		dialog.getByText("Min must not exceed max.", { exact: true }),
	).toBeVisible();
	await dialog.getByRole("spinbutton", { name: "Max", exact: true }).fill("20");
	await dialog.getByRole("button", { name: "Add field", exact: true }).click();
	await dialog
		.getByRole("textbox", { name: "Name", exact: true })
		.fill("flows");
	await save.click();
	await expect(
		dialog
			.getByRole("group", { name: "Field flows", exact: true })
			.getByText("This field name is used more than once.", { exact: true }),
	).toBeVisible();
	await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
	await expect(
		page.getByText("No license schema declared", { exact: true }),
	).toBeVisible();
});
