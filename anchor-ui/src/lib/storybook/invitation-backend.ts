import type {
	OrganizationInvitationResponse,
	OrganizationInvitationStatus,
	ProductResponse,
	ProductRoleResponse,
} from "@/client";
import { client } from "@/client/client.gen";

export type RecordedRequest = {
	method: string;
	path: string;
	body: unknown;
};

export type InvitationBackendState = {
	product: ProductResponse;
	invitations: OrganizationInvitationResponse[];
	roles: ProductRoleResponse[];
	failInvitationSearch?: boolean;
};

function json(body: unknown, status = 200) {
	return Response.json(body, { status });
}

function page<T>(
	items: T[],
	body: { pagination?: { limit?: number; offset?: number } },
) {
	const offset = body.pagination?.offset ?? 0;
	const limit = body.pagination?.limit ?? items.length;
	const slice = items.slice(offset, offset + limit);
	return { items: slice, total: items.length, count: slice.length };
}

/**
 * An in-memory stand-in for the Anchor API endpoints the invitation list
 * calls. It mutates `state` the way the real service does, so a story can
 * assert what the screen shows after a delete.
 */
export function createInvitationBackend(state: InvitationBackendState) {
	const requests: RecordedRequest[] = [];

	async function handle(request: Request): Promise<Response> {
		const url = new URL(request.url);
		const path = url.pathname;
		const rawBody =
			request.method === "GET" || request.method === "DELETE"
				? undefined
				: await request.json().catch(() => undefined);
		requests.push({ method: request.method, path, body: rawBody });
		const body = (rawBody ?? {}) as {
			pagination?: { limit?: number; offset?: number };
			filter?: { statuses?: OrganizationInvitationStatus[] };
		};

		if (path === "/v1/products/search") {
			return json(page([state.product], body));
		}
		if (path.endsWith("/roles/search")) {
			return json(page(state.roles, body));
		}
		if (path.endsWith("/invitations/search")) {
			if (state.failInvitationSearch) {
				return json(
					{
						errors: [
							{
								code: "UNEXPECTED_ERROR",
								message: "The database is unreachable",
							},
						],
					},
					500,
				);
			}
			const statuses = body.filter?.statuses ?? [];
			const matching = state.invitations.filter(
				(invitation) =>
					statuses.length === 0 || statuses.includes(invitation.status),
			);
			return json(page(matching, body));
		}
		const single = path.match(/\/invitations\/([^/]+)$/);
		if (single && request.method === "DELETE") {
			state.invitations = state.invitations.filter(
				(item) => item.id !== single[1],
			);
			return new Response(null, { status: 204 });
		}
		return json(
			{ errors: [{ code: "NOT_FOUND", message: `No fake for ${path}` }] },
			404,
		);
	}

	function install() {
		const previousConfig = client.getConfig();
		client.setConfig({
			baseUrl: window.location.origin,
			fetch: async (input) => {
				if (!(input instanceof Request)) throw new Error("Expected a Request");
				return handle(input);
			},
		});
		return () => {
			client.setConfig({
				...previousConfig,
				fetch: previousConfig.fetch ?? globalThis.fetch.bind(globalThis),
			});
		};
	}

	return { state, requests, install };
}
