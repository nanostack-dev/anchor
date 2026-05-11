import { createRoute } from "@tanstack/react-router";
import { rootRoute } from "@/routes/__root";
import EmailTemplatesPage from "@/pages/email-templates";
import { routeGuard } from "@/lib/route-auth";
import { ROUTE_PATHS } from "@/routes/routePaths";

export const emailTemplatesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: ROUTE_PATHS.EMAIL_TEMPLATES,
  component: EmailTemplatesPage,
  beforeLoad: routeGuard,
});
