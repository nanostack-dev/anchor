import { routeGuard } from "@/lib/route-auth";
import WorkspaceMembershipsPage from "@/pages/workspace-memberships";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const workspaceMembershipsRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.WORKSPACE_MEMBERSHIPS,
	component: WorkspaceMembershipsPage,
	beforeLoad: routeGuard,
});
