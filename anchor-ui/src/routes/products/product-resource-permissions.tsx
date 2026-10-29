import { parseResourceTypeSearch } from "@/components/product/resource-types/resource-type-scope";
import { routeGuard } from "@/lib/route-auth";
import ProductResourcePermissionsPage from "@/pages/resource-permissions";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const productResourcePermissionsRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCT_RESOURCES_PERMISSIONS,
	beforeLoad: routeGuard,
	validateSearch: (
		search: Record<string, unknown>,
	): { resource_type?: string } => {
		const slugs = parseResourceTypeSearch(search.resource_type);
		return { resource_type: slugs.length ? slugs.join(",") : undefined };
	},
	component: function ProductResourcePermissionsRoute() {
		const { resource_type } = productResourcePermissionsRoute.useSearch();
		return (
			<ProductResourcePermissionsPage
				resourceTypes={parseResourceTypeSearch(resource_type)}
			/>
		);
	},
});
