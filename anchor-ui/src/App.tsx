import { AuthErrorBoundary } from "@/components/auth/AuthErrorBoundary";
import { AuthLoadingState } from "@/components/auth/AuthLoadingState";
import { AuthProvider, useAuth } from "@/context/auth/AuthContext";
import { navigationController } from "@/lib/NavigationController";
import { router } from "@/routeTree";
import { useQueryClient } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";
import { useEffect } from "react";

function InnerApp() {
	const queryClient = useQueryClient();
	const auth = useAuth();

	// Initialize NavigationController with router instance
	useEffect(() => {
		navigationController.setRouter(router);
	}, []);

	if (auth.authLoading) {
		return (
			<AuthLoadingState
				authLoading={auth.authLoading}
				tenantLoading={auth.tenantLoading}
			/>
		);
	}
	if (auth.authError && auth.authError.type !== "auth") {
		return (
			<AuthErrorBoundary
				error={auth.authError}
				onRetry={(): void => {
					auth.retryAuth();
				}}
				onLogout={(): void => {
					auth.logout();
				}}
			/>
		);
	}
	return (
		<RouterProvider
			router={router}
			context={{
				queryClient,
				auth: {
					user: auth.user,
					isAuthenticated: auth.isAuthenticated,
					token: auth.token,
					isTenantInitialized: auth.isTenantInitialized,
					authFlowState: auth.authFlowState,
					authLoading: auth.authLoading,
					tenantLoading: auth.tenantLoading,
				},
			}}
		/>
	);
}

export default function App() {
	return (
		<AuthProvider>
			<InnerApp />
		</AuthProvider>
	);
}
