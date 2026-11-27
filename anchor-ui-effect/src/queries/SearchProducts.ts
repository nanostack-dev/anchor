import { ProductPage, ProductSortField, ProductsTag, SortDirection } from "#domain/Product"
import { ApiError } from "#domain/Session"
import { Query } from "@effect/ui"
import { Effect, Schema } from "effect"
import { AnchorApi } from "#services/AnchorApi"

export class SearchProducts extends Query.Service<SearchProducts>()(
  "products/SearchProducts",
  {
    args: {
      text: Schema.String,
      offset: Schema.Number,
      limit: Schema.Number,
      sortBy: ProductSortField,
      sortDirection: SortDirection
    },
    success: ProductPage,
    error: ApiError,
    staleTime: "0 millis",
    tags: [ProductsTag]
  }
) {
  static readonly layer = this.toLayer(
    Effect.gen(function*() {
      const api = yield* AnchorApi
      return (search) => api.searchProducts(search)
    })
  )
}
