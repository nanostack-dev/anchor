import { routeGuard } from "@/lib/route-auth";
import ProductApiKeyFormPage from "@/pages/product-api-key-form";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const productApiKeyEditRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCT_API_KEY_EDIT,
	component: () => {
		const { apiKeyId } = productApiKeyEditRoute.useParams();
		return <ProductApiKeyFormPage mode="edit" apiKeyId={apiKeyId} />;
	},
	beforeLoad: routeGuard,
});
