import { routeGuard } from "@/lib/route-auth";
import ProductResourcePermissionDetailPage from "@/pages/product-resource-permission-detail";
import ProductRoleDetailPage from "@/pages/product-role-detail";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

const validateEditSearch = (
	search: Record<string, unknown>,
): { edit?: boolean } => ({
	edit: search.edit === true || search.edit === "true" ? true : undefined,
});

export const productRoleDetailRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCT_ROLE_DETAIL,
	beforeLoad: routeGuard,
	validateSearch: validateEditSearch,
	component: function ProductRoleDetailRoute() {
		const { roleId } = productRoleDetailRoute.useParams();
		const { edit } = productRoleDetailRoute.useSearch();
		return <ProductRoleDetailPage roleId={roleId} editing={!!edit} />;
	},
});

export const productResourcePermissionDetailRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCT_RESOURCE_PERMISSION_DETAIL,
	beforeLoad: routeGuard,
	validateSearch: validateEditSearch,
	component: function ProductResourcePermissionDetailRoute() {
		const { permissionName } = productResourcePermissionDetailRoute.useParams();
		const { edit } = productResourcePermissionDetailRoute.useSearch();
		return (
			<ProductResourcePermissionDetailPage
				permissionName={permissionName}
				editing={!!edit}
			/>
		);
	},
});
