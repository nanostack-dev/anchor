import type { OrganizationLicenseResponse } from "../../../src/client";
import { expect, test } from "../../support/fixtures";
import {
	adjustLicense,
	chooseOption,
	createOrganization,
	createTemplate,
	declareSchema,
	licensePath,
	openLicensing,
	values,
} from "./helpers";

// Covers: ORGANIZATION_LICENSE, ORGANIZATION_LICENSE_DETAIL
test("selected migrations preview tiers, carry customer adjustments and report changed and unchanged customers", async ({
	page,
	world,
}) => {
	await declareSchema(world);
	const source = await createTemplate(world, "source");
	const target = await createTemplate(world, "target", {
		max_flows: 50,
		seats: 20,
		tier: "pro",
	});
	const archived = await createTemplate(world, "withdrawn");
	await world.api.post(
		`${world.productPath}/licensing/templates/${archived.id}/archive`,
		{},
		200,
	);
	const customized = await createOrganization(world, source, "custom");
	const unchanged = await createOrganization(world, target, "already-target");
	const outsider = await createOrganization(world, source, "unselected");
	await adjustLicense(world, customized, { max_flows: 25 });
	await openLicensing(page, world, "/organizations/license");
	for (const organization of [customized, unchanged])
		await page
			.getByRole("row")
			.filter({ hasText: organization.name })
			.getByRole("checkbox", { name: "Select row", exact: true })
			.check();
	await page
		.getByRole("button", { name: "Move 2 to another tier", exact: true })
		.click();
	const dialog = page.getByRole("dialog", {
		name: "Move 2 organizations",
		exact: true,
	});
	await expect(
		dialog.getByRole("button", { name: "Move 2 organizations", exact: true }),
	).toBeDisabled();
	await dialog
		.getByRole("combobox", { name: "Move to tier", exact: true })
		.click();
	await expect(
		page.getByRole("option", { name: archived.name, exact: true }),
	).toHaveCount(0);
	await page.getByRole("option", { name: target.name, exact: true }).click();
	await expect(
		dialog.getByRole("switch", {
			name: "Discard customer adjustments",
			exact: true,
		}),
	).not.toBeChecked();
	await expect(dialog).toContainText("Adjustments kept");
	await expect(dialog).toContainText(customized.name);
	await dialog
		.getByRole("button", { name: "Move 2 organizations", exact: true })
		.click();
	const results = page.getByRole("dialog", {
		name: "Migration results",
		exact: true,
	});
	await expect(
		results.getByRole("heading", { name: /^Set\s*1$/ }),
	).toBeVisible();
	await expect(
		results.getByRole("heading", { name: /^Already there\s*1$/ }),
	).toBeVisible();
	await expect(
		results.getByRole("listitem").filter({ hasText: customized.name }),
	).toContainText("Set");
	await expect(
		results.getByRole("listitem").filter({ hasText: unchanged.name }),
	).toContainText("Already held these values from this tier.");
	await results.getByRole("button", { name: "Done", exact: true }).click();
	const license = await world.api.get<OrganizationLicenseResponse>(
		licensePath(world, customized),
	);
	expect(license.template_id).toBe(target.id);
	expect(license.values).toEqual({ ...target.values, max_flows: 25 });
	expect(license.adjusted_fields).toEqual(["max_flows"]);
	expect(
		(
			await world.api.get<OrganizationLicenseResponse>(
				licensePath(world, outsider),
			)
		).template_id,
	).toBe(source.id);
	await page.goto(`/organizations/license/${customized.id}/changes`);
	await expect(
		page.getByText("Moved to another tier", { exact: true }),
	).toBeVisible();
	await expect(
		page
			.getByRole("listitem")
			.filter({ hasText: "Moved to another tier" })
			.getByText(source.name, { exact: true }),
	).toBeVisible();
});

// Covers: ORGANIZATION_LICENSE, ORGANIZATION_LICENSE_DETAIL
test("all matching query spans pages, excludes outsiders and can discard adjustments while granting a first license", async ({
	page,
	world,
}) => {
	await declareSchema(world);
	const source = await createTemplate(world, "source");
	const target = await createTemplate(world, "target", {
		max_flows: 50,
		tier: "pro",
	});
	const cohort = world.name("cohort");
	const customers = [];
	for (let index = 0; index < 21; index += 1)
		customers.push(
			await createOrganization(world, index === 0 ? undefined : source, cohort),
		);
	const customized = customers[1];
	const unlicensed = customers[0];
	const outsider = await createOrganization(world, source, "excluded");
	await adjustLicense(world, customized, { max_flows: 25 });
	await openLicensing(page, world, "/organizations/license");
	await page
		.getByRole("textbox", { name: "Search organizations", exact: true })
		.fill(cohort);
	await expect(
		page.getByRole("row").filter({ hasText: outsider.name }),
	).toHaveCount(0);
	await expect(
		page.getByRole("checkbox", { name: "Select row", exact: true }),
	).toHaveCount(20);
	await page.getByRole("button", { name: "Next", exact: true }).click();
	await expect(
		page.getByRole("checkbox", { name: "Select row", exact: true }),
	).toHaveCount(1);
	await expect(
		page.getByRole("button", { name: "Next", exact: true }),
	).toBeDisabled();
	await page.getByRole("button", { name: "Previous", exact: true }).click();
	await expect(
		page.getByRole("checkbox", { name: "Select row", exact: true }),
	).toHaveCount(20);
	await expect(
		page.getByRole("button", { name: "Previous", exact: true }),
	).toBeDisabled();
	await page
		.getByRole("button", { name: "Selection options", exact: true })
		.click();
	await page
		.getByRole("menuitem", { name: "All in current page", exact: true })
		.click();
	await expect(
		page.getByRole("button", { name: "Move 20 to another tier", exact: true }),
	).toBeVisible();
	await page
		.getByRole("button", { name: "Selection options", exact: true })
		.click();
	await page.getByRole("menuitem", { name: "None", exact: true }).click();
	await expect(
		page.getByRole("checkbox", { name: "Select all", exact: true }),
	).not.toBeChecked();
	await page
		.getByRole("button", { name: "Selection options", exact: true })
		.click();
	await page
		.getByRole("menuitem", { name: "All matching query", exact: true })
		.click();
	await page
		.getByRole("button", { name: /Move all 21 to another tier/ })
		.click();
	const dialog = page.getByRole("dialog", {
		name: "Move 21 organizations",
		exact: true,
	});
	await chooseOption(
		page,
		dialog.getByRole("combobox", { name: "Move to tier", exact: true }),
		target.name,
	);
	await expect(dialog).toContainText("Adjustments kept");
	await dialog
		.getByRole("switch", { name: "Discard customer adjustments", exact: true })
		.check();
	await expect(
		dialog.getByText("Adjustments kept", { exact: true }),
	).toHaveCount(0);
	await dialog
		.getByRole("button", { name: "Move 21 organizations", exact: true })
		.click();
	const results = page.getByRole("dialog", {
		name: "Migration results",
		exact: true,
	});
	await expect(
		results.getByRole("heading", { name: /^Set\s*21$/ }),
	).toBeVisible();
	await expect(
		results.getByRole("listitem").filter({ hasText: unlicensed.name }),
	).toContainText("Granted this tier");
	await results.getByRole("button", { name: "Done", exact: true }).click();
	for (const organization of customers) {
		const license = await world.api.get<OrganizationLicenseResponse>(
			licensePath(world, organization),
		);
		expect(license.template_id).toBe(target.id);
		expect(license.values).toEqual({ ...values, ...target.values });
		expect(license.adjusted_fields).toEqual([]);
	}
	expect(
		(
			await world.api.get<OrganizationLicenseResponse>(
				licensePath(world, outsider),
			)
		).template_id,
	).toBe(source.id);
	await page.goto(`/organizations/license/${unlicensed.id}/changes`);
	await expect(
		page.getByText("Licensed for the first time", { exact: true }),
	).toBeVisible();
});

// Covers: ORGANIZATION_LICENSE
test("a migration reports a disappeared customer as failed while continuing successful writes", async ({
	page,
	world,
}) => {
	await declareSchema(world);
	const target = await createTemplate(world);
	const removed = await createOrganization(
		world,
		undefined,
		"removed-during-review",
	);
	const surviving = await createOrganization(world, undefined, "surviving");
	await openLicensing(page, world, "/organizations/license");
	await page.getByRole("checkbox", { name: "Select all", exact: true }).check();
	await page
		.getByRole("button", { name: "Move 2 to another tier", exact: true })
		.click();
	const dialog = page.getByRole("dialog", {
		name: "Move 2 organizations",
		exact: true,
	});
	await chooseOption(
		page,
		dialog.getByRole("combobox", { name: "Move to tier", exact: true }),
		target.name,
	);
	await (await world.productAPI()).remove(
		`${world.productPath}/organizations/${removed.id}`,
	);
	await dialog
		.getByRole("button", { name: "Move 2 organizations", exact: true })
		.click();
	const results = page.getByRole("dialog", {
		name: "Migration results",
		exact: true,
	});
	await expect(
		results.getByRole("heading", { name: /^Failed\s*1$/ }),
	).toBeVisible();
	await expect(
		results.getByRole("listitem").filter({ hasText: removed.name }),
	).toContainText("Failed");
	await expect(
		results.getByRole("heading", { name: /^Set\s*1$/ }),
	).toBeVisible();
	await expect(
		results.getByRole("listitem").filter({ hasText: surviving.name }),
	).toContainText("Granted this tier");
	expect(
		(
			await world.api.get<OrganizationLicenseResponse>(
				licensePath(world, surviving),
			)
		).template_id,
	).toBe(target.id);
	await results.getByRole("button", { name: "Done", exact: true }).click();
});
