import { routeGuard } from "@/lib/route-auth";
import { InitPage } from "@/pages/init";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";
import { z } from "zod";

const searchSchema = z.object({
	redirect: z.string().optional(),
});

export const initRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.INIT,
	component: InitPage,
	validateSearch: (search) => {
		return searchSchema.parse(search);
	},
	beforeLoad: routeGuard,
});
