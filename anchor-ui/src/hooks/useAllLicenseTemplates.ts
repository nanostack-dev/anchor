import {
	type LicenseTemplateResponse,
	SortDirection,
	searchLicenseTemplates,
} from "@/client";
import { queryOptions } from "@tanstack/react-query";

const TEMPLATE_PAGE_SIZE = 100;

export function allLicenseTemplatesQueryKey(productId: string) {
	return [
		{
			_id: "searchLicenseTemplates",
			path: { product_id: productId },
			scope: "all-pages",
		},
	] as const;
}

/**
 * Every license template of a product, for the places that look one up by id or
 * offer them all in a picker. It follows `next_cursor` to the last page, so a
 * product with more templates than a page still lists every one of them.
 * Its key keeps the `searchLicenseTemplates` id, so invalidating that id
 * refreshes this list together with the table.
 */
export function allLicenseTemplatesOptions(productId: string) {
	return queryOptions({
		queryKey: allLicenseTemplatesQueryKey(productId),
		queryFn: async ({ signal }) => {
			const items: LicenseTemplateResponse[] = [];
			let cursor: string | undefined;
			do {
				const { data } = await searchLicenseTemplates({
					path: { product_id: productId },
					body: {
						pagination: { limit: TEMPLATE_PAGE_SIZE, cursor },
						sort_by: "name",
						sort_direction: SortDirection.ASC,
					},
					signal,
					throwOnError: true,
				});
				items.push(...data.items);
				cursor = data.next_cursor;
			} while (cursor);
			return { items };
		},
	});
}
