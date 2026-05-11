import { routeGuard } from "@/lib/route-auth";
import ProductRolesPage from "@/pages/product-roles";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const productRolesRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCT_ROLES,
	component: ProductRolesPage,
	beforeLoad: routeGuard,
});
