import { routeGuard } from "@/lib/route-auth";
import PlatformIntegrationsPage from "@/pages/platform-integrations";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const productIntegrationsRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCT_INTEGRATIONS,
	component: PlatformIntegrationsPage,
	beforeLoad: routeGuard,
});
