import {
	type ProductResourcePermissionResponse,
	type ProductRoleResponse,
	SortDirection,
	searchProductResourcePermissions,
	searchProductRoles,
} from "@/client";
import { queryOptions, useQuery } from "@tanstack/react-query";

const PAGE_SIZE = 100;

/**
 * Every permission scoped to the given resource types, or every permission
 * when `resourceTypes` is undefined. It follows `next_cursor` to the last page.
 * The key keeps the `searchProductResourcePermissions` id, so invalidating
 * that id refreshes it with the tables.
 */
export function scopedPermissionsOptions(
	productId: string,
	resourceTypes: string[] | undefined,
) {
	return queryOptions({
		queryKey: [
			{
				_id: "searchProductResourcePermissions",
				path: { product_id: productId },
				resourceTypes: resourceTypes ?? "all",
				scope: "all-pages",
			},
		] as const,
		queryFn: async ({ signal }) => {
			const items: ProductResourcePermissionResponse[] = [];
			let cursor: string | undefined;
			do {
				const { data } = await searchProductResourcePermissions({
					path: { product_id: productId },
					body: {
						pagination: { limit: PAGE_SIZE, cursor },
						sort_by: "name",
						sort_direction: SortDirection.ASC,
						filter: resourceTypes
							? { resource_types: resourceTypes }
							: undefined,
					},
					signal,
					throwOnError: true,
				});
				items.push(...data.items);
				cursor = data.next_cursor;
			} while (cursor);
			return items;
		},
	});
}

export function useScopedPermissions(
	productId: string,
	resourceTypes: string[] | undefined,
	enabled = true,
) {
	return useQuery({
		...scopedPermissionsOptions(productId, resourceTypes),
		enabled,
	});
}

/**
 * The role templates that bundle each of `permissionNames`, from one roles
 * search filtered on those names (followed to its last page), keyed by the
 * permission name in lower case: names are unique regardless of case.
 */
export function rolesBundlingOptions(
	productId: string,
	permissionNames: string[],
) {
	return queryOptions({
		queryKey: [
			{
				_id: "searchProductRoles",
				path: { product_id: productId },
				permissions: permissionNames,
				scope: "all-pages",
			},
		] as const,
		queryFn: async ({ signal }) => {
			const roles: ProductRoleResponse[] = [];
			let cursor: string | undefined;
			do {
				const { data } = await searchProductRoles({
					path: { product_id: productId },
					body: {
						pagination: { limit: PAGE_SIZE, cursor },
						sort_by: "name",
						sort_direction: SortDirection.ASC,
						filter: { permissions: permissionNames },
					},
					signal,
					throwOnError: true,
				});
				roles.push(...data.items);
				cursor = data.next_cursor;
			} while (cursor);
			const wanted = new Set(permissionNames.map((name) => name.toLowerCase()));
			const byPermission = new Map<string, ProductRoleResponse[]>();
			for (const role of roles)
				for (const grant of role.permissions) {
					const key = grant.permission_name.toLowerCase();
					if (!wanted.has(key)) continue;
					byPermission.set(key, [...(byPermission.get(key) ?? []), role]);
				}
			return byPermission;
		},
	});
}

export function useRolesBundling(productId: string, permissionNames: string[]) {
	return useQuery({
		...rolesBundlingOptions(productId, permissionNames),
		enabled: permissionNames.length > 0,
	});
}
