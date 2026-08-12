import { routeGuard } from "@/lib/route-auth";
import LicenseSchemaPage from "@/pages/license-schema";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const licenseSchemaRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCT_LICENSE_SCHEMA,
	component: LicenseSchemaPage,
	beforeLoad: routeGuard,
});
