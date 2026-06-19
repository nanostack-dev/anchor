import { routeGuard } from "@/lib/route-auth";
import ProductApiKeyFormPage from "@/pages/product-api-key-form";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const productApiKeyNewRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCT_API_KEY_NEW,
	component: () => <ProductApiKeyFormPage mode="create" />,
	beforeLoad: routeGuard,
});
