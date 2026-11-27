import { Route } from "@effect/ui"
import { Schema } from "effect"

export const AppRoute = Route.union({
  Index: Route.make("/"),
  Login: Route.make("/login", {
    search: { redirect: Schema.optional(Schema.String) }
  }),
  Products: Route.make("/products"),
  NotFound: Route.make("/*")
})
export type AppRoute = typeof AppRoute.Type
