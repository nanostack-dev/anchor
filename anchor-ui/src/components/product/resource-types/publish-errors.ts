import type { ApiError } from "@/client";
import { getApiErrors, getErrorDetail } from "@/lib/api-error";
import { MAX_DEPTH, MAX_OWN_TYPES, plural } from "./resource-type-tree";

export interface PublishProblem {
	code: string;
	message: string;
	roleIds: string[];
}

interface BlockedBy {
	child_types?: number;
	roles?: number;
	permissions?: number;
}

function text(value: unknown) {
	return typeof value === "string" ? value : undefined;
}

function blockers(blocked: BlockedBy) {
	return [
		blocked.child_types &&
			`${plural(blocked.child_types, "type sits", "types sit")} under it`,
		blocked.roles &&
			`${plural(blocked.roles, "role is", "roles are")} scoped to it`,
		blocked.permissions &&
			`${plural(blocked.permissions, "permission is", "permissions are")} scoped to it`,
	].filter(Boolean);
}

function describe(error: ApiError, nameOf: (slug: string) => string) {
	const metadata = error.metadata ?? {};
	const slug = text(metadata.slug);
	const subject = slug ? nameOf(slug) : "A type";
	switch (error.code) {
		case "RESOURCE_TYPE_CYCLE": {
			const chain = Array.isArray(metadata.chain)
				? metadata.chain.map((item) => nameOf(String(item))).join(" → ")
				: undefined;
			return `Types can't sit under each other${chain ? ` (${chain})` : ""}. Only a type that nests in itself may sit under its own type.`;
		}
		case "RESOURCE_TYPE_TOO_DEEP":
			return `${subject} would sit at level ${metadata.depth ?? MAX_DEPTH + 1}. The tree is at most ${MAX_DEPTH} levels deep, counting Organization.`;
		case "RESOURCE_TYPE_IN_USE": {
			if (Array.isArray(metadata.role_ids))
				return "Some roles would hold permissions outside their scope with this tree. Remove those permissions from the roles first.";
			const reasons = blockers((metadata.blocked_by ?? {}) as BlockedBy);
			return reasons.length
				? `${subject} can't be deleted: ${reasons.join(", ")}.`
				: `${subject} can't be deleted while other records use it.`;
		}
		case "BUILT_IN_RESOURCE_TYPE":
			return "Organization is built in. It can't be renamed, moved or deleted.";
		case "RESOURCE_TYPE_SLUG_DUPLICATE":
		case "RESOURCE_TYPE_SLUG_EXISTS":
			return `Another type already uses the slug ${slug ?? "you chose"}.`;
		case "RESOURCE_TYPE_LIMIT_REACHED":
			return `A product declares at most ${MAX_OWN_TYPES} resource types besides Organization.`;
		case "RESOURCE_TYPE_PARENT_REQUIRED":
			return "Every type except Organization needs at least one parent type.";
		case "RESOURCE_TYPE_SLUG_INVALID":
			return `${slug ? `The slug ${slug}` : "A slug"} isn't valid. Use lowercase letters, digits and underscores, starting with a letter.`;
		case "RESOURCE_TYPE_SELF_PARENT":
			return `${subject} lists itself as a parent. Turn on "Nests in itself" instead.`;
		case "RESOURCE_TYPE_NOT_FOUND":
			return "The tree names a type that no longer exists. Discard your draft and try again.";
		default:
			return error.message;
	}
}

export function publishProblems(
	error: unknown,
	nameOf: (slug: string) => string,
): PublishProblem[] {
	const errors = getApiErrors(error);
	if (!errors.length)
		return [
			{
				code: "UNEXPECTED_ERROR",
				message:
					getErrorDetail(error) ?? "Anchor couldn't be reached. Try again.",
				roleIds: [],
			},
		];
	return errors.map((apiError) => ({
		code: apiError.code,
		message: describe(apiError, nameOf),
		roleIds: Array.isArray(apiError.metadata?.role_ids)
			? apiError.metadata.role_ids.map(String)
			: [],
	}));
}
