import type { ProductResourceTypeResponse } from "@/client";

export const ORGANIZATION_SLUG = "organization";
export const RESOURCE_TYPE_FILTER_LIMIT = 50;

export interface ScopeType {
	slug: string;
	name: string;
	level: number;
	parents: string[];
	builtIn: boolean;
	customRoles: boolean;
}

function levels(types: ProductResourceTypeResponse[]) {
	const bySlug = new Map(types.map((type) => [type.slug, type]));
	const memo = new Map<string, number>();
	const visit = (slug: string, path: Set<string>): number => {
		const known = memo.get(slug);
		if (known !== undefined) return known;
		const type = bySlug.get(slug);
		if (!type || path.has(slug)) return 1;
		path.add(slug);
		const parents = type.parent_types.filter(
			(parent) => parent !== slug && bySlug.has(parent),
		);
		const level = parents.length
			? 1 + Math.max(...parents.map((parent) => visit(parent, path)))
			: 1;
		path.delete(slug);
		memo.set(slug, level);
		return level;
	};
	for (const type of types) visit(type.slug, new Set());
	return memo;
}

/** Organization first, then each level of the tree from the top, in the order Anchor returns them. */
export function treeOrder(types: ProductResourceTypeResponse[]): ScopeType[] {
	const level = levels(types);
	return types
		.map((type, index) => ({ type, index }))
		.sort(
			(left, right) =>
				Number(right.type.built_in) - Number(left.type.built_in) ||
				(level.get(left.type.slug) ?? 1) - (level.get(right.type.slug) ?? 1) ||
				left.index - right.index,
		)
		.map(({ type }) => ({
			slug: type.slug,
			name: type.name,
			level: level.get(type.slug) ?? 1,
			parents: type.parent_types,
			builtIn: type.built_in,
			customRoles: type.custom_roles,
		}));
}

/** The slug and every type reachable downward from it: the types a role scoped to it may bundle permissions of. */
export function slugsBelow(types: ScopeType[], slug: string) {
	const found = new Set<string>([slug]);
	const queue = [slug];
	while (queue.length) {
		const current = queue.shift() as string;
		for (const type of types)
			if (type.parents.includes(current) && !found.has(type.slug)) {
				found.add(type.slug);
				queue.push(type.slug);
			}
	}
	return found;
}

export function typesInScope(types: ScopeType[], slug: string) {
	const below = slugsBelow(types, slug);
	return types.filter((type) => below.has(type.slug));
}

export function typesOutsideScope(types: ScopeType[], slug: string) {
	const below = slugsBelow(types, slug);
	return types.filter((type) => !below.has(type.slug));
}

/** The `resource_types` search filter for a scope, or none when the scope covers every type. */
export function scopeFilter(types: ScopeType[], slug: string) {
	const inScope = typesInScope(types, slug);
	if (inScope.length === types.length) return undefined;
	return inScope.map((type) => type.slug);
}

export function typeName(types: ScopeType[], slug: string) {
	return types.find((type) => type.slug === slug)?.name ?? slug;
}

export function parentNames(types: ScopeType[], type: ScopeType) {
	return type.parents.map((parent) => typeName(types, parent));
}

export function parseResourceTypeSearch(value: unknown): string[] {
	const raw = Array.isArray(value)
		? value
		: typeof value === "string"
			? value.split(",")
			: [];
	return [
		...new Set(
			raw
				.filter((item): item is string => typeof item === "string")
				.map((item) => item.trim())
				.filter((item) => /^[a-z][a-z0-9_]{0,48}$/.test(item)),
		),
	].slice(0, RESOURCE_TYPE_FILTER_LIMIT);
}

export function resourceTypeSearch(slugs: string[]) {
	return slugs.length ? slugs.join(",") : undefined;
}
