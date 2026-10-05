import { useProduct } from "@/hooks/useProduct";
import { routeGuard } from "@/lib/route-auth";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { EmptyState } from "@nanostackorg/design-system/blocks/empty-state";
import { createRoute, useNavigate } from "@tanstack/react-router";
import { useEffect } from "react";

function PlatformIntegrationsRedirect() {
	const { currentProduct, isLoading } = useProduct();
	const navigate = useNavigate();
	const productId = currentProduct?.id;
	useEffect(() => {
		if (!isLoading && productId)
			void navigate({
				to: ROUTE_PATHS.PRODUCT_INTEGRATIONS,
				params: { productId },
				replace: true,
			});
	}, [isLoading, productId, navigate]);

	if (isLoading) {
		return <div>Loading...</div>;
	}

	if (!currentProduct) {
		return (
			<EmptyState
				title="Select a product"
				description="Select a product to manage integrations."
			/>
		);
	}

	return null;
}

export const platformIntegrationsRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PLATFORM_INTEGRATIONS,
	component: PlatformIntegrationsRedirect,
	beforeLoad: routeGuard,
});
