import type { ProductResourceTypeResponse } from "@/client";
import { describe, expect, it } from "vitest";
import { publishProblems } from "./publish-errors";
import { layoutTree, neighbour } from "./resource-type-layout";
import {
	addChild,
	addChildBlocker,
	deletedSlugs,
	describeDraft,
	draftProblems,
	parentCandidates,
	rename,
	setSlug,
	slugify,
	summarizeDraft,
	switchChanges,
	toggleParent,
	treeChanged,
	treeFromResponse,
	treeRequest,
	updateType,
} from "./resource-type-tree";

function type(
	slug: string,
	parents: string[],
	extra: Partial<ProductResourceTypeResponse> = {},
): ProductResourceTypeResponse {
	return {
		id: `rtype_${slug}`,
		product_id: "prd_test",
		slug,
		name: slug[0].toUpperCase() + slug.slice(1),
		parent_types: parents,
		nests_in_itself: false,
		allows_custom_roles: false,
		built_in: slug === "organization",
		created_at: "2026-10-24T09:00:00Z",
		updated_at: "2026-10-24T09:00:00Z",
		...extra,
	};
}

const published = treeFromResponse([
	type("project", ["organization"]),
	type("organization", []),
	type("folder", ["project"], { nests_in_itself: true }),
	type("document", ["folder", "project"]),
]);

describe("resource type tree", () => {
	it("puts organization first and refers to parents by key", () => {
		expect(published.map((item) => item.slug)).toEqual([
			"organization",
			"project",
			"folder",
			"document",
		]);
		expect(published[3].parents).toEqual(["rtype_folder", "rtype_project"]);
	});

	it("publishes every type but organization with parents as slugs", () => {
		expect(treeRequest(published)).toEqual([
			{
				slug: "project",
				name: "Project",
				description: undefined,
				parent_types: ["organization"],
				nests_in_itself: false,
			},
			{
				slug: "folder",
				name: "Folder",
				description: undefined,
				parent_types: ["project"],
				nests_in_itself: true,
			},
			{
				slug: "document",
				name: "Document",
				description: undefined,
				parent_types: ["folder", "project"],
				nests_in_itself: false,
			},
		]);
	});

	it("derives a new type's slug from its name until the slug is edited", () => {
		let draft = rename(
			addChild(published, "rtype_folder", "draft_1"),
			"draft_1",
			"Section Ünïcode 2",
		);
		expect(draft.at(-1)?.slug).toBe("section_unicode_2");
		draft = setSlug(draft, "draft_1", "chapter");
		draft = rename(draft, "draft_1", "Chapter one");
		expect(draft.at(-1)?.slug).toBe("chapter");
	});

	it("never changes a published slug", () => {
		const draft = setSlug(
			rename(published, "rtype_folder", "Directory"),
			"rtype_folder",
			"directory",
		);
		expect(draft[2]).toMatchObject({ name: "Directory", slug: "folder" });
	});

	it("never reuses the slug of a type the draft deletes", () => {
		let draft = published.filter((item) => item.slug !== "document");
		const reserved = deletedSlugs(published, draft);
		draft = rename(
			addChild(draft, "rtype_folder", "draft_1"),
			"draft_1",
			"Document",
			reserved,
		);
		expect(draft.at(-1)?.slug).toBe("document_2");
		draft = setSlug(draft, "draft_1", "document");
		expect(draftProblems(draft, reserved)).toEqual([
			"Document: document belongs to a type this draft deletes. Choose another slug, or publish the deletion first.",
		]);
	});

	it("keeps generated slugs valid and unique", () => {
		expect(slugify("Project", new Set(["project"]))).toBe("project_2");
		expect(slugify("42 Things!", new Set())).toBe("things");
		expect(slugify("日本", new Set())).toBe("type");
		expect(slugify("x".repeat(80), new Set())).toHaveLength(49);
	});

	it("summarizes a draft against the published tree", () => {
		let draft = rename(published, "rtype_folder", "Directory");
		draft = rename(addChild(draft, "rtype_project", "draft_1"), "draft_1", "A");
		draft = rename(addChild(draft, "rtype_project", "draft_2"), "draft_2", "B");
		draft = toggleParent(draft, "rtype_document", "rtype_project");
		draft = draft.filter((item) => item.slug !== "document");
		const summary = summarizeDraft(published, draft);
		expect(summary).toEqual({
			added: 2,
			renamed: 1,
			changed: 0,
			deleted: 1,
			switched: 0,
		});
		expect(describeDraft(summary)).toBe("2 types added, 1 renamed, 1 deleted");
		expect(describeDraft(summarizeDraft(published, published))).toBe(
			"No changes yet",
		);
	});

	it("counts a custom roles switch apart from the tree, which the tree call doesn't carry", () => {
		const draft = updateType(published, "rtype_organization", (item) => ({
			...item,
			customRoles: true,
		}));
		const summary = summarizeDraft(published, draft);
		expect(summary.switched).toBe(1);
		expect(treeChanged(summary)).toBe(false);
		expect(describeDraft(summary)).toBe("custom roles switched on 1 type");
		expect(switchChanges(published, draft).map((item) => item.slug)).toEqual([
			"organization",
		]);
		expect(treeRequest(draft)).toEqual(treeRequest(published));
	});

	it("never removes a type's last parent", () => {
		const draft = toggleParent(
			published,
			"rtype_project",
			"rtype_organization",
		);
		expect(draft[1].parents).toEqual(["rtype_organization"]);
	});

	it("offers only parents that keep the tree acyclic and at most five deep", () => {
		expect(
			parentCandidates(published, "rtype_project").map((item) => item.slug),
		).toEqual(["organization"]);
		expect(
			parentCandidates(published, "rtype_document").map((item) => item.slug),
		).toEqual(["organization", "project", "folder"]);
	});

	it("blocks a child below level five and past fifty types", () => {
		let deep = published;
		deep = addChild(deep, "rtype_document", "level_5");
		expect(addChildBlocker(deep, "rtype_document")).toBeUndefined();
		expect(addChildBlocker(deep, "level_5")).toMatch(/at most 5 levels/);
		const full = treeFromResponse([
			type("organization", []),
			...Array.from({ length: 50 }, (_, index) =>
				type(`type_${index}`, ["organization"]),
			),
		]);
		expect(addChildBlocker(full, full[0].key)).toMatch(/at most 50/);
	});

	it("reports names and slugs the draft can't publish", () => {
		const draft = addChild(published, "rtype_project", "draft_1");
		expect(draftProblems(draft)).toEqual([
			"Every type needs a name.",
			"A new type: Use lowercase letters, digits and underscores, starting with a letter (at most 49).",
		]);
	});
});

describe("resource type layout", () => {
	const layout = layoutTree(published);

	it("lays the tree out top down with organization alone on top", () => {
		const y = (key: string) => layout.get(key)?.y ?? 0;
		expect(y("rtype_organization")).toBeLessThan(y("rtype_project"));
		expect(y("rtype_project")).toBeLessThan(y("rtype_folder"));
		expect(y("rtype_folder")).toBeLessThan(y("rtype_document"));
	});

	it("moves along parents, children and rows", () => {
		expect(neighbour(published, layout, "rtype_folder", "up")).toBe(
			"rtype_project",
		);
		expect(neighbour(published, layout, "rtype_organization", "down")).toBe(
			"rtype_project",
		);
		expect(neighbour(published, layout, "rtype_organization", "left")).toBe(
			undefined,
		);
	});
});

describe("publish errors", () => {
	const nameOf = (slug: string) =>
		published.find((item) => item.slug === slug)?.name ?? slug;

	it("names the blocking records of a type in use", () => {
		expect(
			publishProblems(
				{
					errors: [
						{
							code: "RESOURCE_TYPE_IN_USE",
							message: "folder can't be deleted while other records use it.",
							metadata: {
								slug: "folder",
								blocked_by: { child_types: 0, roles: 3, permissions: 1 },
							},
						},
					],
				},
				nameOf,
			),
		).toEqual([
			{
				code: "RESOURCE_TYPE_IN_USE",
				message:
					"Folder can't be deleted: 3 roles are scoped to it, 1 permission is scoped to it.",
				roleIds: [],
			},
		]);
	});

	it("names the registered resources that block a delete", () => {
		const [problem] = publishProblems(
			{
				errors: [
					{
						code: "RESOURCE_TYPE_IN_USE",
						message: "document can't be deleted while other records use it.",
						metadata: {
							slug: "document",
							blocked_by: {
								child_types: 0,
								roles: 0,
								permissions: 0,
								resources: 2,
							},
						},
					},
				],
			},
			nameOf,
		);
		expect(problem.message).toBe(
			"Document can't be deleted: 2 registered resources use it.",
		);
	});

	it("explains a parent type that registered resources still sit under", () => {
		const [problem] = publishProblems(
			{
				errors: [
					{
						code: "RESOURCE_TYPE_IN_USE",
						message:
							"1 registered resource of type document sits under one of type project.",
						metadata: {
							slug: "document",
							parent_type: "project",
							blocked_by: { resources: 1 },
						},
					},
				],
			},
			nameOf,
		);
		expect(problem.message).toBe(
			"Document can't lose Project as a parent type: 1 registered resource sits under a Project. Move it first.",
		);
	});

	it("explains nesting that registered resources still use", () => {
		const [problem] = publishProblems(
			{
				errors: [
					{
						code: "RESOURCE_TYPE_IN_USE",
						message:
							"3 registered resources of type folder sit under one of type folder.",
						metadata: {
							slug: "folder",
							parent_type: "folder",
							blocked_by: { resources: 3 },
						},
					},
				],
			},
			nameOf,
		);
		expect(problem.message).toBe(
			"Folder can't stop nesting in itself: 3 registered resources sit under another Folder. Move them first.",
		);
	});

	it("keeps the roles that block a reshape", () => {
		const [problem] = publishProblems(
			{
				errors: [
					{
						code: "RESOURCE_TYPE_IN_USE",
						message: "Roles would hold permissions outside their scope.",
						metadata: { blocked_by: { roles: 1 }, role_ids: ["role_1"] },
					},
				],
			},
			nameOf,
		);
		expect(problem.roleIds).toEqual(["role_1"]);
		expect(problem.message).toMatch(/outside their scope/);
	});

	it("explains a cycle and a tree too deep with type names", () => {
		const problems = publishProblems(
			{
				errors: [
					{
						code: "RESOURCE_TYPE_CYCLE",
						message: "cycle",
						metadata: { chain: ["project", "folder", "project"] },
					},
					{
						code: "RESOURCE_TYPE_TOO_DEEP",
						message: "deep",
						metadata: { slug: "document", depth: 6 },
					},
				],
			},
			nameOf,
		);
		expect(problems.map((problem) => problem.message)).toEqual([
			"Types can't sit under each other (Project → Folder → Project). Only a type that nests in itself may sit under its own type.",
			"Document would sit at level 6. The tree is at most 5 levels deep, counting Organization.",
		]);
	});

	it("falls back to the server message and to a network failure", () => {
		expect(
			publishProblems(
				{ errors: [{ code: "VALIDATION_ERROR", message: "Too many" }] },
				nameOf,
			)[0].message,
		).toBe("Too many");
		expect(publishProblems(new TypeError("Failed to fetch"), nameOf)).toEqual([
			{ code: "UNEXPECTED_ERROR", message: "Failed to fetch", roleIds: [] },
		]);
	});
});
