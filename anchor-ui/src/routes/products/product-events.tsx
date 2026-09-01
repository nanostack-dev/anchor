import { routeGuard } from "@/lib/route-auth";
import ProductEventsPage from "@/pages/product-events";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const productEventsRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCT_EVENTS,
	component: ProductEventsPage,
	beforeLoad: routeGuard,
});
