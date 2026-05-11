import { useProduct } from "@/hooks/useProduct";
import { routeGuard } from "@/lib/route-auth";
import ClerkIntegrationPage from "@/pages/integration-clerk";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { Navigate, createRoute } from "@tanstack/react-router";

function IntegrationClerkRedirect() {
	const { currentProduct, isLoading } = useProduct();

	if (isLoading) {
		return <div>Loading...</div>;
	}

	if (!currentProduct) {
		return <ClerkIntegrationPage />;
	}

	return (
		<Navigate
			to={ROUTE_PATHS.PRODUCT_INTEGRATION_CLERK}
			params={{ productId: currentProduct.id }}
			replace
		/>
	);
}

export const integrationClerkRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.INTEGRATION_CLERK,
	component: IntegrationClerkRedirect,
	beforeLoad: routeGuard,
});
