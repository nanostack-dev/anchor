import {
	OrganizationRoleOrigin,
	type OrganizationRoleResponse,
	type ProductResourcePermissionResponse,
	type ProductResourceTypeResponse,
	type ProductRoleResponse,
} from "@/client";
import { client } from "@/client/client.gen";

export const STORY_PRODUCT_ID = "prd_2Nq8xKf3pLmR";
export const STORY_ORGANIZATION_ID = "org_2qL9aC3mE8xP1rT6vN4bK2sJ";
const STAMP = "2026-10-24T09:12:31Z";

export function storyType(
	slug: string,
	name: string,
	parents: string[] = ["organization"],
	extra: Partial<ProductResourceTypeResponse> = {},
): ProductResourceTypeResponse {
	return {
		id: `rtype_${slug}`,
		product_id: STORY_PRODUCT_ID,
		slug,
		name,
		parent_types: slug === "organization" ? [] : parents,
		nests_in_itself: false,
		custom_roles: false,
		built_in: slug === "organization",
		created_at: STAMP,
		updated_at: STAMP,
		...extra,
	};
}

export function storyPermission(
	name: string,
	resourceType: string,
	description: string | null = null,
): ProductResourcePermissionResponse {
	return {
		product_id: STORY_PRODUCT_ID,
		name,
		description,
		resource_type: resourceType,
		created_at: STAMP,
		updated_at: STAMP,
	};
}

export function storyRole(
	name: string,
	resourceType: string,
	permissions: string[],
	extra: Partial<ProductRoleResponse> = {},
): ProductRoleResponse {
	const id = `role_${name
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, "_")
		.slice(0, 40)}`;
	return {
		id,
		product_id: STORY_PRODUCT_ID,
		name,
		description: `The ${name} role.`,
		resource_type: resourceType,
		key: name.toLowerCase(),
		is_default: false,
		permissions: permissions.map((permission_name) => ({
			product_id: STORY_PRODUCT_ID,
			product_role_id: id,
			permission_name,
		})),
		created_at: STAMP,
		updated_at: STAMP,
		...extra,
	};
}

export function storyOrganizationRole(
	role: ProductRoleResponse,
	extra: Partial<OrganizationRoleResponse> = {},
): OrganizationRoleResponse {
	return {
		id: role.id,
		organization_id: STORY_ORGANIZATION_ID,
		template_id: role.id,
		key: role.key,
		name: role.name,
		description: role.description,
		origin: OrganizationRoleOrigin.TEMPLATE,
		customized: false,
		resource_type: role.resource_type,
		is_default: role.is_default,
		permissions: role.permissions.map((grant) => grant.permission_name),
		created_at: STAMP,
		updated_at: STAMP,
		...extra,
	};
}

export const DOCUMENT_TREE = [
	storyType("organization", "Organization"),
	storyType("project", "Project"),
	storyType("folder", "Folder", ["project"], { nests_in_itself: true }),
	storyType("document", "Document", ["folder", "project"]),
];

export const DOCUMENT_PERMISSIONS = [
	storyPermission(
		"members:update",
		"organization",
		"Change who belongs to the organization.",
	),
	storyPermission(
		"billing:read",
		"organization",
		"Read invoices and the plan.",
	),
	storyPermission(
		"project:manage",
		"project",
		"Rename, archive and configure a project.",
	),
	storyPermission("folder:view", "folder", "Open a folder and list it."),
	storyPermission("folder:edit", "folder", "Rename and move a folder."),
	storyPermission("document:view", "document", "Read a document."),
	storyPermission("document:edit", "document", "Change a document."),
	storyPermission("document:share", "document", "Share a document."),
];

export const DOCUMENT_ROLES = [
	storyRole(
		"admin",
		"organization",
		DOCUMENT_PERMISSIONS.map((permission) => permission.name),
	),
	storyRole("member", "organization", [], { is_default: true }),
	storyRole("project-admin", "project", [
		"project:manage",
		"folder:edit",
		"document:view",
		"document:edit",
		"document:share",
	]),
	storyRole("folder-viewer", "folder", ["folder:view", "document:view"]),
	storyRole("document-viewer", "document", ["document:view"]),
];

/** A chain five levels deep, the deepest tree Anchor accepts. */
export const DEEP_TREE = [
	storyType("organization", "Organization"),
	storyType("region", "Region"),
	storyType("site", "Site", ["region"]),
	storyType("building", "Building", ["site"]),
	storyType("room", "Room", ["building"]),
];

/** The largest catalog: fifty types besides Organization, two permissions each. */
export const FIFTY_ONE_TYPES = [
	storyType("organization", "Organization"),
	...Array.from({ length: 50 }, (_, index) =>
		storyType(`type_${index}`, `Resource type number ${index}`),
	),
];

export const LONG_NAME =
	"RegulatoryComplianceEvidenceBundleWithAttachmentsAndSignaturesForAuditors";

type Reply<T> = T | "pending" | "fail";

export interface AccessBackend {
	types?: Reply<ProductResourceTypeResponse[]>;
	permissions?: Reply<ProductResourcePermissionResponse[]>;
	roles?: Reply<ProductRoleResponse[]>;
	organizationRoles?: Reply<OrganizationRoleResponse[]>;
	saveStatus?: number;
}

export interface RecordedCall {
	method: string;
	path: string;
	body: Record<string, unknown>;
}

function json(body: unknown, status = 200) {
	return Response.json(body, { status });
}

function failure() {
	return json(
		{ errors: [{ code: "UNEXPECTED_ERROR", message: "Unavailable" }] },
		500,
	);
}

function paged<T>(items: T[], body: Record<string, unknown>) {
	const pagination = (body.pagination ?? {}) as {
		limit?: number;
		offset?: number;
	};
	const offset = pagination.offset ?? 0;
	const limit = pagination.limit ?? 10;
	const slice = items.slice(offset, offset + limit);
	return json({ items: slice, total: items.length, count: slice.length });
}

/**
 * Installs an in-memory Anchor for the roles, permissions and resource type
 * screens, and returns the calls it received plus a restore function.
 */
export function mockAccessBackend(backend: AccessBackend) {
	const calls: RecordedCall[] = [];
	const previous = client.getConfig();
	let roles = backend.roles;
	client.setConfig({
		baseUrl: window.location.origin,
		fetch: async (request) => {
			if (!(request instanceof Request)) throw new Error("Expected Request");
			const path = decodeURIComponent(new URL(request.url).pathname);
			const body = (
				request.method === "GET" || request.method === "DELETE"
					? {}
					: await request.json().catch(() => ({}))
			) as Record<string, unknown>;
			calls.push({ method: request.method, path, body });
			const filter = (body.filter ?? {}) as {
				resource_types?: string[];
				permissions?: string[];
				ids?: string[];
				names?: string[];
			};
			const reply = <T>(
				value: Reply<T[]> | undefined,
				keep: (item: T) => boolean,
			) => {
				if (value === "pending") return new Promise<Response>(() => {});
				if (value === "fail") return failure();
				return paged((value ?? []).filter(keep), body);
			};
			if (path === "/v1/products/search")
				return json({
					items: [
						{
							id: STORY_PRODUCT_ID,
							tenant_id: "tenant_story",
							name: "Lumen Docs",
							created_at: STAMP,
							updated_at: STAMP,
						},
					],
					total: 1,
				});
			if (path.endsWith("/resource-types/search"))
				return reply(backend.types ?? DOCUMENT_TREE, () => true);
			if (path.endsWith("/resource-permissions/search"))
				return reply<ProductResourcePermissionResponse>(
					backend.permissions,
					(permission) =>
						(!filter.resource_types ||
							filter.resource_types.includes(permission.resource_type)) &&
						(!filter.names || filter.names.includes(permission.name)),
				);
			if (path.endsWith(`/organizations/${STORY_ORGANIZATION_ID}/roles/search`))
				return reply<OrganizationRoleResponse>(
					backend.organizationRoles,
					(role) =>
						!filter.resource_types ||
						filter.resource_types.includes(role.resource_type),
				);
			if (path.endsWith("/roles/search"))
				return reply<ProductRoleResponse>(
					roles,
					(role) =>
						(!filter.resource_types ||
							filter.resource_types.includes(role.resource_type)) &&
						(!filter.ids || filter.ids.includes(role.id)) &&
						(!filter.names || filter.names.includes(role.name)) &&
						(!filter.permissions ||
							role.permissions.some((grant) =>
								filter.permissions?.includes(grant.permission_name),
							)),
				);
			if (path.endsWith("/roles") && request.method === "POST") {
				if (backend.saveStatus && backend.saveStatus !== 201)
					return json(
						{
							errors: [
								{
									code: "ROLE_NAME_DUPLICATE",
									message: "A role with this name already exists.",
								},
							],
						},
						backend.saveStatus,
					);
				const created = storyRole(
					String(body.name),
					String(body.resource_type),
					(body.permissions as string[]) ?? [],
					{ is_default: body.is_default === true },
				);
				roles = Array.isArray(roles) ? [...roles, created] : [created];
				return json(created, 201);
			}
			const roleMatch = path.match(/\/roles\/([^/]+)$/);
			if (roleMatch && Array.isArray(roles)) {
				const current = roles.find((role) => role.id === roleMatch[1]);
				if (!current) return json({ errors: [] }, 404);
				if (request.method === "PUT") {
					if (backend.saveStatus && backend.saveStatus !== 200)
						return json(
							{
								errors: [
									{ code: "UNEXPECTED_ERROR", message: "Update failed" },
								],
							},
							backend.saveStatus,
						);
					const updated = storyRole(
						String(body.name),
						current.resource_type,
						(body.permissions as string[]) ?? [],
						{
							id: current.id,
							key: current.key,
							description: (body.description as string | null) ?? null,
							is_default:
								typeof body.is_default === "boolean"
									? body.is_default
									: current.is_default,
						},
					);
					roles = roles.map((role) =>
						role.id === current.id ? updated : role,
					);
					return json(updated);
				}
				return json(current);
			}
			if (path.endsWith("/resource-permissions") && request.method === "POST") {
				if (backend.saveStatus && backend.saveStatus !== 201)
					return json(
						{
							errors: [
								{
									code: "RESOURCE_PERMISSION_DUPLICATE",
									message: "A permission with this name already exists.",
								},
							],
						},
						backend.saveStatus,
					);
				return json(
					storyPermission(
						String(body.name),
						String(body.resource_type),
						(body.description as string | null) ?? null,
					),
					201,
				);
			}
			throw new Error(`Unexpected request: ${request.method} ${path}`);
		},
	});
	return {
		calls,
		restore: () => client.setConfig(previous),
	};
}
