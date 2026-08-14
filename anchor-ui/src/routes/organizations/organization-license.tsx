import { routeGuard } from "@/lib/route-auth";
import OrganizationLicensePage from "@/pages/organization-license";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const organizationLicenseRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.ORGANIZATION_LICENSE,
	component: OrganizationLicensePage,
	beforeLoad: routeGuard,
});
