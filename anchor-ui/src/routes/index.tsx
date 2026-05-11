import { routeGuard } from "@/lib/route-auth";
import DashboardPage from "@/pages/index";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const indexRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.INDEX,
	component: DashboardPage,
	beforeLoad: routeGuard,
});
