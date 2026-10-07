import { expect, test } from "../../support/fixtures";
import { captureReviewCheckpoint } from "../../support/review";
import {
	createOrganization,
	openWorkflows,
	workflowIdFrom,
	workflowRuns,
	workspaceNames,
} from "./workflow-helpers";

// Covers: PRODUCT_WORKFLOWS, PRODUCT_WORKFLOW_NEW, PRODUCT_WORKFLOW_DETAIL
test("a recipe becomes a workflow that dry-runs, saves and reacts to the next organization", async ({
	page,
	world,
}, testInfo) => {
	const existing = await createOrganization(world, world.name("existing-org"));
	await openWorkflows(page, world);
	await expect(
		page.getByText(/A workflow reacts to a product event/),
	).toBeVisible();
	await page
		.getByRole("button", { name: /Give every new organization a workspace/ })
		.click();

	await expect(page).toHaveURL(
		/\/products\/workflows\/new\?recipe=default-workspace$/,
	);
	await expect(page.getByLabel("Product event", { exact: true })).toHaveValue(
		"organization.created",
	);
	await expect(
		page.getByRole("article", { name: "Step 1: Create workspace" }),
	).toBeVisible();
	await expect(
		page.getByRole("article", { name: "Step 2: Update organization" }),
	).toBeVisible();

	await page
		.getByLabel("event.data.organization_id", { exact: true })
		.fill(existing.id);
	await page.getByRole("button", { name: "Dry run", exact: true }).click();
	const dryRun = page.getByRole("region", { name: "Run result", exact: true });
	await expect(dryRun).toContainText("Dry run");
	await expect(dryRun.getByText("Succeeded", { exact: true })).toBeVisible();
	await expect(dryRun.getByText("Simulated", { exact: true })).toHaveCount(2);
	await captureReviewCheckpoint(page, testInfo, "dry-run-result");
	expect(await workspaceNames(world, existing.id)).toEqual([]);

	await page
		.getByRole("button", { name: "Create workflow", exact: true })
		.click();
	await expect(
		page.getByRole("heading", {
			name: "Default workspace for new organizations",
			exact: true,
		}),
	).toBeVisible();
	const workflowId = workflowIdFrom(page);

	const created = await createOrganization(world, world.name("new-org"));
	await expect
		.poll(async () => (await workflowRuns(world, workflowId)).count, {
			message: "the saved workflow runs once for the new organization",
		})
		.toBe(1);
	expect(await workspaceNames(world, created.id)).toEqual(["General"]);
	expect(await workspaceNames(world, existing.id)).toEqual([]);

	await page.getByRole("tab", { name: /Runs/ }).click();
	const runs = page.getByRole("list", { name: "Runs" });
	await expect(runs.getByText("Succeeded", { exact: true })).toBeVisible();
	await expect(page.getByText(created.id, { exact: true })).toBeVisible();
	await captureReviewCheckpoint(page, testInfo, "run-history");

	await page.goto("/products/workflows");
	await expect(
		page
			.getByRole("list", { name: "Your workflows", exact: true })
			.getByRole("button", { name: /Default workspace for new organizations/ }),
	).toContainText("Succeeded");
});

// Covers: PRODUCT_WORKFLOW_NEW, PRODUCT_WORKFLOW_DETAIL
test("a workflow built from scratch is validated, edited with step conditions, persisted and deleted", async ({
	page,
	world,
}, testInfo) => {
	await openWorkflows(page, world);
	await page
		.getByRole("button", { name: "New workflow", exact: true })
		.first()
		.click();
	await expect(
		page.getByRole("heading", { name: "New workflow", exact: true }),
	).toBeVisible();

	await page
		.getByRole("button", { name: "Create workflow", exact: true })
		.click();
	await expect(page.getByText("Not saved", { exact: true })).toBeVisible();

	const name = world.name("domain-join");
	await page.getByLabel("Name", { exact: true }).fill(name);
	await page
		.getByLabel("Product event", { exact: true })
		.selectOption("product_user.created");
	await page.getByRole("button", { name: "Add a step", exact: true }).click();
	await page
		.getByRole("menuitem", { name: "Read product user", exact: true })
		.click();
	await expect(
		page.getByRole("textbox", { name: "Product user *", exact: true }),
	).toHaveValue("{{event.data.product_user_id}}");

	await page.getByRole("button", { name: "Add a step", exact: true }).click();
	await page
		.getByRole("menuitem", { name: "Invite to organization", exact: true })
		.click();
	const invite = page.getByRole("article", {
		name: "Step 2: Invite to organization",
	});
	await invite
		.getByRole("button", { name: "Insert a value into Email" })
		.click();
	await page.getByRole("menuitem", { name: "email", exact: true }).click();
	await expect(
		invite.getByRole("textbox", { name: "Email *", exact: true }),
	).toHaveValue("{{steps.product_user.email}}");
	await page
		.getByRole("button", { name: "Create workflow", exact: true })
		.click();
	await expect(
		page.getByText(
			/Parameter "organization_id" of action invitation.create is required/,
		),
	).toBeVisible();
	await captureReviewCheckpoint(page, testInfo, "server-validation");

	await invite
		.getByRole("textbox", { name: "Organization *", exact: true })
		.fill("org_example");
	await invite
		.getByRole("textbox", { name: "Role *", exact: true })
		.fill("role_example");
	await invite.getByRole("button", { name: /Only run this step when/ }).click();
	await invite
		.getByRole("button", { name: "Add condition", exact: true })
		.click();
	await invite
		.getByRole("combobox", { name: "Value to test", exact: true })
		.fill("steps.product_user.email_domain");
	await invite
		.getByRole("textbox", { name: "Compared with", exact: true })
		.fill("acme.com");
	await page
		.getByRole("switch", { name: /Enabled: runs on every matching event/ })
		.click();
	await page
		.getByRole("button", { name: "Create workflow", exact: true })
		.click();
	await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
	const workflowId = workflowIdFrom(page);

	await page.reload();
	await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
	const saved = page.getByRole("article", {
		name: "Step 2: Invite to organization",
	});
	await expect(
		saved.getByRole("textbox", { name: "Compared with", exact: true }),
	).toHaveValue("acme.com");
	await expect(
		page.getByRole("switch", { name: /Disabled: runs only by hand/ }),
	).not.toBeChecked();
	const persisted = await world.api.get<{
		enabled: boolean;
		definition: { steps: unknown[] };
	}>(`${world.productPath}/workflows/${workflowId}`);
	expect(persisted.enabled).toBe(false);
	expect(persisted.definition.steps).toHaveLength(2);
	await captureReviewCheckpoint(page, testInfo, "saved-workflow");

	await page.getByRole("button", { name: "Delete", exact: true }).click();
	await page
		.getByRole("alertdialog")
		.getByRole("button", { name: "Delete workflow", exact: true })
		.click();
	await expect(page).toHaveURL(/\/products\/workflows$/);
	await expect(
		page.getByRole("button", { name: new RegExp(name) }),
	).toHaveCount(0);
	await world.api.get(`${world.productPath}/workflows/${workflowId}`, 404);
});
