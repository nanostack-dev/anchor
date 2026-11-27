import { Cause, Option, Schema } from "effect"
import { AsyncResult } from "effect/reactivity"

export const AuthSession = Schema.Struct({
  token: Schema.String,
  userId: Schema.String,
  tenantId: Schema.String
})
export type AuthSession = typeof AuthSession.Type

export const CurrentUser = Schema.Struct({
  id: Schema.String,
  email: Schema.String
})
export type CurrentUser = typeof CurrentUser.Type

export class ApiError extends Schema.TaggedError<ApiError>()("ApiError", {
  status: Schema.Number,
  message: Schema.String
}) {}

const decodeSegment = (segment: string): unknown =>
  JSON.parse(atob(segment.replace(/-/g, "+").replace(/_/g, "/")))

export const sessionFromToken = (token: string): AuthSession | null => {
  const payload = token.split(".")[1]
  if (payload === undefined) return null
  try {
    const claims = decodeSegment(payload)
    if (typeof claims !== "object" || claims === null) return null
    const { user_id, tenant_id, exp } = claims as Record<string, unknown>
    if (typeof user_id !== "string" || typeof tenant_id !== "string") return null
    if (typeof exp === "number" && exp * 1000 <= Date.now()) return null
    return { token, userId: user_id, tenantId: tenant_id }
  } catch {
    return null
  }
}

export const apiErrorOf = (cause: Cause.Cause<ApiError>): ApiError | null =>
  Option.getOrNull(Cause.findErrorOption(cause))

export const isUnauthorized = (result: AsyncResult.AsyncResult<unknown, ApiError>): boolean =>
  AsyncResult.isFailure(result) && apiErrorOf(result.cause)?.status === 401
