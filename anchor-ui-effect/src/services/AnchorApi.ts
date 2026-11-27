import { ProductPage, type ProductSortField, type SortDirection } from "#domain/Product"
import { ApiError, CurrentUser } from "#domain/Session"
import { Context, Effect, Layer, Schema } from "effect"
import { HttpClient, HttpClientRequest, HttpClientResponse } from "effect/http"
import type { LoginRequest, ProductRequest, ProductSearchRequest } from "../client/types.gen.ts"
import { SessionStore } from "./SessionStore.ts"

export interface ProductSearch {
  readonly text: string
  readonly offset: number
  readonly limit: number
  readonly sortBy: ProductSortField
  readonly sortDirection: SortDirection
}

const ApiErrorBody = Schema.Struct({
  errors: Schema.Array(Schema.Struct({ message: Schema.String }))
})

const AccessToken = Schema.Struct({ accessToken: Schema.String })

const unreachable = new ApiError({ status: 0, message: "The Anchor API could not be reached." })

const decodeOk = <A, I>(schema: Schema.Codec<A, I>, fallback: string) =>
  (response: HttpClientResponse.HttpClientResponse) =>
    response.status >= 200 && response.status < 300
      ? HttpClientResponse.schemaBodyJson(schema)(response).pipe(
        Effect.mapError(() => new ApiError({ status: response.status, message: fallback }))
      )
      : HttpClientResponse.schemaBodyJson(ApiErrorBody)(response).pipe(
        Effect.map((body) => body.errors[0]?.message ?? fallback),
        Effect.orElseSucceed(() => fallback),
        Effect.flatMap((message) => Effect.fail(new ApiError({ status: response.status, message })))
      )

export class AnchorApi extends Context.Service<AnchorApi, {
  login(email: string, password: string): Effect.Effect<string, ApiError>
  readonly logout: Effect.Effect<void>
  readonly currentUser: Effect.Effect<CurrentUser, ApiError>
  searchProducts(search: ProductSearch): Effect.Effect<ProductPage, ApiError>
  createProduct(input: { readonly name: string; readonly description: string }): Effect.Effect<void, ApiError>
}>()("anchor/AnchorApi") {
  static readonly layer = Layer.effect(
    AnchorApi,
    Effect.gen(function*() {
      const sessions = yield* SessionStore
      const http = yield* HttpClient.HttpClient

      const send = <A, I>(request: HttpClientRequest.HttpClientRequest, schema: Schema.Codec<A, I>, fallback: string) =>
        sessions.current.pipe(
          Effect.map((session) =>
            session === null ? request : HttpClientRequest.bearerToken(request, session.token)
          ),
          Effect.flatMap((authorized) => http.execute(authorized)),
          Effect.mapError(() => unreachable),
          Effect.flatMap(decodeOk(schema, fallback))
        )

      const postJson = <A, I>(url: string, body: unknown, schema: Schema.Codec<A, I>, fallback: string) =>
        send(HttpClientRequest.post(url).pipe(HttpClientRequest.bodyJsonUnsafe(body)), schema, fallback)

      return AnchorApi.of({
        login: (email, password) =>
          postJson(
            "/v1/auth/login",
            { email, password } satisfies LoginRequest,
            AccessToken,
            "An error occurred during login."
          ).pipe(Effect.map(({ accessToken }) => accessToken)),
        logout: send(HttpClientRequest.post("/v1/auth/logout"), Schema.Unknown, "Logout failed.").pipe(Effect.ignore),
        currentUser: send(HttpClientRequest.get("/v1/me"), CurrentUser, "Unable to load your account."),
        searchProducts: ({ limit, offset, sortBy, sortDirection, text }) =>
          postJson(
            "/v1/products/search",
            {
              pagination: { limit, offset },
              sort_by: sortBy,
              sort_direction: sortDirection,
              ...(text === "" ? {} : { full_text_search: text }),
              filter: {}
            } satisfies ProductSearchRequest,
            ProductPage,
            "Products could not be loaded."
          ),
        createProduct: ({ description, name }) =>
          postJson(
            "/v1/products",
            { name, ...(description === "" ? {} : { description }) } satisfies ProductRequest,
            Schema.Unknown,
            "The product could not be created."
          ).pipe(Effect.asVoid)
      })
    })
  )
}
