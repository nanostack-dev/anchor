import { createServer } from "node:http";
import type { Page } from "playwright/test";
import type {
	ProductOrganizationResponse,
	ProductResponse,
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

export async function configureEventEndpoint(world: World, url: string) {
	await world.api.put<ProductResponse>(world.productPath, {
		name: world.product.name,
		description: world.product.description,
		config: { events: { endpoint_url: url, events: ["organization.deleted"] } },
	});
}

// A Product backend that, while answering a workflow step, creates an
// organization in Anchor and sends the causation header back.
export async function writingBackend(world: World) {
	const api = await world.productAPI();
	const calls: string[] = [];
	const server = createServer(async (request, response) => {
		for await (const _chunk of request) {
			/* drain */
		}
		if (request.url !== "/backend") {
			response.writeHead(204).end();
			return;
		}
		const causation = String(
			request.headers["anchor-workflow-causation"] ?? "",
		);
		calls.push(causation);
		const created = await api.context.post(
			`${world.productPath}/organizations`,
			{
				data: { name: `backend-${calls.length}-${Date.now()}` },
				headers: { "Anchor-Workflow-Causation": causation },
			},
		);
		response
			.writeHead(created.status() === 201 ? 200 : 500, {
				"Content-Type": "application/json",
			})
			.end(JSON.stringify({ created: created.status() }));
	});
	await new Promise<void>((ready, reject) => {
		server.once("error", reject);
		server.listen(0, "127.0.0.1", ready);
	});
	const address = server.address();
	if (!address || typeof address === "string")
		throw new Error("Backend did not bind TCP.");
	const origin = `http://127.0.0.1:${address.port}`;
	return {
		backendURL: `${origin}/backend`,
		eventsURL: `${origin}/events`,
		calls,
		async close() {
			await new Promise<void>((closed, reject) =>
				server.close((error) => (error ? reject(error) : closed())),
			);
		},
	};
}
