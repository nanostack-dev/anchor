import { routeGuard } from "@/lib/route-auth";
import ProductLicensesPage from "@/pages/product-licenses";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const productLicensesRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCT_LICENSES,
	component: ProductLicensesPage,
	beforeLoad: routeGuard,
});
