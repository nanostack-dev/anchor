import { routeGuard } from "@/lib/route-auth";
import EmailSendsPage from "@/pages/email-sends";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const emailSendsRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.EMAIL_SENDS,
	component: EmailSendsPage,
	beforeLoad: routeGuard,
});
