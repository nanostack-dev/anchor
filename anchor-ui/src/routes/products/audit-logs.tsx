import { routeGuard } from "@/lib/route-auth";
import ProductAuditLogsPage from "@/pages/product-audit-logs";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const productAuditLogsRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCT_AUDIT_LOGS,
	component: ProductAuditLogsPage,
	beforeLoad: routeGuard,
});
