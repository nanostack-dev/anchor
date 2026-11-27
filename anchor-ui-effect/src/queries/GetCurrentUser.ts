import { ApiError, CurrentUser } from "#domain/Session"
import { Query } from "@effect/ui"
import { Effect, Schema } from "effect"
import { AnchorApi } from "#services/AnchorApi"

export class GetCurrentUser extends Query.Service<GetCurrentUser>()(
  "session/GetCurrentUser",
  {
    args: { userId: Schema.String },
    success: CurrentUser,
    error: ApiError,
    staleTime: "5 minutes"
  }
) {
  static readonly layer = this.toLayer(
    Effect.gen(function*() {
      const api = yield* AnchorApi
      return () => api.currentUser
    })
  )
}
