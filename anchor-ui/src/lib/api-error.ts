import type { ApiErrorResponse } from "@/client";

export function getApiErrorMessage(error: unknown): string | undefined {
	if (!error || typeof error !== "object") {
		return undefined;
	}

	const maybeError = error as Partial<ApiErrorResponse>;
	return maybeError.errors?.[0]?.message;
}
