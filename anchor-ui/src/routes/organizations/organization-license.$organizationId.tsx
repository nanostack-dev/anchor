import { routeGuard } from "@/lib/route-auth";
import OrganizationLicenseDetailPage from "@/pages/organization-license-detail";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const organizationLicenseDetailRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.ORGANIZATION_LICENSE_DETAIL,
	component: OrganizationLicenseDetailPage,
	beforeLoad: routeGuard,
});
