import type { ProductResourceTypeResponse } from "@/client";
import { describe, expect, it } from "vitest";

import {
	parseResourceTypeSearch,
	resourceTypeSearch,
	scopeFilter,
	slugsBelow,
	treeOrder,
	typesInScope,
	typesOutsideScope,
} from "./resource-type-scope";

function type(
	slug: string,
	parents: string[] = ["organization"],
): ProductResourceTypeResponse {
	return {
		id: `rtype_${slug}`,
		product_id: "prd_1",
		slug,
		name: slug[0].toUpperCase() + slug.slice(1),
		parent_types: slug === "organization" ? [] : parents,
		nests_in_itself: false,
		allows_custom_roles: false,
		built_in: slug === "organization",
		created_at: "2026-10-24T09:12:31Z",
		updated_at: "2026-10-24T09:12:31Z",
	};
}

const documentTree = [
	type("organization"),
	type("document", ["folder", "project"]),
	type("project"),
	type("folder", ["project"]),
	type("team"),
];

describe("treeOrder", () => {
	it("puts Organization first, then each level from the top in Anchor's order", () => {
		const ordered = treeOrder(documentTree);
		expect(ordered.map((item) => `${item.slug}:${item.level}`)).toEqual([
			"organization:1",
			"project:2",
			"team:2",
			"folder:3",
			"document:4",
		]);
	});

	it("keeps Organization first when Anchor returns it later", () => {
		const ordered = treeOrder([type("project"), type("organization")]);
		expect(ordered[0].slug).toBe("organization");
	});
});

describe("scope", () => {
	const ordered = treeOrder(documentTree);

	it("covers the type and every type below it through any parent", () => {
		expect([...slugsBelow(ordered, "project")].sort()).toEqual([
			"document",
			"folder",
			"project",
		]);
		expect(typesInScope(ordered, "folder").map((item) => item.slug)).toEqual([
			"folder",
			"document",
		]);
	});

	it("leaves out the types above and beside the scope", () => {
		expect(
			typesOutsideScope(ordered, "folder").map((item) => item.slug),
		).toEqual(["organization", "project", "team"]);
	});

	it("sends no filter for Organization, which covers every type", () => {
		expect(scopeFilter(ordered, "organization")).toBeUndefined();
		expect(scopeFilter(ordered, "document")).toEqual(["document"]);
	});

	it("covers itself only for a leaf type", () => {
		expect(typesInScope(ordered, "team").map((item) => item.slug)).toEqual([
			"team",
		]);
	});
});

describe("resource type search parameter", () => {
	it("reads a comma-separated list of valid slugs", () => {
		expect(parseResourceTypeSearch("folder,document,folder")).toEqual([
			"folder",
			"document",
		]);
	});

	it("drops invalid slugs and anything that is not text", () => {
		expect(parseResourceTypeSearch("Folder,9x,ok_type")).toEqual(["ok_type"]);
		expect(parseResourceTypeSearch(42)).toEqual([]);
	});

	it("keeps at most 50 slugs, the search filter limit", () => {
		const many = Array.from({ length: 55 }, (_, index) => `t${index}`);
		expect(parseResourceTypeSearch(many.join(","))).toHaveLength(50);
	});

	it("writes nothing for an empty filter", () => {
		expect(resourceTypeSearch([])).toBeUndefined();
		expect(resourceTypeSearch(["a", "b"])).toBe("a,b");
	});
});
