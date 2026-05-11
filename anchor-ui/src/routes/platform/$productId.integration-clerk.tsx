import { routeGuard } from "@/lib/route-auth";
import ClerkIntegrationPage from "@/pages/integration-clerk";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const productIntegrationClerkRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCT_INTEGRATION_CLERK,
	component: ClerkIntegrationPage,
	beforeLoad: routeGuard,
});
