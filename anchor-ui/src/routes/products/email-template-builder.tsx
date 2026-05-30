import { routeGuard } from "@/lib/route-auth";
import EmailTemplateBuilderPage from "@/pages/email-template-builder";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const emailTemplateBuilderRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.EMAIL_TEMPLATE_BUILDER,
	component: () => {
		const { templateId } = emailTemplateBuilderRoute.useParams();
		return <EmailTemplateBuilderPage templateId={templateId} />;
	},
	beforeLoad: routeGuard,
});
