import { routeGuard } from "@/lib/route-auth";
import ProductResourceTypesPage from "@/pages/product-resource-types";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const productResourceTypesRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCT_RESOURCE_TYPES,
	component: ProductResourceTypesPage,
	beforeLoad: routeGuard,
});
