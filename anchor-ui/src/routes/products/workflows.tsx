import { routeGuard } from "@/lib/route-auth";
import WorkflowPage from "@/pages/workflow";
import WorkflowsPage from "@/pages/workflows";
import { rootRoute } from "@/routes/__root";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { createRoute } from "@tanstack/react-router";

export const workflowsRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCT_WORKFLOWS,
	component: WorkflowsPage,
	beforeLoad: routeGuard,
});

export const workflowNewRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCT_WORKFLOW_NEW,
	beforeLoad: routeGuard,
	validateSearch: (search: Record<string, unknown>): { recipe?: string } => ({
		recipe: typeof search.recipe === "string" ? search.recipe : undefined,
	}),
	component: WorkflowNewRoute,
});

function WorkflowNewRoute() {
	const { recipe } = workflowNewRoute.useSearch();
	return <WorkflowPage recipeId={recipe} />;
}

export const workflowDetailRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: ROUTE_PATHS.PRODUCT_WORKFLOW_DETAIL,
	beforeLoad: routeGuard,
	component: WorkflowDetailRoute,
});

function WorkflowDetailRoute() {
	const { workflowId } = workflowDetailRoute.useParams();
	return <WorkflowPage key={workflowId} workflowId={workflowId} />;
}
