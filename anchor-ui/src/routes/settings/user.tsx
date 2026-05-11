import { routeGuard } from "@/lib/route-auth";
import UserSettingsPage from "@/pages/settings/user";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const settingsUserRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.SETTINGS_USER,
	component: UserSettingsPage,
	beforeLoad: routeGuard,
});
