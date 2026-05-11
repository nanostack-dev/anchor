import { routeGuard } from "@/lib/route-auth";
import { RegisterPage } from "@/pages/register";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";
import { z } from "zod";

const searchSchema = z.object({
	invitationCode: z.string().optional(),
	tenantId: z.string().optional(),
	email: z.string().email().optional(),
	redirect: z.string().optional(),
});
export const registerRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.REGISTER,
	component: RegisterPage,
	validateSearch: (search) => {
		return searchSchema.parse(search);
	},
	beforeLoad: routeGuard,
});
