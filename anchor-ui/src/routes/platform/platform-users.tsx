import { routeGuard } from "@/lib/route-auth";
import PlatformUsersPage from "@/pages/platform-users/platform-users";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const platformUsersRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PLATFORM_USERS,
	component: PlatformUsersPage,
	beforeLoad: routeGuard,
});
