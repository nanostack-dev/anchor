import { routeGuard } from "@/lib/route-auth";
import PermissionsPage from "@/pages/permissions";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const productPermissionsRoute = createRoute({
	getParentRoute: () => rootRoute,
	component: PermissionsPage,
	path: ROUTE_PATHS.PRODUCT_PERMISSIONS,
	beforeLoad: routeGuard,
});
