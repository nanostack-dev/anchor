import { routeGuard } from "@/lib/route-auth";
import WorkspacesPage from "@/pages/workspaces";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const workspacesRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.WORKSPACES,
	component: WorkspacesPage,
	beforeLoad: routeGuard,
});
