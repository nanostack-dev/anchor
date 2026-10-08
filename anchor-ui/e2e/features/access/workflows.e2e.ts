import type { Locator, Page } from "playwright/test";
import { expect, test } from "../../support/fixtures";
import { captureReviewCheckpoint } from "../../support/review";
import {
	configureEventEndpoint,
	createOrganization,
	createWorkflowViaAPI,
	latestRun,
	openWorkflows,
	workflowIdFrom,
	workflowRuns,
	workspaceNames,
	writingBackend,
} from "./workflow-helpers";

async function backToFlow(page: Page) {
	await page
		.getByRole("button", { name: /^(Close|Back to flow)$/ })
		.first()
		.click();
	await expect(
		page.getByRole("region", { name: "Workflow settings" }),
	).toBeVisible();
	await expect(
		page.getByRole("button", { name: "Add a step", exact: true }),
	).toBeVisible();
}

/** The palette starts open only where it floats beside the steps. */
async function openStepPalette(page: Page) {
	const toggle = page.getByRole("button", { name: "Steps", exact: true });
	if ((await toggle.getAttribute("aria-expanded")) !== "true")
		await toggle.click();
	await expect(toggle).toHaveAttribute("aria-expanded", "true");
	await expect(
		page.getByRole("region", { name: "Steps to add", exact: true }),
	).toBeVisible();
}

/**
 * The box of a canvas element once the canvas has stopped moving. A hover
 * would scroll the canvas wrapper, which React Flow scrolls straight back, and
 * leave the mouse on stale coordinates.
 */
async function settledBox(locator: Locator) {
	let previous = "";
	await expect
		.poll(
			async () => {
				const box = JSON.stringify(await locator.boundingBox());
				const settled = box === previous && box !== "null";
				previous = box;
				return settled;
			},
			{ message: "the canvas stops moving", intervals: [100] },
		)
		.toBe(true);
	const box = await locator.boundingBox();
	if (!box) throw new Error("the canvas element has no box");
	return box;
}

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
	await expect(
		page.getByRole("button", { name: "Trigger: Organization created" }),
	).toBeVisible();
	const firstStep = page.getByRole("button", {
		name: "Step 1: Create General workspace",
	});
	const secondStep = page.getByRole("button", {
		name: "Step 2: Remember it on the organization",
	});
	await expect(firstStep).toBeVisible();
	await expect(secondStep).toBeVisible();

	await page
		.getByLabel("event.data.organization_id", { exact: true })
		.fill(existing.id);
	await page.getByRole("button", { name: "Dry run", exact: true }).click();
	const dryRun = page.getByRole("region", { name: "Run result", exact: true });
	await expect(dryRun).toContainText("Dry run");
	await expect(dryRun.getByText("Succeeded", { exact: true })).toBeVisible();
	await expect(dryRun.getByText("Simulated", { exact: true })).toHaveCount(2);
	await expect(firstStep).toContainText("Simulated");
	await expect(secondStep).toContainText("Simulated");
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

	const name = world.name("domain-join");
	await page.getByLabel("Name", { exact: true }).fill(name);
	await page
		.getByRole("button", { name: "Create workflow", exact: true })
		.click();
	await expect(
		page.getByText("Pick the event that starts this workflow."),
	).toBeVisible();
	await page
		.getByLabel("Event", { exact: true })
		.selectOption("product_user.created");
	await backToFlow(page);
	await expect(
		page.getByRole("button", { name: "Trigger: Product user created" }),
	).toBeInViewport();
	await page.getByRole("button", { name: "Add a step", exact: true }).click();
	await page
		.getByRole("menuitem", { name: "Read product user", exact: true })
		.click();
	await expect(
		page.getByRole("button", {
			name: "Product user *: product_user_id from Event. Edit",
		}),
	).toBeVisible();

	await backToFlow(page);
	await page.getByRole("button", { name: "Add a step", exact: true }).click();
	await page
		.getByRole("menuitem", { name: "Invite to organization", exact: true })
		.click();
	const invite = page.getByRole("article", {
		name: "Step 2: Invite to organization",
	});
	const emailInput = invite.getByRole("textbox", {
		name: "Email *",
		exact: true,
	});
	const emailChip = invite
		.getByRole("region", { name: "Data you can use" })
		.getByRole("button", { name: "email, Email, from Read product user" });
	if ((testInfo.project.use.viewport?.width ?? 1280) < 640) {
		// A phone inserts a field by tapping the input, then the chip.
		await emailInput.click();
		await emailChip.click();
	} else {
		await emailChip.dragTo(emailInput);
	}
	await expect(
		invite.getByRole("button", {
			name: "Email *: email from Read product user. Edit",
		}),
	).toBeVisible();
	await page
		.getByRole("button", { name: "Create workflow", exact: true })
		.click();
	await expect(
		invite.getByText("Organization is required for “Invite to organization”.", {
			exact: true,
		}),
	).toBeVisible();
	await expect(
		invite.getByRole("textbox", { name: "Organization *", exact: true }),
	).toHaveAttribute("aria-invalid", "true");
	await expect(
		invite.getByText("Role is required for “Invite to organization”.", {
			exact: true,
		}),
	).toBeVisible();
	await invite.scrollIntoViewIfNeeded();
	await captureReviewCheckpoint(page, testInfo, "server-validation");

	await invite
		.getByRole("textbox", { name: "Organization *", exact: true })
		.fill("org_example");
	await expect(
		invite.getByRole("textbox", { name: "Organization *", exact: true }),
	).not.toHaveAttribute("aria-invalid", "true");
	await expect(
		invite.getByText("Organization is required for “Invite to organization”.", {
			exact: true,
		}),
	).toHaveCount(0);
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
	await page
		.getByRole("button", { name: "Step 2: Invite to organization" })
		.click();
	const saved = page.getByRole("article", {
		name: "Step 2: Invite to organization",
	});
	await expect(saved).toBeInViewport({ ratio: 0.2 });
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
		saved.getByRole("button", {
			name: "Email *: email from Read product user. Edit",
		}),
	).toBeVisible();
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

	await backToFlow(page);
	await page
		.getByRole("button", { name: "Insert a step before step 2" })
		.click();
	await page
		.getByRole("menuitem", { name: "Read product user", exact: true })
		.click();
	await expect(
		page.getByRole("article", { name: "Step 2: Read product user" }),
	).toBeVisible();
	await backToFlow(page);
	const inserted = page.getByRole("button", {
		name: "Step 2: Read product user",
	});
	await expect(inserted).toBeInViewport();
	await expect(inserted).toBeFocused();
	await expect(
		page.getByRole("button", { name: "Step 3: Invite to organization" }),
	).toBeAttached();

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

// Covers: PRODUCT_WORKFLOW_NEW
test("a step dragged from the palette lands where it is dropped, and steps reorder by handle and keyboard", async ({
	page,
	world,
}, testInfo) => {
	await openWorkflows(page, world);
	await page
		.getByRole("button", { name: /Give every new organization a workspace/ })
		.click();
	const canvas = page.getByRole("application", { name: "Workflow canvas" });
	const step = (position: number, name: string) =>
		canvas.getByRole("button", {
			name: `Step ${position}: ${name}`,
			exact: true,
		});
	await expect(step(1, "Create General workspace")).toBeVisible();
	await expect(step(2, "Remember it on the organization")).toBeVisible();

	await openStepPalette(page);
	await page
		.getByRole("button", { name: "Add step: Read product user", exact: true })
		.dragTo(
			canvas.getByRole("button", {
				name: "Insert a step before step 1",
				exact: true,
			}),
		);
	await expect(
		page.getByRole("article", { name: "Step 1: Read product user" }),
	).toBeVisible();
	await backToFlow(page);
	await expect(step(1, "Read product user")).toBeVisible();
	await expect(step(2, "Create General workspace")).toBeAttached();
	await expect(step(3, "Remember it on the organization")).toBeAttached();
	await captureReviewCheckpoint(page, testInfo, "step-dropped-from-palette");

	const grip = await settledBox(
		canvas.getByRole("button", { name: "Reorder step 1", exact: true }),
	);
	const next = await settledBox(step(2, "Create General workspace"));
	const x = grip.x + grip.width / 2;
	const y = grip.y + grip.height / 2;
	await page.mouse.move(x, y);
	await page.mouse.down();
	await page.mouse.move(x, y + 4);
	await page.mouse.move(x, next.y + next.height * 0.75, { steps: 10 });
	await page.mouse.up();
	await expect(step(1, "Create General workspace")).toBeVisible();
	await expect(step(2, "Read product user")).toBeVisible();
	await expect(step(3, "Remember it on the organization")).toBeAttached();
	await captureReviewCheckpoint(page, testInfo, "step-reordered-by-handle");

	await canvas
		.getByRole("button", { name: "Reorder step 2", exact: true })
		.press("ArrowUp");
	await expect(step(1, "Read product user")).toBeVisible();
	await expect(step(2, "Create General workspace")).toBeAttached();
	await expect(
		canvas.getByRole("button", { name: "Reorder step 1", exact: true }),
	).toBeFocused();
	await expect(
		page.getByText("Step moved to position 1", { exact: true }),
	).toBeAttached();
	await captureReviewCheckpoint(page, testInfo, "step-moved-by-keyboard");
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
						data_types: '{"organization_id": "organization", "plan": "text"}',
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
	await page.getByRole("button", { name: "Trigger: Pick a trigger" }).click();
	await page
		.getByLabel("Event", { exact: true })
		.selectOption("custom.onboarding.started");
	const triggerSettings = page.getByRole("region", {
		name: "Trigger settings",
		exact: true,
	});
	await expect(
		triggerSettings.getByText(`Started after “${handoff.name}”.`, {
			exact: true,
		}),
	).toBeVisible();
	const carried = triggerSettings.getByRole("list", {
		name: "Fields the event carries",
	});
	await expect(carried.getByRole("listitem").first()).toHaveText(
		"event.data.organization_idOrganization ID",
	);
	await backToFlow(page);
	await expect(
		page.getByRole("button", { name: `Open “${handoff.name}”` }),
	).toBeVisible();
	await page.getByRole("button", { name: "Add a step", exact: true }).click();
	await page
		.getByRole("menuitem", { name: "Create workspace", exact: true })
		.click();
	const workspace = page.getByRole("article", {
		name: "Step 1: Create workspace",
	});
	await expect(
		workspace.getByRole("button", {
			name: "Organization *: organization_id from Event. Edit",
		}),
	).toBeVisible();
	await workspace
		.getByRole("textbox", { name: "Name *", exact: true })
		.fill("Kickoff {{event.data.plan}}");
	await page
		.getByRole("button", { name: "Create workflow", exact: true })
		.click();
	await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
	const followUpId = workflowIdFrom(page);
	const savedToast = page.getByRole("dialog", {
		name: `Workflow “${name}” saved.`,
		exact: true,
	});
	await savedToast.hover();
	await savedToast
		.getByRole("button", { name: "Close toast", exact: true })
		.press("Enter");
	await expect(savedToast).toHaveCount(0);

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
	await expect(
		page.getByRole("button", { name: "Step 1: Create workspace" }),
	).toBeVisible();
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
		page
			.getByRole("toolbar", { name: "Workflow actions" })
			.getByText("Loop", { exact: true }),
	).toBeVisible();
	await expect(
		emit.getByText("This step would start a loop", { exact: true }),
	).toBeVisible();
	await expect(
		emit.getByRole("list", { name: "Events step 2 emits" }),
	).toContainText("starts “this workflow”");
	await emit.scrollIntoViewIfNeeded();
	await captureReviewCheckpoint(page, testInfo, "loop-warning");
	await expect(
		emit.getByRole("list", { name: "Events step 2 emits" }),
	).toContainText("custom.onboarding.started");
	await backToFlow(page);
	await expect(
		page
			.getByRole("application", { name: "Workflow canvas" })
			.getByText("Loop", { exact: true }),
	).toBeVisible();
	await captureReviewCheckpoint(page, testInfo, "loop-on-canvas");
	await page.getByRole("button", { name: "Save changes", exact: true }).click();
	await expect(page.getByText("Not saved", { exact: true })).toBeVisible();
	await expect(
		page.getByText(/Saving this would let the workflow start itself again/),
	).toBeVisible();
	await captureReviewCheckpoint(page, testInfo, "loop-refused");

	await page
		.getByRole("application", { name: "Workflow canvas" })
		.scrollIntoViewIfNeeded();
	await page.getByRole("button", { name: "Fit the workflow" }).click();
	const openHandoff = page.getByRole("button", {
		name: `Open “${handoff.name}”`,
	});
	await expect(openHandoff).toBeInViewport();
	await openHandoff.click();
	const leave = page.getByRole("alertdialog");
	await expect(leave).toContainText("Leave without saving?");
	await leave.getByRole("button", { name: "Stay", exact: true }).click();
	await expect(leave).toHaveCount(0);
	await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
	await openHandoff.click();
	await page
		.getByRole("alertdialog")
		.getByRole("button", { name: "Discard and leave", exact: true })
		.click();
	await expect(
		page.getByRole("heading", { name: handoff.name, exact: true }),
	).toBeVisible();
	const unchanged = await world.api.get<{ definition: { steps: unknown[] } }>(
		`${world.productPath}/workflows/${followUpId}`,
	);
	expect(unchanged.definition.steps).toHaveLength(1);

	await page.goto("/products/workflows");
	const links = page.getByRole("list", { name: "Workflow links" });
	await expect(links.getByRole("listitem")).toHaveCount(1);
	await expect(links.getByRole("listitem")).toHaveText(
		`${handoff.name}emitscustom.onboarding.startedwhich starts${name}`,
	);
	await links.scrollIntoViewIfNeeded();
	await captureReviewCheckpoint(page, testInfo, "workflow-links");
});

// Covers: PRODUCT_WORKFLOW_NEW, PRODUCT_WORKFLOW_DETAIL
test("a custom event's data is typed where it is sent, and a field cannot get a second type", async ({
	page,
	world,
}, testInfo) => {
	const event = `billing.${world.name("seats").replace("-", "_")}`;
	const first = await createWorkflowViaAPI(world, {
		name: world.name("seat-sync"),
		description: "",
		enabled: false,
		trigger_event_type: "organization.created",
		definition: {
			conditions: [],
			steps: [
				{
					id: "send",
					action: "workflow.emit",
					params: {
						event,
						data: '{"seats": "5"}',
						data_types: '{"seats": "number"}',
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
	const name = world.name("seat-upgrade");
	await page.getByLabel("Name", { exact: true }).fill(name);
	await page.getByRole("button", { name: "Trigger: Pick a trigger" }).click();
	await page
		.getByLabel("Event", { exact: true })
		.selectOption("organization.created");
	await backToFlow(page);
	await page.getByRole("button", { name: "Add a step", exact: true }).click();
	await page
		.getByRole("menuitem", { name: "Start other workflows", exact: true })
		.click();
	const send = page.getByRole("article", {
		name: "Step 1: Start other workflows",
	});
	await send
		.getByRole("combobox", { name: "Custom event *", exact: true })
		.fill(event);
	const data = send.getByRole("region", { name: "Event data" });
	await data.getByRole("button", { name: "Add field" }).click();
	const seats = data.getByRole("group", { name: "Event field 1" });
	await seats.getByRole("textbox", { name: "Name", exact: true }).fill("seats");
	await seats
		.getByRole("textbox", { name: "Value of seats", exact: true })
		.fill("25");
	await data.getByRole("button", { name: "Add field" }).click();
	const owner = data.getByRole("group", { name: "Event field 2" });
	await owner
		.getByRole("textbox", { name: "Name", exact: true })
		.fill("owner_org");
	await owner
		.getByRole("textbox", { name: "Value of owner_org", exact: true })
		.click();
	await send
		.getByRole("region", { name: "Data you can use" })
		.getByRole("button", {
			name: "organization_id, Organization ID, from Event",
		})
		.click();
	await expect(owner.getByRole("combobox", { name: "Type" })).toHaveValue(
		"organization",
	);

	await page
		.getByRole("button", { name: "Create workflow", exact: true })
		.click();
	await expect(
		data.getByText(`“${first.name}” sends it as number`, { exact: false }),
	).toBeVisible();
	await data.scrollIntoViewIfNeeded();
	await captureReviewCheckpoint(page, testInfo, "field-type-conflict");
	await seats.getByRole("combobox", { name: "Type" }).selectOption("number");
	await page
		.getByRole("button", { name: "Create workflow", exact: true })
		.click();
	await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
	const saved = await world.api.get<{
		definition: { steps: { params: Record<string, string> }[] };
	}>(`${world.productPath}/workflows/${workflowIdFrom(page)}`);
	expect(JSON.parse(saved.definition.steps[0].params.data_types)).toEqual({
		seats: "number",
		owner_org: "organization",
	});

	await openWorkflows(page, world);
	await page
		.getByRole("button", { name: "New workflow", exact: true })
		.first()
		.click();
	await page.getByRole("button", { name: "Trigger: Pick a trigger" }).click();
	await page
		.getByLabel("Event", { exact: true })
		.selectOption(`custom.${event}`);
	const carried = page
		.getByRole("region", { name: "Trigger settings", exact: true })
		.getByRole("list", { name: "Fields the event carries" });
	await expect(carried.getByRole("listitem")).toHaveText([
		"event.data.owner_orgOrganization ID",
		"event.data.seatsNumber",
	]);
	await captureReviewCheckpoint(page, testInfo, "typed-custom-event");
});

// Covers: PRODUCT_WORKFLOW_DETAIL
test("a backend that writes back during a step cannot restart its workflow, and the run history says so", async ({
	page,
	world,
}, testInfo) => {
	const backend = await writingBackend(world);
	try {
		await configureEventEndpoint(world, backend.eventsURL);
		const workflow = await createWorkflowViaAPI(world, {
			name: world.name("backend-round-trip"),
			enabled: true,
			trigger_event_type: "organization.created",
			definition: {
				conditions: [],
				steps: [
					{
						id: "call",
						action: "http.request",
						params: { url: backend.backendURL },
					},
				],
			},
		});

		await createOrganization(world, world.name("first-org"));
		await expect
			.poll(
				async () =>
					(await workflowRuns(world, workflow.id)).items
						.map((run) => run.status)
						.sort(),
				{ message: "the write-back is refused by the loop guard" },
			)
			.toEqual(["skipped", "succeeded"]);
		expect(backend.calls).toHaveLength(1);

		await openWorkflows(page, world);
		await page
			.getByRole("list", { name: "Your workflows", exact: true })
			.getByRole("button", { name: new RegExp(workflow.name) })
			.click();
		await expect(
			page.getByRole("heading", { name: workflow.name, exact: true }),
		).toBeVisible();
		await page.getByRole("tab", { name: "Runs (2)", exact: true }).click();
		const runs = page.getByRole("list", { name: "Runs" });
		await runs.getByRole("button").filter({ hasText: "Skipped" }).click();
		await expect(
			page
				.getByRole("tabpanel")
				.getByText(/^Loop prevented: this workflow already ran/),
		).toBeVisible();
		await captureReviewCheckpoint(page, testInfo, "loop-prevented-run");
	} finally {
		await backend.close();
	}
});
