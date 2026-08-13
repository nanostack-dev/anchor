/**
 * A generated-SDK error carrying its real HTTP status alongside whatever
 * body the server sent.
 *
 * Every `*Options()` react-query helper in `@/client/@tanstack/react-query.gen`
 * calls its SDK function with `throwOnError: true`, which throws only the
 * response body — `client.gen.ts`'s `request()` throws `finalError` and
 * nothing else, so the status code never reaches a `useQuery`'s `error`.
 * That's fine when the body is enough on its own to tell errors apart (a
 * structured `ApiErrorResponse` carrying a `code`), but it falls apart the
 * moment a response arrives with an empty or non-JSON body: there is then no
 * way left to tell "the server answered 404, on purpose, for a documented
 * reason" from "something failed outside the app entirely."
 *
 * Wrap a raw (default `throwOnError: false`) SDK call with `unwrapQuery` to
 * get this instead, and match on `status` rather than a body shape that
 * isn't guaranteed to be there.
 */
export class HttpQueryError extends Error {
	readonly status: number;
	readonly body: unknown;

	constructor(status: number, body: unknown) {
		super(`Request failed with status ${status}`);
		this.name = "HttpQueryError";
		this.status = status;
		this.body = body;
	}
}

export function isHttpQueryError(error: unknown): error is HttpQueryError {
	return error instanceof HttpQueryError;
}

interface RawSdkResult<TData> {
	data?: TData;
	error?: unknown;
	response: Response;
}

/**
 * Runs a raw generated SDK call (pass `throwOnError: false`, the default —
 * do not pass `throwOnError: true`, or the status is gone before this ever
 * sees it) and throws an `HttpQueryError` carrying the real HTTP status on
 * failure. Use this in a `queryFn` in place of a `*Options()` helper
 * wherever the caller needs to distinguish failures by status rather than
 * trusting the response body parsed into the shape the contract declares.
 */
export async function unwrapQuery<TData>(
	request: Promise<RawSdkResult<TData>>,
): Promise<TData> {
	const { data, error, response } = await request;
	if (!response.ok || error !== undefined) {
		throw new HttpQueryError(response.status, error);
	}
	return data as TData;
}
