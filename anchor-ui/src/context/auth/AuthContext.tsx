import { useMutation, useQuery } from "@tanstack/react-query";
import dayjs from "dayjs";
import { jwtDecode } from "jwt-decode";
import {
	createContext,
	useCallback,
	useContext,
	useEffect,
	useRef,
	useState,
} from "react";

import type { RefreshTokenError } from "@/client";
import {
	logoutMutation,
	refreshTokenMutation,
} from "@/client/@tanstack/react-query.gen";
import { client } from "@/client/client.gen";
import { navigationController } from "@/lib/NavigationController";
import { loginRoute } from "@/routes/platform/login";
import { authFlowManager } from "./AuthFlowManager";

export interface AuthClaims {
	user_id: string;
	tenant_id: string;
	iss?: string;
	sub?: string;
	aud?: string | string[];
	exp?: number;
	nbf?: number;
	iat?: number;
	jti?: string;

	[key: string]: unknown;
}

export enum AuthFlowState {
	LOADING = "loading",
	TENANT_NOT_INITIALIZED = "tenant_not_initialized",
	UNAUTHENTICATED = "unauthenticated",
	AUTHENTICATED = "authenticated",
	ERROR = "error",
}

export interface AuthError {
	type: "network" | "auth" | "tenantservice" | "token" | "unknown";
	message: string;
	retryable: boolean;
	timestamp: number;
}

interface AuthContextType {
	user: AuthClaims | null;
	isAuthenticated: boolean;
	token: string | null;
	isTenantInitialized: boolean;

	// Loading states
	authLoading: boolean;
	tenantLoading: boolean;

	// Error handling
	authError: AuthError | null;
	retryAuth: () => void;

	// Actions
	login: (token: string, user: AuthClaims) => void;
	logout: () => void;

	// New: Flow state management
	authFlowState: AuthFlowState;
	canAccessRoute: (path: string) => boolean;

	// Navigation methods
	handleSuccessfulAuth: (redirectUrl?: string) => void;
}

const AuthContext = createContext<AuthContextType>({
	user: null,
	isAuthenticated: false,
	token: null,
	isTenantInitialized: false,

	// Loading states
	authLoading: true,
	tenantLoading: true,

	// Error handling
	authError: null,
	retryAuth: () => {},

	// Actions
	login: () => {},
	logout: () => {},

	// New: Flow state management
	authFlowState: AuthFlowState.LOADING,
	canAccessRoute: () => false,

	// Navigation methods
	handleSuccessfulAuth: () => {},
});

// Constants for state synchronization
const AUTH_STATE_KEY = "anchor_auth_state";
const AUTH_SYNC_EVENT = "anchor_auth_sync";

interface AuthState {
	user: AuthClaims | null;
	token: string | null;
	timestamp: number;
}

interface HealthResponse {
	tenant_initialized: boolean;
}

interface HealthQueryError extends Error {
	status?: number;
}

interface RefreshQueryError extends Error {
	status?: number;
}

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({
	children,
}) => {
	const [user, setUser] = useState<AuthClaims | null>(null);
	const [token, setToken] = useState<string | null>(null);
	const [authLoading, setAuthLoading] = useState(true);
	const [authError, setAuthError] = useState<AuthError | null>(null);
	const [shouldBootstrapRefresh, setShouldBootstrapRefresh] = useState(false);

	// Refs for tracking state changes and preventing infinite loops
	const isInitializing = useRef(true);
	const lastSyncTimestamp = useRef(0);
	const syncInProgress = useRef(false);

	// Stable ref always holding the current token — used by the request interceptor
	// so we never need to re-register the interceptor when the token changes.
	const tokenRef = useRef<string | null>(null);

	// State persistence and synchronization functions
	const saveAuthState = useCallback(
		(authUser: AuthClaims | null, authToken: string | null) => {
			if (syncInProgress.current) return;

			try {
				const authState: AuthState = {
					user: authUser,
					token: authToken,
					timestamp: Date.now(),
				};

				localStorage.setItem(AUTH_STATE_KEY, JSON.stringify(authState));
				lastSyncTimestamp.current = authState.timestamp;

				// Broadcast state change to other tabs
				window.dispatchEvent(
					new CustomEvent(AUTH_SYNC_EVENT, {
						detail: authState,
					}),
				);
			} catch (error) {
				console.warn("Failed to save auth state to localStorage:", error);
			}
		},
		[],
	);

	const loadAuthState = useCallback((): AuthState | null => {
		try {
			const stored = localStorage.getItem(AUTH_STATE_KEY);
			if (!stored) return null;

			const authState: AuthState = JSON.parse(stored);

			// Validate stored token if it exists
			if (authState.token) {
				try {
					const claims = jwtDecode<AuthClaims>(authState.token);
					const isExpired = dayjs.unix(claims.exp || 0).isBefore(dayjs());

					if (isExpired) {
						// Token is expired, clear stored state
						localStorage.removeItem(AUTH_STATE_KEY);
						return null;
					}

					// Update user claims from token if they don't match
					if (JSON.stringify(authState.user) !== JSON.stringify(claims)) {
						authState.user = claims;
					}
				} catch (tokenError) {
					console.warn("Invalid token in stored auth state:", tokenError);
					localStorage.removeItem(AUTH_STATE_KEY);
					return null;
				}
			}

			return authState;
		} catch (error) {
			console.warn("Failed to load auth state from localStorage:", error);
			localStorage.removeItem(AUTH_STATE_KEY);
			return null;
		}
	}, []);

	const clearAuthState = useCallback(() => {
		try {
			localStorage.removeItem(AUTH_STATE_KEY);

			// Broadcast logout to other tabs
			window.dispatchEvent(
				new CustomEvent(AUTH_SYNC_EVENT, {
					detail: { user: null, token: null, timestamp: Date.now() },
				}),
			);
		} catch (error) {
			console.warn("Failed to clear auth state from localStorage:", error);
		}
	}, []);

	// Register the request interceptor once at mount.
	// It reads from tokenRef so it always sends the current token without
	// needing to be re-registered every time the token changes.
	useEffect(() => {
		const interceptorId = client.interceptors.request.use((request) => {
			if (tokenRef.current) {
				request.headers.set("Authorization", `Bearer ${tokenRef.current}`);
			}
			return request;
		});

		return () => {
			client.interceptors.request.eject(interceptorId);
		};
	}, []);

	// Initialize auth state from localStorage on mount
	useEffect(() => {
		if (!isInitializing.current) return;

		const storedState = loadAuthState();
		const restoredSession = !!(storedState?.token && storedState.user);

		if (storedState?.token && storedState.user) {
			console.log("AuthContext: Restoring auth state from localStorage");
			tokenRef.current = storedState.token;
			setToken(storedState.token);
			setUser(storedState.user);
			lastSyncTimestamp.current = storedState.timestamp;
		}

		// If there is no restored bearer token, keep auth loading active until
		// we attempt cookie-based bootstrap refresh or determine the user is anonymous.
		setAuthLoading(!restoredSession);
		setShouldBootstrapRefresh(!restoredSession);

		isInitializing.current = false;
	}, [loadAuthState]);

	// Cross-tab synchronization listener
	useEffect(() => {
		const handleAuthSync = (event: CustomEvent<AuthState>) => {
			if (syncInProgress.current) return;

			const { user: syncUser, token: syncToken, timestamp } = event.detail;

			// Only sync if the timestamp is newer than our last sync
			if (timestamp <= lastSyncTimestamp.current) return;

			console.log("AuthContext: Syncing auth state from another tab");
			syncInProgress.current = true;

			try {
				tokenRef.current = syncToken;
				setToken(syncToken);
				setUser(syncUser);
				setAuthLoading(false);
				setShouldBootstrapRefresh(false);
				lastSyncTimestamp.current = timestamp;

				// Clear any auth errors since we have new state
				setAuthError(null);
			} finally {
				syncInProgress.current = false;
			}
		};

		window.addEventListener(AUTH_SYNC_EVENT, handleAuthSync as EventListener);

		return () => {
			window.removeEventListener(
				AUTH_SYNC_EVENT,
				handleAuthSync as EventListener,
			);
		};
	}, []);

	const refresh = useMutation({
		...refreshTokenMutation({ credentials: "include" }),
		mutationKey: ["refreshToken"],
		onSuccess: (data) => {
			const claims = jwtDecode<AuthClaims>(data.accessToken);
			tokenRef.current = data.accessToken;
			setToken(data.accessToken);
			setUser(claims);
			setAuthLoading(false);
			setShouldBootstrapRefresh(false);
			setAuthError(null);
			saveAuthState(claims, data.accessToken);

			return data;
		},
		onError: (error: RefreshTokenError) => {
			console.error("Failed to refresh token", error);
			const hasActiveToken = !!tokenRef.current;

			if (!hasActiveToken) {
				tokenRef.current = null;
				setToken(null);
				setUser(null);
				clearAuthState();

				const authError: AuthError = {
					type: "auth",
					message: "Failed to refresh token",
					retryable: false,
					timestamp: Date.now(),
				};

				setAuthError(authError);
			}

			setAuthLoading(false);
			setShouldBootstrapRefresh(false);
		},
	});

	const {
		data: healthData,
		isLoading: tenantLoading,
		error: healthError,
		refetch: refetchHealth,
	} = useQuery<HealthResponse>({
		queryKey: ["health"],
		queryFn: async () => {
			const baseUrl = client.getConfig().baseUrl || window.location.origin;
			const healthUrl = new URL("/health", baseUrl).toString();

			const response = await fetch(healthUrl, {
				credentials: "include",
			});

			if (!response.ok) {
				const error: HealthQueryError = new Error(
					`Health check failed with status ${response.status}`,
				);
				error.status = response.status;
				throw error;
			}

			return response.json() as Promise<HealthResponse>;
		},
		retry: (failureCount, error: HealthQueryError) => {
			return failureCount < 3 && error?.status !== 401;
		},
		retryDelay: (attemptIndex) => Math.min(1000 * 2 ** attemptIndex, 30000),
	});

	const logoutCall = useMutation({
		...logoutMutation({ credentials: "include" }),
	});

	const triggerBootstrapRefresh = useCallback(() => {
		refresh.mutate({ credentials: "include" });
	}, [refresh]);

	useEffect(() => {
		if (
			!shouldBootstrapRefresh ||
			refresh.isPending ||
			!healthData ||
			tokenRef.current
		)
			return;

		if (!healthData.tenant_initialized) {
			setAuthLoading(false);
			setShouldBootstrapRefresh(false);
			return;
		}

		triggerBootstrapRefresh();
	}, [
		healthData,
		refresh.isPending,
		shouldBootstrapRefresh,
		triggerBootstrapRefresh,
	]);

	// Tab refocus or network reconnect: refresh if expired
	useEffect(() => {
		const checkAndRefreshToken = async () => {
			if (!token || refresh.isPending) return;

			try {
				const claims = jwtDecode<AuthClaims>(token);
				const isExpired = dayjs.unix(claims.exp || 0).isBefore(dayjs());

				if (isExpired) {
					await refresh.mutateAsync({ credentials: "include" });
				}
			} catch (err) {
				console.error("Error during refresh:", err);

				const refreshError = err as RefreshQueryError;
				if (refreshError.status !== 401) {
					setAuthError({
						type: "token",
						message:
							"Failed to refresh your session. Retrying later may recover automatically.",
						retryable: true,
						timestamp: Date.now(),
					});
					return;
				}

				tokenRef.current = null;
				setToken(null);
				setUser(null);
				setShouldBootstrapRefresh(false);
				clearAuthState();
				setAuthError({
					type: "auth",
					message: "Session expired. Please sign in again.",
					retryable: false,
					timestamp: Date.now(),
				});
			}
		};

		const handleVisibilityChange = () => {
			if (document.visibilityState === "visible") {
				checkAndRefreshToken();
			}
		};

		const handleOnline = () => {
			checkAndRefreshToken();
		};

		window.addEventListener("visibilitychange", handleVisibilityChange);
		window.addEventListener("online", handleOnline);

		return () => {
			window.removeEventListener("visibilitychange", handleVisibilityChange);
			window.removeEventListener("online", handleOnline);
		};
	}, [clearAuthState, refresh.isPending, refresh.mutateAsync, token]);

	const login = (token: string, claims: AuthClaims) => {
		tokenRef.current = token;
		setToken(token);
		setUser(claims);
		setAuthLoading(false);
		setShouldBootstrapRefresh(false);
		setAuthError(null);

		// Save auth state to localStorage and sync across tabs
		saveAuthState(claims, token);
	};

	const logout = async () => {
		try {
			await logoutCall.mutateAsync({});
		} catch (error) {
			console.error("Logout API call failed:", error);
		} finally {
			// Clear auth state regardless of API call success/failure
			tokenRef.current = null;
			setToken(null);
			setUser(null);
			setAuthLoading(false);
			setShouldBootstrapRefresh(false);
			setAuthError(null);

			// Clear auth state from localStorage and sync across tabs
			clearAuthState();

			navigationController.redirectToRoute(loginRoute.path);
		}
	};

	const handleSuccessfulAuth = (redirectUrl?: string) => {
		navigationController.handleSuccessfulAuth(redirectUrl);
	};

	// Error handling functions
	const retryAuth = () => {
		setAuthError(null);
		const needsBootstrapRefresh = !tokenRef.current;
		setAuthLoading(needsBootstrapRefresh);
		setShouldBootstrapRefresh(needsBootstrapRefresh);
		void refetchHealth();
	};

	useEffect(() => {
		if (healthData) {
			setAuthError((currentError) => {
				if (currentError?.type === "tenantservice") {
					return null;
				}

				return currentError;
			});
		}
	}, [healthData]);

	// Handle health check errors
	useEffect(() => {
		if (healthError && !tenantLoading) {
			setAuthLoading(false);
			setShouldBootstrapRefresh(false);
			const error = healthError as HealthQueryError;
			const authError: AuthError = {
				type: "tenantservice",
				message: error?.message || "Failed to check tenantservice status",
				retryable: true,
				timestamp: Date.now(),
			};
			setAuthError(authError);
		}
	}, [healthError, tenantLoading]);

	const isAuthenticated = !!user && !!token;
	const isTenantInitialized = healthData?.tenant_initialized || false;

	const authFlowState = authFlowManager.determineAuthState(
		isTenantInitialized,
		isAuthenticated,
		authLoading,
		tenantLoading,
		authError,
	);

	const canAccessRoute = (path: string): boolean => {
		return authFlowManager.canAccessRoute(path, authFlowState);
	};

	return (
		<AuthContext.Provider
			value={{
				user,
				isAuthenticated,
				token,
				isTenantInitialized,

				// Loading states
				authLoading,
				tenantLoading,

				// Error handling
				authError,
				retryAuth,

				// Actions
				login,
				logout,

				authFlowState,
				canAccessRoute,

				// Navigation methods
				handleSuccessfulAuth,
			}}
		>
			{children}
		</AuthContext.Provider>
	);
};

export function useAuth() {
	const ctx = useContext(AuthContext);
	if (!ctx) throw new Error("useAuth must be used within an AuthProvider");
	return ctx;
}
