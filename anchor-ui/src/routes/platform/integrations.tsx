import { useProduct } from "@/hooks/useProduct";
import { routeGuard } from "@/lib/route-auth";
import PlatformIntegrationsPage from "@/pages/platform-integrations";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { Navigate, createRoute } from "@tanstack/react-router";

function PlatformIntegrationsRedirect() {
	const { currentProduct, isLoading } = useProduct();

	if (isLoading) {
		return <div>Loading...</div>;
	}

	if (!currentProduct) {
		return <PlatformIntegrationsPage />;
	}

	return (
		<Navigate
			to={ROUTE_PATHS.PRODUCT_INTEGRATIONS}
			params={{ productId: currentProduct.id }}
			replace
		/>
	);
}

export const platformIntegrationsRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PLATFORM_INTEGRATIONS,
	component: PlatformIntegrationsRedirect,
	beforeLoad: routeGuard,
});
