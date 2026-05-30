import { routeGuard } from "@/lib/route-auth";
import SmtpIntegrationPage from "@/pages/integration-smtp";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const productIntegrationSmtpRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCT_INTEGRATION_SMTP,
	component: SmtpIntegrationPage,
	beforeLoad: routeGuard,
});
