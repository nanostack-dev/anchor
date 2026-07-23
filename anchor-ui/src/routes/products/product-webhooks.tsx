import { routeGuard } from "@/lib/route-auth";
import ProductWebhooksPage from "@/pages/product-webhooks";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const productWebhooksRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCT_WEBHOOKS,
	component: ProductWebhooksPage,
	beforeLoad: routeGuard,
});
