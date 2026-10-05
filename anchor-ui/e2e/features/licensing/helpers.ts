import type { Locator, Page } from "playwright/test";
import {
	type LicenseFieldDeclaration,
	LicenseFieldType,
	type LicenseSchemaResponse,
	type LicenseTemplateResponse,
	type LicenseTemplateValues,
	type OrganizationLicenseResponse,
	type ProductOrganizationResponse,
	UsageShape,
} from "../../../src/client";
import { type World, expect } from "../../support/fixtures";
import { selectProduct } from "../../support/ui";

export const fields: LicenseFieldDeclaration[] = [
	{
		name: "max_flows",
		type: LicenseFieldType.LIMIT,
		usage_shape: UsageShape.GAUGE,
		rules: { min: 0, max: 1000 },
	},
	{
		name: "monthly_runs",
		type: LicenseFieldType.LIMIT,
		usage_shape: UsageShape.WINDOWED_COUNTER,
		rules: { min: 0, max: 100000 },
	},
	{
		name: "seats",
		type: LicenseFieldType.NUMBER,
		rules: { min: 1, max: 1000 },
	},
	{ name: "sso", type: LicenseFieldType.BOOLEAN },
	{
		name: "tier",
		type: LicenseFieldType.ENUM,
		rules: { values: ["free", "pro", "enterprise"] },
	},
	{
		name: "webhook_url",
		type: LicenseFieldType.STRING,
		rules: { pattern: "^https://.*$", min_length: 1, max_length: 2048 },
	},
];

export const values: LicenseTemplateValues = {
	max_flows: 10,
	monthly_runs: 1000,
	seats: 5,
	sso: false,
	tier: "free",
	webhook_url: "https://example.test",
};

export async function declareSchema(world: World, declarations = fields) {
	return world.api.post<LicenseSchemaResponse>(
		`${world.productPath}/licensing/schema`,
		{ description: "Local browser license contract", fields: declarations },
	);
}

export async function createTemplate(
	world: World,
	prefix = "tier",
	overrides: LicenseTemplateValues = {},
) {
	return world.api.post<LicenseTemplateResponse>(
		`${world.productPath}/licensing/templates`,
		{
			name: world.name(prefix),
			description: "API prerequisite",
			values: { ...values, ...overrides },
		},
	);
}

export async function createOrganization(
	world: World,
	template?: LicenseTemplateResponse,
	prefix = "customer",
) {
	return (await world.productAPI()).post<ProductOrganizationResponse>(
		`${world.productPath}/organizations`,
		{
			name: world.name(prefix),
			...(template ? { license: { template_id: template.id } } : {}),
		},
	);
}

export function licensePath(
	world: World,
	organization: ProductOrganizationResponse,
) {
	return `${world.productPath}/organizations/${organization.id}/license`;
}

export async function adjustLicense(
	world: World,
	organization: ProductOrganizationResponse,
	changedValues: LicenseTemplateValues,
) {
	const response = await world.api.context.patch(
		licensePath(world, organization),
		{ data: { values: changedValues } },
	);
	expect(response.status(), await response.text()).toBe(200);
	return response.json() as Promise<OrganizationLicenseResponse>;
}

export async function openLicensing(page: Page, world: World, path: string) {
	await selectProduct(page, world.product);
	await page.goto(path);
}

export async function chooseOption(page: Page, control: Locator, name: string) {
	await control.click();
	await page.getByRole("option", { name, exact: true }).click();
}

export async function fillTemplateValues(page: Page) {
	await page
		.getByRole("spinbutton", { name: "max_flows", exact: true })
		.fill("25");
	await page
		.getByRole("spinbutton", { name: "monthly_runs", exact: true })
		.fill("2500");
	await page.getByRole("spinbutton", { name: "seats", exact: true }).fill("15");
	await page.getByRole("switch", { name: "sso", exact: true }).click();
	await chooseOption(
		page,
		page.getByRole("combobox", { name: "tier", exact: true }),
		"pro",
	);
	await page
		.getByRole("textbox", { name: "webhook_url", exact: true })
		.fill("https://pro.example.test");
}
