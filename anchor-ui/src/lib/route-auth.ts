import type { AuthClaims, AuthFlowState } from "@/context/auth/AuthContext";
import { authFlowManager } from "@/context/auth/AuthFlowManager";
import type { QueryClient } from "@tanstack/react-query";
import { redirect } from "@tanstack/react-router";
import { navigationController } from "./NavigationController";

type RouteSearch = Record<string, unknown> | undefined;

const isSearchParamValue = (
	value: unknown,
): value is string | number | boolean => {
	return (
		typeof value === "string" ||
		typeof value === "number" ||
		typeof value === "boolean"
	);
};

const toURLSearchParams = (search: RouteSearch): URLSearchParams => {
	const params = new URLSearchParams();

	for (const [key, value] of Object.entries(search ?? {})) {
		if (isSearchParamValue(value)) {
			params.append(key, String(value));
			continue;
		}

		if (!Array.isArray(value)) {
			continue;
		}

		for (const item of value) {
			if (isSearchParamValue(item)) {
				params.append(key, String(item));
			}
		}
	}

	return params;
};

export interface AuthRouteContext {
	queryClient: QueryClient;
	auth: {
		user: AuthClaims | null;
		isAuthenticated: boolean;
		token: string | null;
		isTenantInitialized: boolean;
		authFlowState: AuthFlowState;
		authLoading: boolean;
		tenantLoading: boolean;
	};
}

/**
 * Centralized route guard that handles all authentication flow states
 * This replaces all previous route guard functions with a unified approach
 */
export const routeGuard = ({
	context,
	location,
}: {
	context: AuthRouteContext;
	location: { pathname: string; search?: RouteSearch };
}) => {
	const { authFlowState } = context.auth;
	const currentPath = location.pathname;

	console.log(
		"🔐 routeGuard called - authFlowState:",
		authFlowState,
		"currentPath:",
		currentPath,
	);

	const requiredRoute = authFlowManager.getRequiredRoute(
		currentPath,
		authFlowState,
	);
	console.log(
		"🔍 Required route for current path:",
		requiredRoute,
		authFlowState,
	);
	if (requiredRoute && requiredRoute !== currentPath) {
		console.log("🔄 Redirecting from", currentPath, "to", requiredRoute);

		// Preserve redirect URL when redirecting to login from protected routes
		const shouldPreserveRedirect =
			requiredRoute === "/login" &&
			currentPath !== "/login" &&
			currentPath !== "/register" &&
			currentPath !== "/init";

		if (shouldPreserveRedirect) {
			// Construct the full URL to redirect back to after authentication
			const searchParams = toURLSearchParams(location.search);
			const fullRedirectUrl =
				currentPath +
				(searchParams.toString() ? `?${searchParams.toString()}` : "");

			throw redirect({
				to: requiredRoute,
				search: {
					redirect: fullRedirectUrl,
				},
			});
		}
		throw redirect({
			to: requiredRoute,
		});
	}

	console.log("✅ Route access allowed for", currentPath);
};

/**
 * Navigation helper functions for handling post-authentication redirects
 * These functions delegate to NavigationController for consistency
 */

/**
 * Extracts the redirect URL from search parameters
 */
export const getRedirectUrl = (search: RouteSearch): string | undefined => {
	const redirectValue = search?.redirect;
	return typeof redirectValue === "string" ? redirectValue : undefined;
};

/**
 * Validates that a redirect URL is safe to use
 */
export const isValidRedirectUrl = (url: string): boolean => {
	return navigationController.isValidRedirectUrl(url);
};

/**
 * Gets a safe redirect URL, falling back to home if invalid
 */
export const getSafeRedirectUrl = (
	search: RouteSearch,
	fallback = "/",
): string => {
	const redirectUrl = getRedirectUrl(search);
	return navigationController.getSafeRedirectUrl(redirectUrl, fallback);
};
