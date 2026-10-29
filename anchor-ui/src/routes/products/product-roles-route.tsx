import { parseResourceTypeSearch } from "@/components/product/resource-types/resource-type-scope";
import { routeGuard } from "@/lib/route-auth";
import ProductRolesPage from "@/pages/product-roles";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const productRolesRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCT_ROLES,
	beforeLoad: routeGuard,
	validateSearch: (
		search: Record<string, unknown>,
	): { resource_type?: string } => {
		const slugs = parseResourceTypeSearch(search.resource_type);
		return { resource_type: slugs.length ? slugs.join(",") : undefined };
	},
	component: function ProductRolesRoute() {
		const { resource_type } = productRolesRoute.useSearch();
		return (
			<ProductRolesPage
				resourceTypes={parseResourceTypeSearch(resource_type)}
			/>
		);
	},
});
