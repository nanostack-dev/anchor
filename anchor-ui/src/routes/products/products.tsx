import { routeGuard } from "@/lib/route-auth";
import ProductsPage from "@/pages/products";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const productsRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCTS,
	component: ProductsPage,
	beforeLoad: routeGuard,
});
