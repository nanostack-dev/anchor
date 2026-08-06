import {
	RouterProvider,
	createMemoryHistory,
	createRootRoute,
	createRouter,
} from "@tanstack/react-router";
import type { ReactNode } from "react";

/**
 * Minimal TanStack Router context for stories.
 *
 * Any component rendering a `<Link>` needs a router in scope, even when the
 * story only cares that the anchor exists. The memory history keeps navigation
 * inside the story — clicking a link changes the URL and renders nothing new,
 * which is exactly what a component-level story wants.
 */
export function StoryRouter({
	children,
	initialPath = "/",
}: {
	children: ReactNode;
	initialPath?: string;
}) {
	const rootRoute = createRootRoute({ component: () => children });
	const router = createRouter({
		routeTree: rootRoute,
		history: createMemoryHistory({ initialEntries: [initialPath] }),
	});

	return <RouterProvider router={router} />;
}
