import { routeGuard } from "@/lib/route-auth";
import ProductUsersPage from "@/pages/product-users";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const productUsersRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCT_USERS,
	component: ProductUsersPage,
	beforeLoad: routeGuard,
});
