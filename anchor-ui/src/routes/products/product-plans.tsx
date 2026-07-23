import { routeGuard } from "@/lib/route-auth";
import ProductPlansPage from "@/pages/product-plans";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const productPlansRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCT_PLANS,
	component: ProductPlansPage,
	beforeLoad: routeGuard,
});
