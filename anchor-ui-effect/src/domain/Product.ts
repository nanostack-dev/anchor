import { Query } from "@effect/ui"
import { Schema } from "effect"

export const Product = Schema.Struct({
  id: Schema.String,
  name: Schema.String,
  description: Schema.optional(Schema.NullOr(Schema.String)),
  created_at: Schema.String,
  updated_at: Schema.String
})
export type Product = typeof Product.Type

export const ProductPage = Schema.Struct({
  items: Schema.Array(Product),
  total: Schema.Number
})
export type ProductPage = typeof ProductPage.Type

export const ProductSortField = Schema.Literals(["name", "created_at", "updated_at"])
export type ProductSortField = typeof ProductSortField.Type

export const SortDirection = Schema.Literals(["ASC", "DESC"])
export type SortDirection = typeof SortDirection.Type

export const ProductsTag = Query.Tag("products")
