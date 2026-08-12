import type { ApiError, ApiErrorResponse } from "@/client";

export function getApiErrorMessage(error: unknown): string | undefined {
	if (!error || typeof error !== "object") {
		return undefined;
	}

	const maybeError = error as Partial<ApiErrorResponse>;
	return maybeError.errors?.[0]?.message;
}

/** Every structured error the API returned, or `[]` when the shape doesn't match. */
export function getApiErrors(error: unknown): ApiError[] {
	if (!error || typeof error !== "object") {
		return [];
	}

	const maybeError = error as Partial<ApiErrorResponse>;
	return maybeError.errors ?? [];
}

/** Whether any returned error carries the given machine-readable code. */
export function apiErrorHasCode(error: unknown, code: string): boolean {
	return getApiErrors(error).some((e) => e.code === code);
}

/**
 * Maps API errors onto the license field they named, so a schema or values
 * form can surface a validation failure against the offending row instead of
 * only as a toast. Errors without a `field` are dropped — call
 * `getApiErrorMessage` for a general fallback message.
 */
export function getApiFieldErrors(error: unknown): Record<string, string> {
	const fieldErrors: Record<string, string> = {};
	for (const apiError of getApiErrors(error)) {
		if (apiError.field) {
			fieldErrors[apiError.field] = apiError.message;
		}
	}
	return fieldErrors;
}
