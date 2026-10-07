import type { Page } from "playwright/test";
import type {
	ProductOrganizationResponse,
	ProductWorkspaceListResponse,
	WorkflowResponse,
	WorkflowRunListResponse,
	WorkflowWriteRequest,
} from "../../../src/client";
import { type World, expect } from "../../support/fixtures";
import { selectProduct } from "../../support/ui";

export async function openWorkflows(page: Page, world: World) {
	await selectProduct(page, world.product);
	await page.goto("/products/workflows");
	await expect(
		page.getByRole("heading", { name: "Workflows", exact: true }),
	).toBeVisible();
}

export async function createOrganization(world: World, name: string) {
	const api = await world.productAPI();
	return api.post<ProductOrganizationResponse>(
		`${world.productPath}/organizations`,
		{ name },
	);
}

export async function workspaceNames(world: World, organizationId: string) {
	const api = await world.productAPI();
	const result = await api.post<ProductWorkspaceListResponse>(
		`${world.productPath}/organizations/${organizationId}/workspaces/search`,
		{ pagination: { limit: 100, offset: 0 } },
		200,
	);
	return result.items.map((workspace) => workspace.name);
}

export async function workflowRuns(world: World, workflowId: string) {
	return world.api.get<WorkflowRunListResponse>(
		`${world.productPath}/workflows/${workflowId}/runs`,
	);
}

export function workflowIdFrom(page: Page) {
	const match = new URL(page.url()).pathname.match(
		/\/products\/workflows\/(wf_[^/]+)$/,
	);
	expect(match, "the saved workflow opens on its own route").not.toBeNull();
	return match?.[1] ?? "";
}

export async function createWorkflowViaAPI(
	world: World,
	body: WorkflowWriteRequest,
) {
	return world.api.post<WorkflowResponse>(
		`${world.productPath}/workflows`,
		body,
	);
}

export async function latestRun(world: World, workflowId: string) {
	return (await workflowRuns(world, workflowId)).items[0];
}
