import type {
	AccessEvaluationRequest,
	AccessEvaluationResponse,
	OrganizationMemberResponse,
	ProductOrganizationResponse,
	ProductResourcePermissionResponse,
	ProductResourceTypeResponse,
	ResourceResponse,
} from "@/client";

export const PRODUCT_ID = "prd_2Nq8xKf3pLmR";
const STAMP = "2026-10-24T09:12:31Z";

export function resourceType(
	slug: string,
	name: string,
	parentTypes: string[] = ["organization"],
	nests = false,
): ProductResourceTypeResponse {
	return {
		id: `rtype_${slug}`,
		product_id: PRODUCT_ID,
		slug,
		name,
		parent_types: parentTypes,
		nests_in_itself: nests,
		allows_custom_roles: false,
		built_in: slug === "organization",
		created_at: STAMP,
		updated_at: STAMP,
	};
}

export const DOCUMENT_TREE = [
	resourceType("organization", "Organization", []),
	resourceType("project", "Project"),
	resourceType("folder", "Folder", ["project"], true),
	resourceType("document", "Document", ["folder", "project"]),
];

export function organization(
	id: string,
	name: string,
): ProductOrganizationResponse {
	return {
		id,
		product_id: PRODUCT_ID,
		name,
		created_at: STAMP,
		updated_at: STAMP,
	};
}

export function member(
	id: string,
	name: string | null,
	email: string,
	externalId: string | null = `user_${id}`,
): OrganizationMemberResponse {
	return {
		product_user_id: `pusr_${id}`,
		name,
		email,
		external_id: externalId,
		role: { id: "role_member", name: "member" },
		joined_at: STAMP,
	};
}

export function permission(
	name: string,
	scope: string,
): ProductResourcePermissionResponse {
	return {
		product_id: PRODUCT_ID,
		name,
		resource_type: scope,
		created_at: STAMP,
		updated_at: STAMP,
	};
}

export function resource(
	type: string,
	id: string,
	name: string,
	parent: { type: string; id: string },
	organizationId = "org_acme",
): ResourceResponse {
	return {
		type,
		id,
		name,
		organization_id: organizationId,
		parent,
		created_at: STAMP,
		updated_at: STAMP,
	};
}

export const ACME = organization("org_acme", "Acme Corp");
export const GLOBEX = organization("org_globex", "Globex");
export const DAN = member("dan", "Dan Okafor", "dan@acme.example");
export const PRIYA = member("priya", null, "priya@acme.example", null);
export const DOCUMENT_VIEW = permission("document:view", "document");
export const PROJECT_VIEW = permission("project:view", "project");
export const PERMISSIONS = [
	DOCUMENT_VIEW,
	permission("document:edit", "document"),
	permission("folder:view", "folder"),
	PROJECT_VIEW,
	permission("project:manage", "project"),
];
export const WEBSITE = resource("project", "p_website", "Website redesign", {
	type: "organization",
	id: ACME.id,
});
export const DESIGN = resource("folder", "f_design", "Design", {
	type: "project",
	id: WEBSITE.id,
});
export const SPECS = resource("folder", "f_specs", "Specs", {
	type: "folder",
	id: DESIGN.id,
});
export const PRICING = resource("document", "d_pricing", "Pricing page", {
	type: "folder",
	id: SPECS.id,
});
export const RESOURCES = [WEBSITE, DESIGN, SPECS, PRICING];

export function allowed(
	granted: { role: string; type: string; id: string }[],
): AccessEvaluationResponse {
	return {
		decision: true,
		context: {
			organization: { id: ACME.id, name: ACME.name },
			role: { id: "role_member", name: "member" },
			permissions: [],
			granted_by: granted.map((grant) => ({
				role: { id: `role_${grant.role}`, name: grant.role },
				resource: { type: grant.type, id: grant.id },
			})),
		},
	};
}

export function denied(reason: string): AccessEvaluationResponse {
	return {
		decision: false,
		context: { reason } as AccessEvaluationResponse["context"],
	};
}

export type Answer = { status: number; body: unknown };
export type Reply = Answer | Promise<Answer> | "pending";

export interface AccessBackend {
	types?: ProductResourceTypeResponse[];
	organizations?: ProductOrganizationResponse[] | "fail";
	members?: OrganizationMemberResponse[];
	permissions?: ProductResourcePermissionResponse[];
	resources?: ResourceResponse[];
	evaluate?: (request: AccessEvaluationRequest, attempt: number) => Reply;
	evaluated?: AccessEvaluationRequest[];
}

const page = <T>(items: T[]) =>
	Response.json({ items, total: items.length, count: items.length });

const matches = (text: string | null | undefined, term: unknown) =>
	!term || (text ?? "").toLowerCase().includes(String(term).toLowerCase());

export function accessRoute(
	backend: AccessBackend,
	method: string,
	path: string,
	body: {
		full_text_search?: string;
		filter?: { resource_types?: string[]; name_contains?: string };
	},
): Promise<Response> | Response | undefined {
	if (path.endsWith("/organizations/search")) {
		if (backend.organizations === "fail")
			return Response.json(
				{ errors: [{ code: "UNEXPECTED_ERROR", message: "Unavailable" }] },
				{ status: 500 },
			);
		return page(
			(backend.organizations ?? []).filter((item) =>
				matches(item.name, body.full_text_search),
			),
		);
	}
	if (path.endsWith("/members/search"))
		return page(
			(backend.members ?? []).filter(
				(item) =>
					matches(item.name, body.full_text_search) ||
					matches(item.email, body.full_text_search),
			),
		);
	if (path.endsWith("/resource-permissions/search") && !body.filter)
		return page(
			(backend.permissions ?? []).filter((item) =>
				matches(item.name, body.full_text_search),
			),
		);
	if (path.endsWith("/resources/search"))
		return page(
			(backend.resources ?? []).filter(
				(item) =>
					(body.filter?.resource_types ?? [item.type]).includes(item.type) &&
					matches(item.name, body.filter?.name_contains),
			),
		);
	const single = path.match(/\/resources\/([^/]+)\/([^/]+)$/);
	if (single && method === "GET") {
		const found = (backend.resources ?? []).find(
			(item) => item.type === single[1] && item.id === single[2],
		);
		return found
			? Response.json(found)
			: Response.json(
					{ errors: [{ code: "RESOURCE_NOT_FOUND", message: "Not found" }] },
					{ status: 404 },
				);
	}
	if (path.endsWith("/access/v1/evaluation")) {
		const request = body as unknown as AccessEvaluationRequest;
		backend.evaluated?.push(request);
		const reply = backend.evaluate?.(
			request,
			backend.evaluated?.length ?? 1,
		) ?? { status: 200, body: denied("MISSING_PERMISSION") };
		if (reply === "pending") return new Promise<Response>(() => {});
		return Promise.resolve(reply).then((answer) =>
			Response.json(answer.body, { status: answer.status }),
		);
	}
	return undefined;
}
