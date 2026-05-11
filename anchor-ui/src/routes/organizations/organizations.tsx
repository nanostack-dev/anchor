import { routeGuard } from "@/lib/route-auth";
import OrganizationsPage from "@/pages/organizations";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const organizationsRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.ORGANIZATIONS,
	component: OrganizationsPage,
	beforeLoad: routeGuard,
});
