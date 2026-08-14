import { routeGuard } from "@/lib/route-auth";
import LicenseTemplatesPage from "@/pages/license-templates";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const licenseTemplatesRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCT_LICENSE_TEMPLATES,
	component: LicenseTemplatesPage,
	beforeLoad: routeGuard,
});
