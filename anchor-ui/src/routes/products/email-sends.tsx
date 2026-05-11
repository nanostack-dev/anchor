import { createRoute } from "@tanstack/react-router";
import { rootRoute } from "@/routes/__root";
import EmailSendsPage from "@/pages/email-sends";
import { routeGuard } from "@/lib/route-auth";
import { ROUTE_PATHS } from "@/routes/routePaths";

export const emailSendsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: ROUTE_PATHS.EMAIL_SENDS,
  component: EmailSendsPage,
  beforeLoad: routeGuard,
});
