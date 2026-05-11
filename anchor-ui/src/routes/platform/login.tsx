import { routeGuard } from "@/lib/route-auth";
import { LoginPage } from "@/pages/login";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";
import { z } from "zod";

const searchSchema = z.object({
	redirect: z.string().optional(),
});

export const loginRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.LOGIN,
	component: LoginPage,
	validateSearch: (search) => {
		return searchSchema.parse(search);
	},
	beforeLoad: routeGuard,
});
