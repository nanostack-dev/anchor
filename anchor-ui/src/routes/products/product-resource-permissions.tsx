import { routeGuard } from "@/lib/route-auth";
import ProductResourcePermissionsPage from "@/pages/resource-permissions";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const productResourcePermissionsRoute = createRoute({
	getParentRoute: () => rootRoute,
	component: ProductResourcePermissionsPage,
	path: ROUTE_PATHS.PRODUCT_RESOURCES_PERMISSIONS,
	beforeLoad: routeGuard,
});
