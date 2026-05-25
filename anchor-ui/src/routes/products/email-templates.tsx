import { routeGuard } from "@/lib/route-auth";
import EmailTemplatesPage from "@/pages/email-templates";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const emailTemplatesRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.EMAIL_TEMPLATES,
	component: EmailTemplatesPage,
	beforeLoad: routeGuard,
});
