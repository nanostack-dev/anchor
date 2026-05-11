import { routeGuard } from "@/lib/route-auth";
import PlatformInvitationsPage from "@/pages/platform-users/platform-invitations";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const platformInvitationsRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PLATFORM_INVITATIONS,
	component: PlatformInvitationsPage,
	beforeLoad: routeGuard,
});
