import { routeGuard } from "@/lib/route-auth";
import StripeIntegrationPage from "@/pages/integration-stripe";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const productIntegrationStripeRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCT_INTEGRATION_STRIPE,
	component: StripeIntegrationPage,
	beforeLoad: routeGuard,
});
