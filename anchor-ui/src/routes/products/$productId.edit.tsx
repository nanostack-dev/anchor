import { getProductOptions } from "@/client/@tanstack/react-query.gen";
import { routeGuard } from "@/lib/route-auth";
import ProductEditPage from "@/pages/product-edit";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const productEditRoute = createRoute({
	getParentRoute: () => rootRoute,
	component: ProductEditPage,
	path: ROUTE_PATHS.PRODUCT_EDIT,
	beforeLoad: routeGuard,
	loader: ({ context: { queryClient }, params: { productId } }) =>
		queryClient.ensureQueryData(
			getProductOptions({
				path: {
					product_id: productId,
				},
			}),
		),
});
