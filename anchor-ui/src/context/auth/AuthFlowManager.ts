import { ROUTE_PATHS } from "@/routes/routePaths";
import { AuthFlowState } from "./AuthContext";

/**
 * AuthFlowManager handles the logic for determining the current authentication flow state
 * and managing transitions between different states based on tenantservice initialization and user authentication.
 */
export class AuthFlowManager {
	/**
	 * Determines the current authentication flow state based on loading states,
	 * tenantservice initialization status, user authentication status, and error conditions.
	 */
	determineAuthState(
		tenantInitialized: boolean,
		isAuthenticated: boolean,
		authLoading: boolean,
		tenantLoading: boolean,
		authError?: { type: string; retryable: boolean } | null,
	): AuthFlowState {
		// Loading must win before we inspect tenant initialization or auth errors,
		// otherwise bootstrapping can redirect to /init or /login using placeholder data.
		if (authLoading || tenantLoading) {
			return AuthFlowState.LOADING;
		}

		// If tenantservice is not initialized, user needs to complete initialization
		if (!tenantInitialized) {
			return AuthFlowState.TENANT_NOT_INITIALIZED;
		}
		//If error is type auth and retryable is false, then return UNAUTHENTICATED state
		if (authError && authError.type === "auth" && !authError.retryable) {
			return AuthFlowState.UNAUTHENTICATED;
		}
		// If there's a non-retryable error, return error state
		if (authError && !authError.retryable) {
			return AuthFlowState.ERROR;
		}

		// If there's a retryable error and we're not loading, return error state
		if (authError?.retryable) {
			return AuthFlowState.ERROR;
		}

		// If tenantservice is initialized but user is not authenticated
		if (tenantInitialized && !isAuthenticated) {
			return AuthFlowState.UNAUTHENTICATED;
		}

		// If tenantservice is initialized and user is authenticated
		if (tenantInitialized && isAuthenticated) {
			return AuthFlowState.AUTHENTICATED;
		}

		// Fallback to loading state for any unexpected combinations
		return AuthFlowState.LOADING;
	}

	/**
	 * Determines what route the user should be on based on their current path and auth state.
	 * Returns null if the current path is appropriate for the auth state.
	 */
	getRequiredRoute(
		currentPath: string,
		authState: AuthFlowState,
	): string | null {
		switch (authState) {
			case AuthFlowState.LOADING:
				// During loading, don't redirect - let the current page handle loading state
				return null;

			case AuthFlowState.ERROR:
				// During error state, don't redirect - let the current page handle error state
				return null;

			case AuthFlowState.TENANT_NOT_INITIALIZED:
				// User should be on /init page
				if (currentPath !== ROUTE_PATHS.INIT) {
					return ROUTE_PATHS.INIT;
				}
				return null;

			case AuthFlowState.UNAUTHENTICATED:
				// User should be on login or register pages
				if (
					currentPath !== ROUTE_PATHS.LOGIN &&
					currentPath !== ROUTE_PATHS.REGISTER
				) {
					return ROUTE_PATHS.LOGIN;
				}
				return null;

			case AuthFlowState.AUTHENTICATED:
				// Authenticated users should not be on auth pages
				if (
					currentPath === ROUTE_PATHS.LOGIN ||
					currentPath === ROUTE_PATHS.REGISTER ||
					currentPath === ROUTE_PATHS.INIT
				) {
					return ROUTE_PATHS.INDEX;
				}
				return null;

			default:
				return null;
		}
	}

	/**
	 * Determines if a redirect should occur based on current path and auth state.
	 */
	shouldRedirect(currentPath: string, authState: AuthFlowState): boolean {
		const requiredRoute = this.getRequiredRoute(currentPath, authState);
		return requiredRoute !== null && requiredRoute !== currentPath;
	}

	/**
	 * Determines if a user can access a specific route based on their auth state.
	 */
	canAccessRoute(path: string, authState: AuthFlowState): boolean {
		switch (authState) {
			case AuthFlowState.LOADING:
				return true;

			case AuthFlowState.ERROR:
				return true;

			case AuthFlowState.TENANT_NOT_INITIALIZED:
				return path === ROUTE_PATHS.INIT;

			case AuthFlowState.UNAUTHENTICATED:
				return path === ROUTE_PATHS.LOGIN || path === ROUTE_PATHS.REGISTER;

			case AuthFlowState.AUTHENTICATED:
				return (
					path !== ROUTE_PATHS.LOGIN &&
					path !== ROUTE_PATHS.REGISTER &&
					path !== ROUTE_PATHS.INIT
				);

			default:
				return false;
		}
	}
}

// Export a singleton instance for use throughout the application
export const authFlowManager = new AuthFlowManager();
