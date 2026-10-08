import {
	LicenseFieldType,
	type LicenseTemplateResponse,
	type OrganizationLicenseResponse,
	UsageShape,
} from "../../../src/client";
import { expect, test } from "../../support/fixtures";
import {
	adjustLicense,
	createOrganization,
	createTemplate,
	declareSchema,
	licensePath,
	openLicensing,
	values,
} from "./helpers";

// Covers: ORGANIZATION_LICENSE, ORGANIZATION_LICENSE_DETAIL
test("organization license search distinguishes matched, customized and unlicensed customers and missing records", async ({
	page,
	world,
}) => {
	await declareSchema(world);
	const template = await createTemplate(world);
	const matching = await createOrganization(world, template, "matching");
	const customized = await createOrganization(world, template, "customized");
	const unlicensed = await createOrganization(world, undefined, "unlicensed");
	await adjustLicense(world, customized, { max_flows: 25 });
	await openLicensing(page, world, "/organizations/license");
	await expect(
		page.getByRole("row").filter({ hasText: matching.name }),
	).toContainText("Matches");
	await expect(
		page.getByRole("row").filter({ hasText: customized.name }),
	).toContainText("1 custom field");
	await expect(
		page.getByRole("row").filter({ hasText: unlicensed.name }),
	).toContainText("No license");
	await page
		.getByRole("textbox", { name: "Search organizations", exact: true })
		.fill(unlicensed.name);
	await expect(
		page.getByRole("row").filter({ hasText: matching.name }),
	).toHaveCount(0);
	await page
		.getByRole("link", {
			name: `Open ${unlicensed.name}’s license`,
			exact: true,
		})
		.click();
	await expect(
		page.getByRole("heading", { name: unlicensed.name, exact: true }),
	).toBeVisible();
	await expect(
		page.getByText("This organization has no license yet.", { exact: false }),
	).toBeVisible();
	await expect(
		page.getByRole("navigation", { name: "License sections", exact: true }),
	).toBeVisible();
	await expect(
		page.getByRole("link", { name: "Billing", exact: true }),
	).toBeVisible();
	await page.goto("/organizations/license/missing-organization");
	await expect(
		page.getByText("No such organization", { exact: true }),
	).toBeVisible();
	await page
		.getByRole("link", { name: "All organizations", exact: true })
		.click();
	await page.getByRole("button", { name: "Tier", exact: true }).click();
	await page.getByRole("option", { name: "Unlicensed", exact: true }).click();
	await page.keyboard.press("Escape");
	await expect(
		page.getByRole("row").filter({ hasText: unlicensed.name }),
	).toBeVisible();
	await expect(
		page.getByRole("row").filter({ hasText: matching.name }),
	).toHaveCount(0);
});

// Covers: ORGANIZATION_LICENSE_DETAIL
test("usage shows all limit states, gauge and windowed reports, empty history and each retained time range", async ({
	page,
	world,
}) => {
	await declareSchema(world, [
		{
			name: "within",
			type: LicenseFieldType.LIMIT,
			usage_shape: UsageShape.GAUGE,
		},
		{ name: "at", type: LicenseFieldType.LIMIT, usage_shape: UsageShape.GAUGE },
		{
			name: "exceeded",
			type: LicenseFieldType.LIMIT,
			usage_shape: UsageShape.WINDOWED_COUNTER,
		},
		{
			name: "never",
			type: LicenseFieldType.LIMIT,
			usage_shape: UsageShape.GAUGE,
		},
	]);
	const template = await world.api.post<LicenseTemplateResponse>(
		`${world.productPath}/licensing/templates`,
		{
			name: world.name("usage-tier"),
			values: { within: 10, at: 10, exceeded: 10, never: 10 },
		},
	);
	const organization = await createOrganization(world, template);
	const productAPI = await world.productAPI();
	await productAPI.post(`${licensePath(world, organization)}/usage`, {
		key: "within",
		value: 5,
	});
	await productAPI.post(`${licensePath(world, organization)}/usage`, {
		key: "at",
		value: 10,
	});
	await productAPI.post(`${licensePath(world, organization)}/usage`, {
		key: "exceeded",
		value: 12,
		from: new Date(Date.now() - 60_000).toISOString(),
		to: new Date(Date.now() + 60_000).toISOString(),
	});
	await openLicensing(page, world, `/organizations/license/${organization.id}`);
	await expect(page).toHaveURL(
		new RegExp(`/organizations/license/${organization.id}/usage$`),
	);
	await expect(
		page
			.getByRole("navigation", { name: "License sections", exact: true })
			.getByRole("link", { name: "Usage", exact: true }),
	).toHaveAttribute("aria-current", "page");
	await expect(
		page.getByRole("button", { name: /^within Within limit/ }),
	).toContainText(/5\s*of 10/);
	await expect(
		page.getByRole("button", { name: /^at At limit/ }),
	).toContainText(/10\s*of 10/);
	await expect(
		page.getByRole("button", { name: /^exceeded Exceeded/ }),
	).toContainText(/12\s*of 10/);
	await expect(
		page.getByRole("button", { name: /^never Never reported/ }),
	).toContainText("limit 10");
	await expect(
		page.getByRole("img", { name: "12 of 10 used", exact: true }),
	).toBeVisible();
	await page.getByRole("button", { name: /^never Never reported/ }).click();
	await expect(
		page.getByRole("heading", { name: "History for never", exact: true }),
	).toBeVisible();
	await expect(
		page.getByText("Nothing reported in this range", { exact: true }),
	).toBeVisible();
	await expect(page).toHaveURL(
		(url) => url.searchParams.get("field") === "never",
	);
	await page.reload();
	await expect(
		page.getByRole("heading", { name: "History for never", exact: true }),
	).toBeVisible();
	await expect(
		page.getByText("Nothing reported in this range", { exact: true }),
	).toBeVisible();
	await page.getByRole("button", { name: /^within Within limit/ }).click();
	for (const range of [
		{ label: "Last 24h", granularity: "MINUTE" },
		{ label: "Last 7 days", granularity: "HOUR" },
		{ label: "Last 30 days", granularity: "DAY" },
		{ label: "Last 90 days", granularity: "DAY" },
	]) {
		const response = page.waitForResponse((candidate) => {
			const url = new URL(candidate.url());
			return (
				url.pathname.endsWith(
					`/organizations/${organization.id}/license/usage/series`,
				) &&
				url.searchParams.get("key") === "within" &&
				url.searchParams.get("granularity") === range.granularity &&
				candidate.request().method() === "GET"
			);
		});
		await page
			.getByRole("group", { name: "Time range", exact: true })
			.getByRole("button", { name: range.label, exact: true })
			.click();
		expect((await response).status()).toBe(200);
		await expect(
			page
				.getByRole("group", { name: "Time range", exact: true })
				.getByRole("button", { name: range.label, exact: true }),
		).toHaveAttribute("aria-pressed", "true");
		await expect(
			page.getByRole("heading", { name: "History for within", exact: true }),
		).toBeVisible();
	}
});

// Covers: ORGANIZATION_LICENSE, ORGANIZATION_LICENSE_DETAIL
test("customer value adjustments validate omissions and server rules, discard drafts, persist a minimal diff and update history", async ({
	page,
	world,
}) => {
	await declareSchema(world);
	const template = await createTemplate(world);
	const organization = await createOrganization(world, template);
	await openLicensing(
		page,
		world,
		`/organizations/license/${organization.id}/values`,
	);
	await expect(
		page
			.getByRole("navigation", { name: "License sections", exact: true })
			.getByRole("link", { name: "Values", exact: true }),
	).toHaveAttribute("aria-current", "page");
	const flows = page.getByRole("spinbutton", {
		name: "max_flows",
		exact: true,
	});
	await flows.fill("25");
	await expect(
		page.getByText("1 field changed", { exact: true }),
	).toBeVisible();
	await page.getByRole("button", { name: "Discard", exact: true }).click();
	await expect(flows).toHaveValue("10");
	await flows.fill("");
	await page
		.getByRole("button", { name: "Adjust this customer", exact: true })
		.click();
	await expect(flows).toHaveAttribute("aria-invalid", "true");
	await expect(
		page.getByText("This field must keep a value.", { exact: true }),
	).toBeVisible();
	await flows.fill("1001");
	const rejectedAdjustment = page.waitForResponse(
		(response) =>
			response.request().method() === "PATCH" &&
			response.url().endsWith(`/organizations/${organization.id}/license`),
	);
	await page
		.getByRole("button", { name: "Adjust this customer", exact: true })
		.click();
	expect((await rejectedAdjustment).status()).toBe(400);
	await expect(flows).toHaveAttribute("aria-invalid", "true");
	await expect(flows).toHaveValue("1001");
	expect(
		(
			await world.api.get<OrganizationLicenseResponse>(
				licensePath(world, organization),
			)
		).values,
	).toEqual(values);
	await flows.fill("25");
	const adjustment = page.waitForRequest(
		(request) =>
			request.method() === "PATCH" &&
			request.url().endsWith(`/organizations/${organization.id}/license`),
	);
	await page
		.getByRole("button", { name: "Adjust this customer", exact: true })
		.click();
	expect((await adjustment).postDataJSON()).toEqual({
		values: { max_flows: 25 },
	});
	await expect(
		page.getByRole("button", { name: "Adjust this customer", exact: true }),
	).toHaveCount(0);
	await expect(page.getByText("Tier grants 10", { exact: true })).toBeVisible();
	const license = await world.api.get<OrganizationLicenseResponse>(
		licensePath(world, organization),
	);
	expect(license.values).toEqual({ ...values, max_flows: 25 });
	expect(license.adjusted_fields).toEqual(["max_flows"]);
	await page
		.getByRole("navigation", { name: "License sections", exact: true })
		.getByRole("link", { name: "Changes", exact: true })
		.click();
	await expect(
		page.getByText("1 field customized", { exact: true }),
	).toBeVisible();
	await expect(
		page
			.getByRole("listitem")
			.filter({ hasText: "Instantiated" })
			.getByText("Instantiated", { exact: true }),
	).toBeVisible();
	await page
		.getByRole("navigation", { name: "License sections", exact: true })
		.getByRole("link", { name: "Usage", exact: true })
		.click();
	await expect(page.getByRole("button", { name: /^max_flows/ })).toContainText(
		"Custom limit",
	);
	await page
		.getByRole("link", { name: "All organizations", exact: true })
		.click();
	await expect(
		page.getByRole("row").filter({ hasText: organization.name }),
	).toContainText("1 custom field");
	await page.goto(`/organizations/license/${organization.id}/values`);
	await expect(flows).toHaveValue("25");
});

// Covers: ORGANIZATION_LICENSE_DETAIL
test("unsaved customer adjustments require a choice before tab navigation and leaving discards the edit", async ({
	page,
	world,
}) => {
	await declareSchema(world);
	const template = await createTemplate(world);
	const organization = await createOrganization(world, template);
	await openLicensing(
		page,
		world,
		`/organizations/license/${organization.id}/values`,
	);
	await page
		.getByRole("spinbutton", { name: "max_flows", exact: true })
		.fill("42");
	await page
		.getByRole("navigation", { name: "License sections", exact: true })
		.getByRole("link", { name: "Usage", exact: true })
		.click();
	const guard = page.getByRole("alertdialog", {
		name: "Leave without adjusting?",
		exact: true,
	});
	await expect(guard).toContainText("1 field");
	await guard.getByRole("button", { name: "Stay", exact: true }).click();
	await expect(
		page.getByRole("spinbutton", { name: "max_flows", exact: true }),
	).toHaveValue("42");
	await page
		.getByRole("navigation", { name: "License sections", exact: true })
		.getByRole("link", { name: "Changes", exact: true })
		.click();
	await guard
		.getByRole("button", { name: "Discard and leave", exact: true })
		.click();
	await expect(
		page
			.getByRole("navigation", { name: "License sections", exact: true })
			.getByRole("link", { name: "Changes", exact: true }),
	).toHaveAttribute("aria-current", "page");
	await page
		.getByRole("navigation", { name: "License sections", exact: true })
		.getByRole("link", { name: "Values", exact: true })
		.click();
	await expect(
		page.getByRole("spinbutton", { name: "max_flows", exact: true }),
	).toHaveValue("10");
	await page
		.getByRole("spinbutton", { name: "max_flows", exact: true })
		.fill("42");
	await page
		.getByRole("spinbutton", { name: "max_flows", exact: true })
		.fill("10");
	await expect(
		page.getByRole("button", { name: "Adjust this customer", exact: true }),
	).toHaveCount(0);
	await page
		.getByRole("navigation", { name: "License sections", exact: true })
		.getByRole("link", { name: "Usage", exact: true })
		.click();
	await expect(guard).toHaveCount(0);
	await expect(
		page
			.getByRole("navigation", { name: "License sections", exact: true })
			.getByRole("link", { name: "Usage", exact: true }),
	).toHaveAttribute("aria-current", "page");
	await world.api.post(
		`${world.productPath}/licensing/templates/${template.id}/archive`,
		{},
		200,
	);
	await page.reload();
	await expect(page.getByText("Tier withdrawn", { exact: true })).toBeVisible();
});

// Covers: ORGANIZATION_LICENSE_DETAIL
test("license changes can load history beyond the initial fifty records", async ({
	page,
	world,
}) => {
	await declareSchema(world);
	const organization = await createOrganization(
		world,
		await createTemplate(world),
	);
	for (let index = 0; index < 51; index += 1)
		await adjustLicense(world, organization, { max_flows: index + 20 });
	await openLicensing(
		page,
		world,
		`/organizations/license/${organization.id}/changes`,
	);
	await expect(
		page.getByText("Showing 50 of 52", { exact: true }),
	).toBeVisible();
	await page
		.getByRole("button", { name: "Load older changes", exact: true })
		.click();
	await expect(
		page
			.getByRole("listitem")
			.filter({ hasText: "Instantiated" })
			.getByText("Instantiated", { exact: true }),
	).toBeVisible();
	await expect(
		page.getByRole("button", { name: "Load older changes", exact: true }),
	).toHaveCount(0);
});

// Covers: ORGANIZATION_LICENSE_DETAIL
test("a license with only metadata fields has an explicit no-usage state and editable values", async ({
	page,
	world,
}) => {
	await declareSchema(world, [
		{ name: "seats", type: LicenseFieldType.NUMBER },
	]);
	const template = await world.api.post<LicenseTemplateResponse>(
		`${world.productPath}/licensing/templates`,
		{ name: world.name("metadata-tier"), values: { seats: 5 } },
	);
	const organization = await createOrganization(world, template);
	await openLicensing(
		page,
		world,
		`/organizations/license/${organization.id}/usage`,
	);
	await expect(
		page.getByText(
			"This product’s license schema declares no limit fields, so there is no usage to measure.",
			{ exact: true },
		),
	).toBeVisible();
	await page
		.getByRole("navigation", { name: "License sections", exact: true })
		.getByRole("link", { name: "Values", exact: true })
		.click();
	await expect(
		page.getByRole("spinbutton", { name: "seats", exact: true }),
	).toHaveValue("5");
});
