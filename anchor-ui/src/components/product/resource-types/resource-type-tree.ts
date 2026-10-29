import type {
	ProductResourceTypeResponse,
	ProductResourceTypeTreeItem,
} from "@/client";

export const MAX_DEPTH = 5;
export const MAX_OWN_TYPES = 50;
export const SLUG_PATTERN = /^[a-z][a-z0-9_]{0,48}$/;
const SLUG_LENGTH = 49;

export interface TreeType {
	key: string;
	slug: string;
	name: string;
	description?: string | null;
	parents: string[];
	nestsInItself: boolean;
	customRoles: boolean;
	builtIn: boolean;
	published: boolean;
	slugEdited: boolean;
}

export function treeFromResponse(
	types: ProductResourceTypeResponse[],
): TreeType[] {
	const keyBySlug = new Map(types.map((type) => [type.slug, type.id]));
	const ordered = [
		...types.filter((type) => type.built_in),
		...types.filter((type) => !type.built_in),
	];
	return ordered.map((type) => ({
		key: type.id,
		slug: type.slug,
		name: type.name,
		description: type.description,
		parents: type.parent_types.flatMap((slug) => {
			const key = keyBySlug.get(slug);
			return key ? [key] : [];
		}),
		nestsInItself: type.nests_in_itself,
		customRoles: type.allows_custom_roles,
		builtIn: type.built_in,
		published: true,
		slugEdited: false,
	}));
}

export function treeRequest(types: TreeType[]): ProductResourceTypeTreeItem[] {
	const slugByKey = new Map(types.map((type) => [type.key, type.slug]));
	return types
		.filter((type) => !type.builtIn)
		.map((type) => ({
			slug: type.slug,
			name: type.name.trim(),
			description: type.description ?? undefined,
			parent_types: type.parents.flatMap((key) => {
				const slug = slugByKey.get(key);
				return slug ? [slug] : [];
			}),
			nests_in_itself: type.nestsInItself,
		}));
}

export function childrenOf(types: TreeType[], key: string) {
	return types.filter((type) => type.key !== key && type.parents.includes(key));
}

export function depths(types: TreeType[]) {
	const byKey = new Map(types.map((type) => [type.key, type]));
	const memo = new Map<string, number>();
	const visit = (key: string, path: Set<string>): number => {
		const known = memo.get(key);
		if (known !== undefined) return known;
		const type = byKey.get(key);
		if (!type || path.has(key)) return 1;
		path.add(key);
		const parents = type.parents.filter((parent) => parent !== key);
		const depth = parents.length
			? 1 + Math.max(...parents.map((parent) => visit(parent, path)))
			: 1;
		path.delete(key);
		memo.set(key, depth);
		return depth;
	};
	for (const type of types) visit(type.key, new Set());
	return memo;
}

export function descendants(types: TreeType[], key: string) {
	const found = new Set<string>();
	const queue = [key];
	while (queue.length) {
		const current = queue.shift() as string;
		for (const child of childrenOf(types, current))
			if (!found.has(child.key)) {
				found.add(child.key);
				queue.push(child.key);
			}
	}
	return found;
}

function levelsBelow(
	types: TreeType[],
	key: string,
	seen = new Set<string>(),
): number {
	if (seen.has(key)) return 0;
	seen.add(key);
	const children = childrenOf(types, key);
	const below = children.length
		? 1 +
			Math.max(...children.map((child) => levelsBelow(types, child.key, seen)))
		: 0;
	seen.delete(key);
	return below;
}

export function ownTypeCount(types: TreeType[]) {
	return types.filter((type) => !type.builtIn).length;
}

export function addChildBlocker(types: TreeType[], key: string) {
	if (ownTypeCount(types) >= MAX_OWN_TYPES)
		return `A product declares at most ${MAX_OWN_TYPES} resource types besides Organization.`;
	if ((depths(types).get(key) ?? 1) >= MAX_DEPTH)
		return `The tree is at most ${MAX_DEPTH} levels deep, counting Organization.`;
	return undefined;
}

export function parentCandidates(types: TreeType[], key: string) {
	const type = types.find((candidate) => candidate.key === key);
	if (!type || type.builtIn) return [];
	const below = descendants(types, key);
	const height = levelsBelow(types, key);
	const level = depths(types);
	return types.filter(
		(candidate) =>
			candidate.key !== key &&
			!below.has(candidate.key) &&
			(type.parents.includes(candidate.key) ||
				(level.get(candidate.key) ?? 1) + 1 + height <= MAX_DEPTH),
	);
}

export function slugify(name: string, taken: Set<string>) {
	const base =
		name
			.normalize("NFKD")
			.replace(/\p{M}/gu, "")
			.toLowerCase()
			.replace(/[^a-z0-9]+/g, "_")
			.replace(/^[^a-z]+/, "")
			.replace(/_+$/, "")
			.slice(0, SLUG_LENGTH)
			.replace(/_+$/, "") || "type";
	if (!taken.has(base)) return base;
	for (let suffix = 2; ; suffix++) {
		const tail = `_${suffix}`;
		const candidate = `${base.slice(0, SLUG_LENGTH - tail.length)}${tail}`;
		if (!taken.has(candidate)) return candidate;
	}
}

export function takenSlugs(types: TreeType[], except?: string) {
	return new Set(
		types.filter((type) => type.key !== except).map((type) => type.slug),
	);
}

export function addChild(
	types: TreeType[],
	parentKey: string,
	key: string,
): TreeType[] {
	return [
		...types,
		{
			key,
			slug: "",
			name: "",
			parents: [parentKey],
			nestsInItself: false,
			customRoles: false,
			builtIn: false,
			published: false,
			slugEdited: false,
		},
	];
}

export function updateType(
	types: TreeType[],
	key: string,
	change: (type: TreeType) => TreeType,
) {
	return types.map((type) => (type.key === key ? change(type) : type));
}

export function rename(
	types: TreeType[],
	key: string,
	name: string,
	reserved: ReadonlySet<string> = new Set(),
) {
	return updateType(types, key, (type) => ({
		...type,
		name,
		slug:
			type.published || type.slugEdited || type.builtIn
				? type.slug
				: slugify(name, new Set([...takenSlugs(types, key), ...reserved])),
	}));
}

/** Slugs of published types the draft deletes; Anchor matches by slug, so reusing one would keep the old type. */
export function deletedSlugs(published: TreeType[], draft: TreeType[]) {
	const kept = new Set(draft.map((type) => type.key));
	return new Set(
		published.filter((type) => !kept.has(type.key)).map((type) => type.slug),
	);
}

export function setSlug(types: TreeType[], key: string, slug: string) {
	return updateType(types, key, (type) =>
		type.published || type.builtIn ? type : { ...type, slug, slugEdited: true },
	);
}

export function toggleParent(
	types: TreeType[],
	key: string,
	parentKey: string,
) {
	return updateType(types, key, (type) => {
		if (type.parents.includes(parentKey))
			return type.parents.length > 1
				? {
						...type,
						parents: type.parents.filter((parent) => parent !== parentKey),
					}
				: type;
		return { ...type, parents: [...type.parents, parentKey] };
	});
}

export function removeType(types: TreeType[], key: string) {
	return types.filter((type) => type.key !== key);
}

export function slugProblem(
	types: TreeType[],
	type: TreeType,
	reserved: ReadonlySet<string> = new Set(),
) {
	if (!SLUG_PATTERN.test(type.slug))
		return "Use lowercase letters, digits and underscores, starting with a letter (at most 49).";
	if (takenSlugs(types, type.key).has(type.slug))
		return `Another type already uses the slug ${type.slug}.`;
	if (!type.published && reserved.has(type.slug))
		return `${type.slug} belongs to a type this draft deletes. Choose another slug, or publish the deletion first.`;
	return undefined;
}

export function draftProblems(
	types: TreeType[],
	reserved: ReadonlySet<string> = new Set(),
) {
	const problems: string[] = [];
	for (const type of types) {
		if (type.builtIn) continue;
		const label = type.name.trim() || "A new type";
		if (!type.name.trim()) problems.push("Every type needs a name.");
		const slug = slugProblem(types, type, reserved);
		if (slug) problems.push(`${label}: ${slug}`);
	}
	return [...new Set(problems)];
}

export interface DraftSummary {
	added: number;
	renamed: number;
	changed: number;
	deleted: number;
	switched: number;
}

const sameSet = (left: string[], right: string[]) =>
	left.length === right.length && left.every((item) => right.includes(item));

export function summarizeDraft(
	published: TreeType[],
	draft: TreeType[],
): DraftSummary {
	const before = new Map(published.map((type) => [type.key, type]));
	const after = new Set(draft.map((type) => type.key));
	const summary = { added: 0, renamed: 0, changed: 0, deleted: 0, switched: 0 };
	for (const type of draft) {
		const previous = before.get(type.key);
		if (!previous) {
			summary.added++;
			continue;
		}
		if (previous.name !== type.name.trim()) summary.renamed++;
		if (
			previous.nestsInItself !== type.nestsInItself ||
			!sameSet(previous.parents, type.parents)
		)
			summary.changed++;
		if (previous.customRoles !== type.customRoles) summary.switched++;
	}
	for (const type of published) if (!after.has(type.key)) summary.deleted++;
	return summary;
}

export function hasChanges(summary: DraftSummary) {
	return (
		summary.added +
			summary.renamed +
			summary.changed +
			summary.deleted +
			summary.switched >
		0
	);
}

export function describeDraft(summary: DraftSummary) {
	const parts = [
		summary.added &&
			`${summary.added} ${summary.added === 1 ? "type" : "types"} added`,
		summary.renamed && `${summary.renamed} renamed`,
		summary.changed && `${summary.changed} changed`,
		summary.deleted && `${summary.deleted} deleted`,
		summary.switched &&
			`custom roles switched on ${summary.switched} ${summary.switched === 1 ? "type" : "types"}`,
	].filter(Boolean);
	return parts.length ? parts.join(", ") : "No changes yet";
}

export function plural(count: number, one: string, many: string) {
	return `${count} ${count === 1 ? one : many}`;
}

/** The published types whose custom roles switch the draft changes: Anchor saves the switch per type, outside the tree. */
export function switchChanges(published: TreeType[], draft: TreeType[]) {
	const before = new Map(published.map((type) => [type.key, type]));
	return draft.filter((type) => {
		const previous = before.get(type.key);
		return previous && previous.customRoles !== type.customRoles;
	});
}

/** True when the draft changes the tree itself, not only custom roles switches. */
export function treeChanged(summary: DraftSummary) {
	return (
		summary.added + summary.renamed + summary.changed + summary.deleted > 0
	);
}
