import type { ApiError, ApiErrorResponse } from "@/client";

export function getApiErrorMessage(error: unknown): string | undefined {
	if (!error || typeof error !== "object") {
		return undefined;
	}

	const maybeError = error as Partial<ApiErrorResponse>;
	return maybeError.errors?.[0]?.message;
}

/**
 * Returns the first typed API error (code, message, metadata) from a thrown
 * API error body, letting callers branch on machine-readable codes such as
 * `PLAN_IN_USE`.
 */
export function getApiError(error: unknown): ApiError | undefined {
	if (!error || typeof error !== "object") {
		return undefined;
	}

	const maybeError = error as Partial<ApiErrorResponse>;
	return maybeError.errors?.[0];
}
