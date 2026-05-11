import { routeGuard } from "@/lib/route-auth";
import ProductAPIKeysPage from "@/pages/product-api-keys";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const productApiKeysRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCT_API_KEYS,
	component: ProductAPIKeysPage,
	beforeLoad: routeGuard,
});
