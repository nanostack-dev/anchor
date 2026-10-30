import { AccessDenyReason, type ResourceResponse } from "@/client";
import { describe, expect, it } from "vitest";

import {
	type CheckQuestion,
	MAX_CHAIN,
	explain,
	heldOnTypes,
	highlightOf,
	walkChain,
} from "./access-explainer";
import type { TreeType } from "./resource-type-tree";

function type(slug: string, parents: string[] = ["organization"]): TreeType {
	return {
		key: `rtype_${slug}`,
		slug,
		name: slug[0].toUpperCase() + slug.slice(1),
		parents: slug === "organization" ? [] : parents.map((p) => `rtype_${p}`),
		nestsInItself: slug === "folder",
		customRoles: false,
		builtIn: slug === "organization",
		published: true,
		slugEdited: false,
	};
}

const types = [
	type("organization"),
	type("project"),
	type("folder", ["project"]),
	type("document", ["folder", "project"]),
];

function resource(
	typeSlug: string,
	id: string,
	name: string,
	parent: { type: string; id: string },
): ResourceResponse {
	return {
		type: typeSlug,
		id,
		name,
		organization_id: "org_acme",
		parent,
		created_at: "2026-10-24T09:12:31Z",
		updated_at: "2026-10-24T09:12:31Z",
	};
}

const project = resource("project", "p1", "Website redesign", {
	type: "organization",
	id: "org_acme",
});
const design = resource("folder", "f1", "Design", {
	type: "project",
	id: "p1",
});
const specs = resource("folder", "f2", "Specs", { type: "folder", id: "f1" });
const page = resource("document", "d1", "Pricing page", {
	type: "folder",
	id: "f2",
});
const registry = new Map([project, design, specs, page].map((r) => [r.id, r]));

function question(
	subject: ResourceResponse | null,
	permission = { name: "document:view", scope: "document" },
): CheckQuestion {
	return {
		organization: { id: "org_acme", name: "Acme Corp" },
		member: { label: "Dan Okafor", externalId: "user_dan" },
		permission,
		resource: subject,
	};
}

const fetchParent = async ({ id }: { id: string }) => {
	const found = registry.get(id);
	if (!found) throw new Error(`no ${id}`);
	return found;
};

describe("walkChain", () => {
	it("walks a document up through its folders and project to the organization", async () => {
		const chain = await walkChain(question(page), fetchParent);
		expect(chain.complete).toBe(true);
		expect(chain.links.map((link) => link.name)).toEqual([
			"Pricing page",
			"Specs",
			"Design",
			"Website redesign",
			"Acme Corp",
		]);
	});

	it("is the organization alone when the organization itself is checked", async () => {
		const chain = await walkChain(question(null), fetchParent);
		expect(chain.links).toEqual([
			{ type: "organization", id: "org_acme", name: "Acme Corp" },
		]);
	});

	it("stops after fifteen links counting the organization and reports an incomplete chain", async () => {
		let calls = 0;
		const endless = async ({ id }: { id: string }) => {
			calls += 1;
			return resource("folder", `${id}x`, `Folder ${calls}`, {
				type: "folder",
				id: `${id}x`,
			});
		};
		const deep = resource("document", "d", "Deep", { type: "folder", id: "f" });
		const chain = await walkChain(question(deep), endless);
		expect(chain.complete).toBe(false);
		expect(chain.links).toHaveLength(MAX_CHAIN - 1);
		expect(calls).toBe(MAX_CHAIN - 2);
	});
});

describe("explain", () => {
	const chain = [page, specs, design, project].map((r) => ({
		type: r.type,
		id: r.id,
		name: r.name,
	}));
	chain.push({ type: "organization", id: "org_acme", name: "Acme Corp" });

	it("names each grant with its role and the resource it is held on", () => {
		const explanation = explain(
			{
				decision: true,
				context: {
					granted_by: [
						{
							role: { id: "role_fv", name: "folder-viewer" },
							resource: { type: "folder", id: "f1" },
						},
						{
							role: { id: "role_m", name: "member" },
							resource: { type: "organization", id: "org_acme" },
						},
					],
				},
			},
			question(page),
			chain,
			types,
		);
		expect(explanation).toEqual({
			decision: "allowed",
			grants: [
				{
					id: "role_fv",
					holder: "folder-viewer",
					kind: "role",
					link: { type: "folder", id: "f1", name: "Design" },
				},
				{
					id: "role_m",
					holder: "member",
					kind: "role",
					link: { type: "organization", id: "org_acme", name: "Acme Corp" },
				},
			],
		});
	});

	it("keeps a missing explanation apart from an empty one", () => {
		const unread = explain(
			{ decision: true, context: {} },
			question(page),
			chain,
			types,
		);
		const syncing = explain(
			{ decision: true, context: { granted_by: [] } },
			question(page),
			chain,
			types,
		);
		expect(unread).toEqual({ decision: "allowed", grants: null });
		expect(syncing).toEqual({ decision: "allowed", grants: [] });
	});

	it("falls back to IDs for a grant off the chain and a role with no name", () => {
		const explanation = explain(
			{
				decision: true,
				context: {
					granted_by: [
						{ role: { id: "role_x" }, resource: { type: "project", id: "p9" } },
					],
				},
			},
			question(page),
			chain,
			types,
		);
		expect(explanation).toEqual({
			decision: "allowed",
			grants: [
				{
					id: "role_x",
					holder: "role_x",
					kind: "role",
					link: { type: "project", id: "p9", name: "p9" },
				},
			],
		});
	});

	it("says roles reach down, never up, when a permission held on the type is missing", () => {
		const explanation = explain(
			{
				decision: false,
				context: { reason: AccessDenyReason.MISSING_PERMISSION },
			},
			question(project),
			chain.slice(3),
			types,
		);
		expect(explanation).toMatchObject({
			decision: "denied",
			reason: "MISSING_PERMISSION",
			title:
				"No role Dan Okafor holds on Website redesign or above it grants document:view.",
		});
	});

	it("says a permission of a type above doesn't apply to the resource's type", () => {
		const explanation = explain(
			{
				decision: false,
				context: { reason: AccessDenyReason.MISSING_PERMISSION },
			},
			question(page, { name: "project:manage", scope: "project" }),
			chain,
			types,
		);
		expect(explanation).toMatchObject({
			title: "project:manage doesn't apply to Document resources.",
		});
	});

	it.each(Object.values(AccessDenyReason))(
		"describes the %s reason in plain language",
		(reason) => {
			const explanation = explain(
				{ decision: false, context: { reason } },
				question(page),
				chain,
				types,
			);
			expect(explanation.decision).toBe("denied");
			if (explanation.decision !== "denied") return;
			expect(explanation.reason).toBe(reason);
			expect(explanation.title).not.toBe("Anchor denied access.");
			expect(explanation.detail.length).toBeGreaterThan(0);
		},
	);

	it("keeps an unknown reason readable", () => {
		const explanation = explain(
			{
				decision: false,
				context: { reason: "SOMETHING_NEW" as AccessDenyReason },
			},
			question(page),
			chain,
			types,
		);
		expect(explanation).toMatchObject({
			reason: "SOMETHING_NEW",
			title: "Anchor denied access.",
		});
	});
});

describe("heldOnTypes", () => {
	it("holds a permission on its own type and every type above it", () => {
		expect([...heldOnTypes(types, "document")].sort()).toEqual([
			"document",
			"folder",
			"organization",
			"project",
		]);
		expect([...heldOnTypes(types, "project")].sort()).toEqual([
			"organization",
			"project",
		]);
	});

	it("walks a self-nesting type once", () => {
		expect([...heldOnTypes(types, "folder")].sort()).toEqual([
			"folder",
			"organization",
			"project",
		]);
	});
});

describe("highlightOf", () => {
	it("lights each type on the chain once and marks the type that holds the grant", () => {
		const links = [
			{ type: "document", id: "d1", name: "Pricing page" },
			{ type: "folder", id: "f2", name: "Specs" },
			{ type: "folder", id: "f1", name: "Design" },
			{ type: "project", id: "p1", name: "Website redesign" },
			{ type: "organization", id: "org_acme", name: "Acme Corp" },
		];
		expect(
			highlightOf(
				{
					decision: "allowed",
					grants: [
						{
							id: "role_fv",
							holder: "folder-viewer",
							kind: "role",
							link: links[2],
						},
					],
				},
				links,
			),
		).toEqual({
			decision: "allowed",
			path: ["document", "folder", "project", "organization"],
			steps: [
				["folder", "document"],
				["project", "folder"],
				["organization", "project"],
			],
			granted: { folder: ["folder-viewer"] },
		});
	});

	it("keeps two roles that share a name, and lists one role once", () => {
		const design = { type: "folder", id: "f1", name: "Design" };
		const editor = (id: string) => ({
			id,
			holder: "Editor",
			kind: "role" as const,
			link: design,
		});
		expect(
			highlightOf(
				{
					decision: "allowed",
					grants: [
						editor("role_managed"),
						editor("role_custom"),
						editor("role_custom"),
					],
				},
				[design],
			).granted,
		).toEqual({ folder: ["Editor", "Editor"] });
	});

	it("marks no type as granting on a denial", () => {
		const design = { type: "folder", id: "f1", name: "Design" };
		expect(
			highlightOf(
				{
					decision: "denied",
					reason: "MISSING_PERMISSION",
					title: "",
					detail: "",
				},
				[design],
			).granted,
		).toEqual({});
	});
});
