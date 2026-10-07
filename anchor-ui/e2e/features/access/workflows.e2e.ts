import { expect, test } from "../../support/fixtures";
import { captureReviewCheckpoint } from "../../support/review";
import {
	createOrganization,
	createWorkflowViaAPI,
	latestRun,
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
	await expect(page.getByLabel("Event", { exact: true })).toHaveValue(
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
	await dryRun.scrollIntoViewIfNeeded();
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
		.poll(async () => (await latestRun(world, workflowId))?.status, {
			message: "the saved workflow finishes its run for the new organization",
		})
		.toBe("succeeded");
	expect((await workflowRuns(world, workflowId)).count).toBe(1);
	expect(await workspaceNames(world, created.id)).toEqual(["General"]);
	expect(await workspaceNames(world, existing.id)).toEqual([]);

	await page.getByRole("tab", { name: /Runs/ }).click();
	const runs = page.getByRole("list", { name: "Runs" });
	await expect(runs.getByText("Succeeded", { exact: true })).toBeVisible();
	await expect(
		page.getByRole("tabpanel").getByText(created.id, { exact: true }),
	).toBeVisible();
	await captureReviewCheckpoint(page, testInfo, "run-history");

	await page.goto("/products/workflows");
	await expect(
		page
			.getByRole("list", { name: "Your workflows", exact: true })
			.getByRole("button", {
				name: /Default workspace for new organizations/,
			}),
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
	await expect(
		page.getByText("Give the workflow a name of at least 2 characters."),
	).toBeVisible();
	await expect(
		page.getByText("Pick the event that starts this workflow."),
	).toBeVisible();

	const name = world.name("domain-join");
	await page.getByLabel("Name", { exact: true }).fill(name);
	await page
		.getByLabel("Event", { exact: true })
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
	const email = invite.getByRole("textbox", { name: "Email *", exact: true });
	await expect(email).toHaveValue("{{steps.product_user.email}}");
	await page
		.getByRole("button", { name: "Create workflow", exact: true })
		.click();
	await expect(
		invite.getByText(
			/^Parameter "organization_id" of action invitation.create is required/,
		),
	).toBeVisible();
	await expect(
		page.getByText(
			/^Step 2 · Organization: Parameter "organization_id" of action invitation.create is required/,
		),
	).toBeVisible();
	await invite.scrollIntoViewIfNeeded();
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
		.getByRole("combobox", { name: "Step 2 condition 1: value to test" })
		.fill("steps.product_user.email_domain");
	await invite
		.getByRole("combobox", { name: "Step 2 condition 1: comparison" })
		.selectOption("ends_with");
	await invite
		.getByRole("textbox", { name: "Step 2 condition 1: compared with" })
		.fill("acme.com");
	const enabled = page.getByRole("switch", { name: "Enabled", exact: true });
	await enabled.click();
	await expect(enabled).not.toBeChecked();
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
		saved.getByRole("combobox", { name: "Step 2 condition 1: value to test" }),
	).toHaveValue("steps.product_user.email_domain");
	await expect(
		saved.getByRole("combobox", { name: "Step 2 condition 1: comparison" }),
	).toHaveValue("ends_with");
	await expect(
		saved.getByRole("textbox", { name: "Step 2 condition 1: compared with" }),
	).toHaveValue("acme.com");
	await expect(
		saved.getByRole("textbox", { name: "Email *", exact: true }),
	).toHaveValue("{{steps.product_user.email}}");
	await expect(
		page.getByRole("switch", { name: "Enabled", exact: true }),
	).not.toBeChecked();
	const persisted = await world.api.get<{
		enabled: boolean;
		definition: { steps: unknown[] };
	}>(`${world.productPath}/workflows/${workflowId}`);
	expect(persisted.enabled).toBe(false);
	expect(persisted.definition.steps).toHaveLength(2);
	await saved.scrollIntoViewIfNeeded();
	await captureReviewCheckpoint(page, testInfo, "saved-workflow");

	await page.getByRole("button", { name: "Delete", exact: true }).click();
	await page
		.getByRole("alertdialog")
		.getByRole("button", { name: "Delete workflow", exact: true })
		.click();
	await expect(page).toHaveURL(/\/products\/workflows$/);
	await expect(
		page.getByRole("heading", { name: "Your workflows", exact: true }),
	).toBeVisible();
	await expect(
		page.getByText(/A workflow reacts to a product event/),
	).toBeVisible();
	await expect(page.getByText(name, { exact: true })).toHaveCount(0);
	await world.api.get(`${world.productPath}/workflows/${workflowId}`, 404);
});

// Covers: PRODUCT_WORKFLOWS, PRODUCT_WORKFLOW_NEW, PRODUCT_WORKFLOW_DETAIL
test("workflows chain through a custom event, and a loop is flagged then refused", async ({
	page,
	world,
}, testInfo) => {
	const handoff = await createWorkflowViaAPI(world, {
		name: world.name("handoff"),
		enabled: true,
		trigger_event_type: "organization.created",
		definition: {
			conditions: [],
			steps: [
				{
					id: "handoff",
					action: "workflow.emit",
					params: {
						event: "onboarding.started",
						data: '{"organization_id": "{{event.data.organization_id}}", "plan": "pro"}',
					},
				},
			],
		},
	});
	await openWorkflows(page, world);
	await page
		.getByRole("button", { name: "New workflow", exact: true })
		.first()
		.click();

	const name = world.name("follow-up");
	await page.getByLabel("Name", { exact: true }).fill(name);
	await page
		.getByLabel("Event", { exact: true })
		.selectOption("custom.onboarding.started");
	await expect(
		page.getByText(`Started after “${handoff.name}”.`, { exact: true }),
	).toBeVisible();
	await expect(
		page
			.getByRole("region", { name: "onboarding.started", exact: true })
			.getByText("event.data.organization_id", { exact: true }),
	).toBeVisible();
	await page.getByRole("button", { name: "Add a step", exact: true }).click();
	await page
		.getByRole("menuitem", { name: "Create workspace", exact: true })
		.click();
	const workspace = page.getByRole("article", {
		name: "Step 1: Create workspace",
	});
	await expect(
		workspace.getByRole("textbox", { name: "Organization *", exact: true }),
	).toHaveValue("{{event.data.organization_id}}");
	await workspace
		.getByRole("textbox", { name: "Name *", exact: true })
		.fill("Kickoff {{event.data.plan}}");
	await page
		.getByRole("button", { name: "Create workflow", exact: true })
		.click();
	await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
	const followUpId = workflowIdFrom(page);

	const organization = await createOrganization(
		world,
		world.name("chained-org"),
	);
	await expect
		.poll(async () => (await latestRun(world, followUpId))?.status, {
			message: "the custom event starts the follow-up workflow",
		})
		.toBe("succeeded");
	expect(await workspaceNames(world, organization.id)).toEqual(["Kickoff pro"]);

	await page.getByRole("tab", { name: "Build", exact: true }).click();
	await page.getByRole("button", { name: "Add a step", exact: true }).click();
	await page
		.getByRole("menuitem", { name: "Start other workflows", exact: true })
		.click();
	const emit = page.getByRole("article", {
		name: "Step 2: Start other workflows",
	});
	await emit
		.getByRole("combobox", { name: "Custom event *", exact: true })
		.fill("onboarding.started");
	await expect(
		page.getByText("This would loop", { exact: true }),
	).toBeVisible();
	await expect(
		emit.getByRole("list", { name: "Events step 2 emits" }),
	).toContainText("custom.onboarding.started");
	await page.getByRole("button", { name: "Save changes", exact: true }).click();
	await expect(page.getByText("Not saved", { exact: true })).toBeVisible();
	await expect(
		page.getByText(/Saving this would let the workflow start itself again/),
	).toBeVisible();
	await captureReviewCheckpoint(page, testInfo, "loop-refused");
	const unchanged = await world.api.get<{ definition: { steps: unknown[] } }>(
		`${world.productPath}/workflows/${followUpId}`,
	);
	expect(unchanged.definition.steps).toHaveLength(1);

	await page.goto("/products/workflows");
	const links = page.getByRole("list", { name: "Workflow links" });
	await expect(links).toContainText(handoff.name);
	await expect(links).toContainText("custom.onboarding.started");
	await expect(links).toContainText(name);
	await links.scrollIntoViewIfNeeded();
	await captureReviewCheckpoint(page, testInfo, "workflow-links");
});
