import { createRoute } from "@tanstack/react-router";

import { routeGuard } from "@/lib/route-auth";
import OrganizationApiKeysPage from "@/pages/organization-api-keys";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";

export const organizationApiKeysRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.ORGANIZATIONS_APIS_KEYS,
	component: OrganizationApiKeysPage,
	beforeLoad: routeGuard,
});
