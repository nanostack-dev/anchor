import { AuthenticatedLayout } from "@/components/layout/AuthenticatedLayout";
import type { QueryClient } from "@tanstack/react-query";
import { ReactQueryDevtools } from "@tanstack/react-query-devtools";
import {
	Outlet,
	createRootRouteWithContext,
	useLocation,
} from "@tanstack/react-router";
import { TanStackRouterDevtools } from "@tanstack/react-router-devtools";

export interface RouteContext {
	queryClient: QueryClient;
	auth: {
		user: import("@/context/auth/AuthContext").AuthClaims | null;
		isAuthenticated: boolean;
		token: string | null;
		isTenantInitialized: boolean;
		authFlowState: import("@/context/auth/AuthContext").AuthFlowState;
		authLoading: boolean;
		tenantLoading: boolean;
	};
}

export const rootRoute = createRootRouteWithContext<RouteContext>()({
	component: __root,
});

function DevTools() {
	return (
		<>
			<TanStackRouterDevtools />
			<ReactQueryDevtools initialIsOpen={true} buttonPosition="bottom-right" />
		</>
	);
}

function __root() {
	const location = useLocation();

	if (
		location.pathname === "/login" ||
		location.pathname === "/register" ||
		location.pathname === "/init"
	) {
		return (
			<>
				<Outlet />
				<DevTools />
			</>
		);
	}

	return (
		<AuthenticatedLayout>
			<Outlet />
			<DevTools />
		</AuthenticatedLayout>
	);
}
