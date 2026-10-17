import { routeGuard } from "@/lib/route-auth";
import PricingPage from "@/pages/pricing";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const productPricingRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCT_PRICING,
	component: PricingPage,
	beforeLoad: routeGuard,
});
