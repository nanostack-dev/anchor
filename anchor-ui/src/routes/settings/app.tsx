import { routeGuard } from "@/lib/route-auth";
import AppSettingsPage from "@/pages/settings/app";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const settingsAppRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.SETTINGS_APP,
	component: AppSettingsPage,
	beforeLoad: routeGuard,
});
