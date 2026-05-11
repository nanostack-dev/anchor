import { routeGuard } from "@/lib/route-auth";
import OrganizationMembershipsPage from "@/pages/organization-memberships";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const organizationMembershipsRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.ORGANIZATION_MEMBERSHIPS,
	component: OrganizationMembershipsPage,
	beforeLoad: routeGuard,
});
