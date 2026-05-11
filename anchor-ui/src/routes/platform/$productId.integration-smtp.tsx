import { createRoute } from "@tanstack/react-router";
import SmtpIntegrationPage from "@/pages/integration-smtp";
import { routeGuard } from "@/lib/route-auth";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";

export const productIntegrationSmtpRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCT_INTEGRATION_SMTP,
	component: SmtpIntegrationPage,
	beforeLoad: routeGuard,
});
