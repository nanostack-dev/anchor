import { routeGuard } from "@/lib/route-auth";
import OrganizationLicenseChangesPage from "@/pages/organization-license-changes";
import OrganizationLicenseDetailPage from "@/pages/organization-license-detail";
import OrganizationLicenseUsagePage from "@/pages/organization-license-usage";
import OrganizationLicenseValuesPage from "@/pages/organization-license-values";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute, redirect } from "@tanstack/react-router";

export const organizationLicenseDetailRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.ORGANIZATION_LICENSE_DETAIL,
	component: OrganizationLicenseDetailPage,
	beforeLoad: routeGuard,
});

export const organizationLicenseUsageRoute = createRoute({
	getParentRoute: () => organizationLicenseDetailRoute,
	path: "usage",
	/**
	 * The charted limit lives in the URL, not in component state, so a support
	 * conversation can be handed over as one link that opens on the limit being
	 * argued about.
	 */
	validateSearch: (search: Record<string, unknown>): { field?: string } => ({
		field: typeof search.field === "string" ? search.field : undefined,
	}),
	component: OrganizationLicenseUsagePage,
});

export const organizationLicenseChangesRoute = createRoute({
	getParentRoute: () => organizationLicenseDetailRoute,
	path: "changes",
	component: OrganizationLicenseChangesPage,
});

export const organizationLicenseValuesRoute = createRoute({
	getParentRoute: () => organizationLicenseDetailRoute,
	path: "values",
	component: OrganizationLicenseValuesPage,
});

export const organizationLicenseDetailIndexRoute = createRoute({
	getParentRoute: () => organizationLicenseDetailRoute,
	path: "/",
	beforeLoad: ({ params }) => {
		throw redirect({
			to: organizationLicenseUsageRoute.to,
			params,
			replace: true,
		});
	},
});
