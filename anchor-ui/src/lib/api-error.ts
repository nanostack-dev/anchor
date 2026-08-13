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

const RAW_BODY_PREVIEW_LENGTH = 300;

/**
 * Best-effort detail string for an error surface, covering the shapes a
 * failed request can actually take — not just the structured
 * `ApiErrorResponse` case `getApiErrorMessage` handles.
 *
 * The generated client (`client.gen.ts`) throws whatever it got: the parsed
 * `ApiErrorResponse` when the body was JSON, the raw response text when it
 * wasn't (an HTML error page from a proxy or a misrouted request lands here
 * as a plain string, not an `Error`), or a bare `Error`/`TypeError` when
 * `fetch` itself never got a response at all (offline, CORS, DNS). Treating
 * "no structured message" as "the network is down" — the previous
 * behaviour — is wrong for the middle case: the request completed and the
 * server said something, we just didn't parse it as the shape we expected.
 * Losing that detail behind a generic "check your connection" message is
 * exactly what makes a real backend/proxy/CORS misconfiguration
 * indistinguishable from an offline laptop.
 *
 * Returns `undefined` only when there is truly nothing to show — the one
 * case that actually is "the request never completed."
 */
export function getErrorDetail(error: unknown): string | undefined {
	const structured = getApiErrorMessage(error);
	if (structured) return structured;

	if (error instanceof Error && error.message) {
		return error.message;
	}

	if (typeof error === "string" && error.trim()) {
		const trimmed = error.trim();
		const looksLikeMarkup = trimmed.startsWith("<");
		const preview =
			trimmed.length > RAW_BODY_PREVIEW_LENGTH
				? `${trimmed.slice(0, RAW_BODY_PREVIEW_LENGTH)}…`
				: trimmed;
		return looksLikeMarkup
			? "The server returned a page instead of a JSON response — the request likely never reached the API (a proxy or routing misconfiguration, not necessarily this browser's connection)."
			: preview;
	}

	return undefined;
}
