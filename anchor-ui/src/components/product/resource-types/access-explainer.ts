import {
	type AccessDecisionContext,
	AccessDenyReason,
	type AccessGrant,
	type ResourceParentRef,
	type ResourceResponse,
} from "@/client";

import type { TreeType } from "./resource-type-tree";

export const ORGANIZATION = "organization";
export const MAX_CHAIN = 15;

export interface ChainLink {
	type: string;
	id: string;
	name: string;
}

export interface Chain {
	links: ChainLink[];
	complete: boolean;
}

export interface CheckQuestion {
	organization: { id: string; name: string };
	member: { label: string; externalId: string };
	permission: { name: string; scope: string };
	resource: ResourceResponse | null;
}

export interface Grant {
	id: string;
	holder: string;
	kind: "role" | "api_key";
	link: ChainLink;
}

export type Explanation =
	// grants is null when Anchor couldn't read what granted the access.
	| { decision: "allowed"; grants: Grant[] | null }
	| {
			decision: "denied";
			reason: string;
			title: string;
			detail: string;
	  };

export interface AccessHighlight {
	decision: "allowed" | "denied";
	path: string[];
	steps: [parent: string, child: string][];
	granted: Record<string, string[]>;
}

export function organizationLink(organization: {
	id: string;
	name: string;
}): ChainLink {
	return { type: ORGANIZATION, id: organization.id, name: organization.name };
}

export function resourceLink(resource: ResourceResponse): ChainLink {
	return { type: resource.type, id: resource.id, name: resource.name };
}

export function subjectLink(question: CheckQuestion): ChainLink {
	return question.resource
		? resourceLink(question.resource)
		: organizationLink(question.organization);
}

export async function walkChain(
	question: CheckQuestion,
	fetchParent: (parent: ResourceParentRef) => Promise<ResourceResponse>,
): Promise<Chain> {
	const root = organizationLink(question.organization);
	if (!question.resource) return { links: [root], complete: true };
	const links: ChainLink[] = [resourceLink(question.resource)];
	let parent = question.resource.parent;
	while (parent.type !== ORGANIZATION && links.length < MAX_CHAIN - 1) {
		const next = await fetchParent(parent);
		links.push(resourceLink(next));
		parent = next.parent;
	}
	const complete = parent.type === ORGANIZATION;
	return { links: complete ? [...links, root] : links, complete };
}

const sameLink = (
	link: { type: string; id: string },
	other: { type: string; id: string },
) => link.type === other.type && link.id === other.id;

export function grantsOf(
	granted: AccessGrant[] | undefined,
	chain: ChainLink[],
): Grant[] | null {
	if (!granted) return null;
	return granted.map((grant) => ({
		id: grant.role?.id ?? grant.api_key?.id ?? "",
		holder: grant.role?.name ?? grant.api_key?.name ?? grant.role?.id ?? "",
		kind: grant.role ? "role" : "api_key",
		link: chain.find((link) => sameLink(link, grant.resource)) ?? {
			...grant.resource,
			name: grant.resource.id,
		},
	}));
}

export function typeName(types: TreeType[], slug: string) {
	return types.find((type) => type.slug === slug)?.name ?? slug;
}

export function heldOnTypes(types: TreeType[], scope: string) {
	const byKey = new Map(types.map((type) => [type.key, type]));
	const held = new Set<string>();
	const queue = types.filter((type) => type.slug === scope);
	if (!queue.length) held.add(scope);
	while (queue.length) {
		const type = queue.shift() as TreeType;
		if (held.has(type.slug)) continue;
		held.add(type.slug);
		for (const parent of type.parents) {
			const found = byKey.get(parent);
			if (found) queue.push(found);
		}
	}
	return held;
}

interface DenyContext {
	member: string;
	organization: string;
	permission: string;
	resource: string;
	resourceType: string;
	scopeType: string;
	heldOnResource: boolean;
}

function denyCopy(
	reason: string,
	context: DenyContext,
): { title: string; detail: string } {
	const { member, organization, permission, resource } = context;
	switch (reason) {
		case AccessDenyReason.MISSING_PERMISSION:
			if (!context.heldOnResource)
				return {
					title: `${permission} doesn't apply to ${context.resourceType} resources.`,
					detail: `A permission is held on its own type, ${context.scopeType}, and on the types above it. Choose a permission of ${context.resourceType} or of a type below it.`,
				};
			return {
				title: `No role ${member} holds on ${resource} or above it grants ${permission}.`,
				detail:
					"Roles reach down the tree, never up: a role held on something below this resource doesn't count here.",
			};
		case AccessDenyReason.NOT_A_MEMBER:
			return {
				title: `${member} isn't a member of ${organization}.`,
				detail:
					"Roles count only inside an Organization the member belongs to.",
			};
		case AccessDenyReason.SUBJECT_NOT_SYNCED:
			return {
				title: `Anchor knows no product user with ${member}'s external ID.`,
				detail:
					"A product user gets its external ID when the product creates it or when an identity integration syncs it.",
			};
		case AccessDenyReason.ORGANIZATION_NOT_FOUND:
			return {
				title: `${organization} doesn't exist in this product.`,
				detail: "It may have been deleted since you chose it.",
			};
		case AccessDenyReason.RESOURCE_TYPE_NOT_FOUND:
			return {
				title: `This product declares no ${context.resourceType} resource type.`,
				detail: "The type may have been removed since you chose the resource.",
			};
		case AccessDenyReason.RESOURCE_NOT_FOUND:
			return {
				title: `${resource} isn't registered in ${organization}.`,
				detail: "It may have been deleted since you chose it.",
			};
		case AccessDenyReason.RESOURCE_NOT_IN_ORGANIZATION:
			return {
				title: `${resource} belongs to another Organization.`,
				detail:
					"Anchor checks a resource only in the Organization that holds it.",
			};
		case AccessDenyReason.PARENT_TYPE_NOT_ALLOWED:
			return {
				title: `${resource} sits under a type it can't sit under.`,
				detail:
					"Its parent's type isn't one of its type's parent types. The resource type tree may have changed.",
			};
		case AccessDenyReason.API_KEY_INVALID:
			return {
				title: "No Organization API key has this value.",
				detail: "Check the key and try again.",
			};
		case AccessDenyReason.API_KEY_INACTIVE:
			return {
				title: "The Organization API key is inactive or expired.",
				detail: "Activate it or use another key.",
			};
		case AccessDenyReason.NOT_EVALUATED:
			return {
				title: "Anchor skipped this check.",
				detail: "An earlier answer in the same batch decided the outcome.",
			};
		default:
			return {
				title: "Anchor denied access.",
				detail: "Anchor gave a reason this page doesn't describe yet.",
			};
	}
}

export function explain(
	decision: { decision: boolean; context: AccessDecisionContext },
	question: CheckQuestion,
	chain: ChainLink[],
	types: TreeType[],
): Explanation {
	if (decision.decision)
		return {
			decision: "allowed",
			grants: grantsOf(decision.context.granted_by, chain),
		};
	const subject = subjectLink(question);
	const reason = decision.context.reason ?? "UNKNOWN";
	return {
		decision: "denied",
		reason,
		...denyCopy(reason, {
			member: question.member.label,
			organization: question.organization.name,
			permission: question.permission.name,
			resource: subject.name,
			resourceType: typeName(types, subject.type),
			scopeType: typeName(types, question.permission.scope),
			heldOnResource: heldOnTypes(types, question.permission.scope).has(
				subject.type,
			),
		}),
	};
}

export function highlightOf(
	explanation: Explanation,
	chain: ChainLink[],
): AccessHighlight {
	const path = [...new Set(chain.map((link) => link.type))];
	const steps: [string, string][] = [];
	for (let index = 1; index < chain.length; index += 1) {
		const child = chain[index - 1].type;
		const parent = chain[index].type;
		if (
			parent !== child &&
			!steps.some(([from, to]) => from === parent && to === child)
		)
			steps.push([parent, child]);
	}
	const granted: Record<string, string[]> = {};
	const seen = new Set<string>();
	if (explanation.decision === "allowed")
		for (const grant of explanation.grants ?? []) {
			const key = `${grant.link.type}:${grant.id}`;
			if (!seen.has(key)) {
				seen.add(key);
				granted[grant.link.type] = [
					...(granted[grant.link.type] ?? []),
					grant.holder,
				];
			}
			if (!path.includes(grant.link.type)) path.push(grant.link.type);
		}
	return { decision: explanation.decision, path, steps, granted };
}
