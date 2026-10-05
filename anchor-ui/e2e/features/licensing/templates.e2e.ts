import type {
	LicenseTemplateResponse,
	OrganizationLicenseResponse,
} from "../../../src/client";
import { expect, test } from "../../support/fixtures";
import {
	adjustLicense,
	chooseOption,
	createOrganization,
	createTemplate,
	declareSchema,
	fillTemplateValues,
	licensePath,
	openLicensing,
} from "./helpers";

// Covers: PRODUCT_LICENSE_TEMPLATES, PRODUCT_LICENSE_TEMPLATE_NEW, PRODUCT_LICENSE_TEMPLATE_DETAIL
test("templates validate every required value and support create, view, cancel and persistent editing", async ({
	page,
	world,
}) => {
	await declareSchema(world);
	await openLicensing(page, world, "/products/licensing/templates");
	await page
		.getByRole("link", { name: "Create Template", exact: true })
		.click();
	await expect(
		page.getByRole("heading", { name: "Create License Template", exact: true }),
	).toBeVisible();
	await page
		.getByRole("button", { name: "Create Template", exact: true })
		.click();
	await expect(
		page.getByText("Name is required.", { exact: true }),
	).toBeVisible();
	await expect(
		page.getByText("This field is required.", { exact: true }),
	).toHaveCount(6);
	const name = world.name("browser-tier");
	await page.getByRole("textbox", { name: "Name", exact: true }).fill(name);
	await page
		.getByRole("textbox", { name: "Description", exact: true })
		.fill("Created from every schema type");
	await fillTemplateValues(page);
	await page
		.getByRole("spinbutton", { name: "max_flows", exact: true })
		.fill("1001");
	await page
		.getByRole("button", { name: "Create Template", exact: true })
		.click();
	await expect(
		page.getByRole("spinbutton", { name: "max_flows", exact: true }),
	).toHaveAttribute("aria-invalid", "true");
	await page
		.getByRole("spinbutton", { name: "max_flows", exact: true })
		.fill("25");
	await page
		.getByRole("button", { name: "Create Template", exact: true })
		.click();
	await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
	await expect(
		page.getByRole("region", { name: "Template details", exact: true }),
	).toContainText("https://pro.example.test");
	const createdToast = page.getByRole("dialog", {
		name: "License template created",
		exact: true,
	});
	await expect(createdToast).toBeVisible();
	await createdToast
		.getByRole("button", { name: "Close toast", exact: true })
		.press("Enter");
	await expect(createdToast).toHaveCount(0);
	await page
		.getByRole("button", { name: "Edit template", exact: true })
		.click();
	await page
		.getByRole("textbox", { name: "Name", exact: true })
		.fill("Unsaved name");
	await page.getByRole("button", { name: "Cancel", exact: true }).click();
	await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
	await page
		.getByRole("button", { name: "Edit template", exact: true })
		.click();
	const renamed = world.name("revised-tier");
	await page.getByRole("textbox", { name: "Name", exact: true }).fill(renamed);
	await page.getByRole("spinbutton", { name: "seats", exact: true }).fill("30");
	await page
		.getByRole("button", { name: "Save Template", exact: true })
		.click();
	await expect(
		page.getByRole("heading", { name: renamed, exact: true }),
	).toBeVisible();
	await page.reload();
	await page
		.getByRole("button", { name: "Edit template", exact: true })
		.click();
	await expect(
		page.getByRole("spinbutton", { name: "seats", exact: true }),
	).toHaveValue("30");
	await expect(
		page.getByRole("switch", { name: "sso", exact: true }),
	).toBeChecked();
	await page.getByRole("button", { name: "Cancel", exact: true }).click();
	await page.getByRole("link", { name: "All templates", exact: true }).click();
	await expect(
		page.getByRole("row").filter({ hasText: renamed }),
	).toContainText("6 fields");
});

// Covers: PRODUCT_LICENSE_TEMPLATES
test("template pagination and page size stay useful when a later-page search narrows the result", async ({
	page,
	world,
}) => {
	await declareSchema(world);
	const templates = [];
	for (let index = 0; index < 11; index += 1)
		templates.push(
			await createTemplate(world, `tier-${String(index).padStart(2, "0")}`),
		);
	await openLicensing(page, world, "/products/licensing/templates");
	await expect(
		page.getByRole("link", { name: `View ${templates[0].name}`, exact: true }),
	).toBeVisible();
	await page.getByRole("button", { name: "Next", exact: true }).click();
	await expect(
		page.getByRole("link", { name: `View ${templates[10].name}`, exact: true }),
	).toBeVisible();
	await expect(
		page.getByRole("link", { name: `View ${templates[0].name}`, exact: true }),
	).toHaveCount(0);
	await page
		.getByRole("textbox", { name: "Search templates", exact: true })
		.fill(templates[0].name);
	await expect(
		page.getByRole("link", { name: `View ${templates[0].name}`, exact: true }),
	).toBeVisible();
	await expect(
		page.getByRole("button", { name: "Previous", exact: true }),
	).toBeDisabled();
	await page
		.getByRole("textbox", { name: "Search templates", exact: true })
		.fill("");
	await chooseOption(
		page,
		page.getByRole("combobox", { name: "Rows per page", exact: true }),
		"Show 20",
	);
	await expect(page.getByRole("link", { name: /^View tier-/ })).toHaveCount(11);
	await expect(
		page.getByRole("button", { name: "Next", exact: true }),
	).toBeDisabled();
	await page
		.getByRole("columnheader", { name: "Name", exact: true })
		.getByRole("button", { name: "Name", exact: true })
		.click();
	await page
		.getByRole("columnheader", { name: "Name", exact: true })
		.getByRole("button", { name: "Name", exact: true })
		.click();
	await expect(
		page.getByRole("link", { name: /^View tier-/ }).first(),
	).toHaveAttribute("aria-label", `View ${templates[10].name}`);
});

// Covers: PRODUCT_LICENSE_TEMPLATES, PRODUCT_LICENSE_TEMPLATE_DETAIL
test("template search and status filters expose archived templates as read-only, including direct edit URLs", async ({
	page,
	world,
}) => {
	await declareSchema(world);
	const active = await createTemplate(world, "active-tier");
	const archived = await createTemplate(world, "retired-tier");
	await world.api.post(
		`${world.productPath}/licensing/templates/${archived.id}/archive`,
		{},
		200,
	);
	await openLicensing(page, world, "/products/licensing/templates");
	await page
		.getByRole("textbox", { name: "Search templates", exact: true })
		.fill(archived.name);
	const row = page.getByRole("row").filter({ hasText: archived.name });
	await expect(row).toContainText("ARCHIVED");
	await expect(
		page.getByRole("row").filter({ hasText: active.name }),
	).toHaveCount(0);
	await expect(
		row.getByRole("link", { name: `Edit ${archived.name}`, exact: true }),
	).toHaveCount(0);
	await page
		.getByRole("textbox", { name: "Search templates", exact: true })
		.fill("");
	await page
		.getByRole("button", { name: "Status", exact: true, expanded: false })
		.click();
	await page.getByRole("option", { name: "Archived", exact: true }).click();
	await page.keyboard.press("Escape");
	await expect(
		page.getByRole("row").filter({ hasText: active.name }),
	).toHaveCount(0);
	await page
		.getByRole("link", { name: `View ${archived.name}`, exact: true })
		.click();
	await expect(
		page.getByText("This template is archived and can no longer be edited.", {
			exact: true,
		}),
	).toBeVisible();
	await expect(
		page.getByRole("button", { name: "Edit template", exact: true }),
	).toHaveCount(0);
	await page.goto(`/products/licensing/templates/${archived.id}?edit=true`);
	await expect(
		page.getByRole("heading", { name: archived.name, exact: true }),
	).toBeVisible();
	await expect(
		page.getByRole("textbox", { name: "Name", exact: true }),
	).toHaveCount(0);
	await page.goto(`/products/licensing/templates/ltpl_${"0".repeat(27)}`);
	await expect(
		page.getByText("Template not found", { exact: true }),
	).toBeVisible();
	await expect(
		page.getByRole("link", { name: "All templates", exact: true }),
	).toBeVisible();
});

// Covers: PRODUCT_LICENSE_TEMPLATE_DETAIL, ORGANIZATION_LICENSE_DETAIL
test("editing a template propagates followed values while preserving a customer's adjusted fields", async ({
	page,
	world,
}) => {
	await declareSchema(world);
	const template = await createTemplate(world);
	const organization = await createOrganization(world, template);
	await adjustLicense(world, organization, { seats: 9 });
	await openLicensing(
		page,
		world,
		`/products/licensing/templates/${template.id}?edit=true`,
	);
	await page
		.getByRole("spinbutton", { name: "max_flows", exact: true })
		.fill("20");
	await page.getByRole("spinbutton", { name: "seats", exact: true }).fill("15");
	await page
		.getByRole("button", { name: "Save Template", exact: true })
		.click();
	await expect(
		page.getByRole("heading", { name: template.name, exact: true }),
	).toBeVisible();
	await expect
		.poll(async () => {
			const license = await world.api.get<OrganizationLicenseResponse>(
				licensePath(world, organization),
			);
			return {
				flows: license.values.max_flows,
				seats: license.values.seats,
				adjusted: license.adjusted_fields,
			};
		})
		.toEqual({ flows: 20, seats: 9, adjusted: ["seats"] });
	await page.goto(`/organizations/license/${organization.id}/values`);
	await expect(
		page.getByRole("spinbutton", { name: "max_flows", exact: true }),
	).toHaveValue("20");
	await expect(
		page.getByRole("spinbutton", { name: "seats", exact: true }),
	).toHaveValue("9");
	await expect(page.getByText("Tier grants 15", { exact: true })).toBeVisible();
	await page
		.getByRole("navigation", { name: "License sections", exact: true })
		.getByRole("link", { name: "Changes", exact: true })
		.click();
	await expect(
		page.getByText("Followed a template update", { exact: true }),
	).toBeVisible();
	const persisted = await world.api.get<LicenseTemplateResponse>(
		`${world.productPath}/licensing/templates/${template.id}`,
	);
	expect(persisted.values.seats).toBe(15);
});
