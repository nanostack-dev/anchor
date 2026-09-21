import { routeGuard } from "@/lib/route-auth";
import LicenseTemplatePage from "@/pages/license-template";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const licenseTemplateDetailRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCT_LICENSE_TEMPLATE_DETAIL,
	beforeLoad: routeGuard,
	validateSearch: (search: Record<string, unknown>): { edit?: boolean } => ({
		edit: search.edit === true || search.edit === "true" ? true : undefined,
	}),
	component: LicenseTemplateDetailRoute,
});

function LicenseTemplateDetailRoute() {
	const { templateId } = licenseTemplateDetailRoute.useParams();
	const { edit } = licenseTemplateDetailRoute.useSearch();
	return <LicenseTemplatePage templateId={templateId} editing={edit} />;
}

export const licenseTemplateNewRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCT_LICENSE_TEMPLATE_NEW,
	beforeLoad: routeGuard,
	component: () => <LicenseTemplatePage />,
});
