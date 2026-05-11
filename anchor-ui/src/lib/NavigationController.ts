import { ROUTE_PATHS } from "@/routes/routePaths";
import type { AnyRouter } from "@tanstack/react-router";

const isSearchParamValue = (
	value: unknown,
): value is string | number | boolean => {
	return (
		typeof value === "string" ||
		typeof value === "number" ||
		typeof value === "boolean"
	);
};

const toURLSearchParams = (search: Record<string, unknown> | undefined) => {
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

/**
 * NavigationController handles redirect management and post-authentication navigation
 * This class provides centralized navigation logic for the authentication flow
 */
export class NavigationController {
	private router: AnyRouter | null = null;
	private readonly REDIRECT_PARAM = "redirect";
	private readonly DEFAULT_HOME_ROUTE = ROUTE_PATHS.INDEX;
	private readonly SAFE_ROUTES: string[] = [
		ROUTE_PATHS.INIT,
		ROUTE_PATHS.LOGIN,
		ROUTE_PATHS.REGISTER,
		ROUTE_PATHS.INDEX,
	];

	/**
	 * Initialize the NavigationController with a router instance
	 */
	setRouter(router: AnyRouter): void {
		this.router = router;
	}

	/**
	 * Redirects to a specific route with optional URL preservation
	 * @param targetRoute - The route to redirect to
	 * @param preserveCurrentUrl - Whether to preserve the current URL as a redirect parameter
	 */
	redirectToRoute(targetRoute: string, preserveCurrentUrl = false): void {
		if (!this.router) {
			console.error("NavigationController: Router not initialized");
			window.location.href = targetRoute;
			return;
		}

		try {
			const currentLocation = this.router.state.location;

			if (
				preserveCurrentUrl &&
				this.shouldPreserveUrl(currentLocation.pathname, targetRoute)
			) {
				const searchParams = toURLSearchParams(currentLocation.search);
				const fullCurrentUrl =
					currentLocation.pathname +
					(searchParams.toString() ? `?${searchParams.toString()}` : "");

				this.router.navigate({
					to: targetRoute,
					search: {
						[this.REDIRECT_PARAM]: fullCurrentUrl,
					},
				});
			} else {
				this.router.navigate({
					to: targetRoute,
				});
			}
		} catch (error) {
			console.error("NavigationController: Error during redirect", error);
			window.location.href = targetRoute;
		}
	}

	/**
	 * Handles navigation after successful authentication
	 * @param redirectUrl - Optional redirect URL from search parameters
	 */
	handleSuccessfulAuth(redirectUrl?: string): void {
		const fallbackTarget = this.determinePostAuthRoute(redirectUrl);

		if (!this.router) {
			console.error("NavigationController: Router not initialized");
			window.location.href = fallbackTarget;
			return;
		}

		try {
			const targetUrl = fallbackTarget;

			console.log(
				"NavigationController: Handling successful auth, redirecting to:",
				targetUrl,
			);

			this.router.navigate({
				to: targetUrl,
				replace: true,
			});
		} catch (error) {
			console.error(
				"NavigationController: Error during post-auth navigation",
				error,
			);
			window.location.href = this.DEFAULT_HOME_ROUTE;
		}
	}

	/**
	 * Extracts the redirect URL from the current location's search parameters
	 */
	getRedirectUrl(): string | null {
		if (!this.router) {
			return null;
		}

		const search = this.router.state.location.search;
		const redirectValue = search?.[this.REDIRECT_PARAM];
		return typeof redirectValue === "string" ? redirectValue : null;
	}

	/**
	 * Validates that a redirect URL is safe to use
	 * @param url - The URL to validate
	 */
	isValidRedirectUrl(url: string): boolean {
		if (!url || typeof url !== "string") {
			return false;
		}

		try {
			if (url.startsWith("/")) {
				if (url.startsWith("//")) {
					return false;
				}

				return this.isInternalRoute(url);
			}

			const redirectUrl = new URL(url, window.location.origin);
			const currentOrigin = window.location.origin;

			if (redirectUrl.origin !== currentOrigin) {
				return false;
			}

			return this.isInternalRoute(redirectUrl.pathname);
		} catch (error) {
			console.warn("NavigationController: Invalid redirect URL format:", url);
			return false;
		}
	}

	/**
	 * Gets a safe redirect URL, falling back to home if invalid
	 * @param redirectUrl - The redirect URL to validate
	 * @param fallback - Fallback URL if redirect is invalid
	 */
	getSafeRedirectUrl(
		redirectUrl?: string,
		fallback: string = this.DEFAULT_HOME_ROUTE,
	): string {
		if (!redirectUrl) {
			return fallback;
		}

		return this.isValidRedirectUrl(redirectUrl) ? redirectUrl : fallback;
	}

	/**
	 * Determines if the current URL should be preserved when redirecting
	 * @param currentPath - Current path
	 * @param targetRoute - Target route for redirect
	 */
	private shouldPreserveUrl(currentPath: string, targetRoute: string): boolean {
		if (this.isAuthPage(currentPath)) {
			return false;
		}

		if (targetRoute !== "/login") {
			return false;
		}

		if (this.SAFE_ROUTES.includes(currentPath)) {
			return false;
		}

		return true;
	}

	/**
	 * Determines the appropriate route after successful authentication
	 * @param redirectUrl - Optional redirect URL
	 */
	private determinePostAuthRoute(redirectUrl?: string): string {
		if (!redirectUrl) {
			return this.DEFAULT_HOME_ROUTE;
		}

		if (!this.isValidRedirectUrl(redirectUrl)) {
			console.warn(
				"NavigationController: Invalid redirect URL, using fallback:",
				redirectUrl,
			);
			return this.DEFAULT_HOME_ROUTE;
		}

		const redirectPath = this.extractPathFromUrl(redirectUrl);
		if (this.isAuthPage(redirectPath)) {
			return this.DEFAULT_HOME_ROUTE;
		}

		return redirectUrl;
	}

	/**
	 * Checks if a path is an authentication-related page
	 * @param path - The path to check
	 */
	private isAuthPage(path: string): boolean {
		return path === "/login" || path === "/register" || path === "/init";
	}

	/**
	 * Checks if a route is considered internal/safe
	 * @param path - The path to check
	 */
	private isInternalRoute(path: string): boolean {
		if (!path.startsWith("/")) {
			return false;
		}

		const suspiciousPatterns = [
			"javascript:",
			"data:",
			"vbscript:",
			"file:",
			"ftp:",
			"<script",
			"javascript%3a",
			"data%3a",
		];

		const lowerPath = path.toLowerCase();
		return !suspiciousPatterns.some((pattern) => lowerPath.includes(pattern));
	}

	/**
	 * Extracts the path from a URL string
	 * @param url - The URL to extract path from
	 */
	private extractPathFromUrl(url: string): string {
		try {
			if (url.startsWith("/")) {
				// Already a path, extract just the pathname part
				const urlObj = new URL(url, window.location.origin);
				return urlObj.pathname;
			}

			const urlObj = new URL(url);
			return urlObj.pathname;
		} catch {
			return url.startsWith("/") ? url.split("?")[0] : "/";
		}
	}
}

export const navigationController = new NavigationController();
