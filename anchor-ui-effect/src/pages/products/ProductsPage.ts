import { type Product, type ProductPage, ProductSortField, ProductsTag, SortDirection } from "#domain/Product"
import { apiErrorOf } from "#domain/Session"
import { SearchProducts } from "#queries/SearchProducts"
import { AnchorApi } from "#services/AnchorApi"
import { Command, Component, Head, UiSchema } from "@effect/ui"
import { Html } from "@effect/ui/html"
import { Dialog } from "@effect/ui-kit/Dialog"
import {
  Button,
  dialogBodyClass,
  dialogFooterClass,
  dialogHeaderClass,
  dialogOverlayClass,
  dialogPopupStyles,
  dialogTitleClass,
  FieldError,
  Input,
  Label,
  PageHeader,
  Table,
  TableBody,
  TableCell,
  TableEmpty,
  TableHead,
  TableHeader,
  TableRow
} from "@nanostackorg/design-system/effect"
import { Effect, Layer, Schema } from "effect"
import { AsyncResult } from "effect/reactivity"

const pageSize = 10

const dateFormat = new Intl.DateTimeFormat("en", {
  day: "numeric",
  month: "long",
  year: "numeric",
  hour: "numeric",
  minute: "2-digit",
  hourCycle: "h23"
})

const formatDate = (iso: string) => dateFormat.format(new Date(iso)).replace(" at ", " ")

export const ProductsPageMessage = UiSchema.TaggedEnum({
  UpdatedSearch: { text: Schema.String },
  ClickedClearSearch: {},
  ClickedSortColumn: { field: ProductSortField },
  ClickedPreviousPage: {},
  ClickedNextPage: {},
  RequestedOpenCreateDialog: {},
  RequestedCloseCreateDialog: {},
  UpdatedNewProductName: { value: Schema.String },
  UpdatedNewProductDescription: { value: Schema.String },
  SubmittedCreateProduct: {},
  SucceededCreateProduct: {},
  FailedCreateProduct: { message: Schema.String }
})

export class CreateProduct extends Command.Service<CreateProduct>()("products/CreateProduct", {
  args: { name: Schema.String, description: Schema.String },
  success: ProductsPageMessage,
  invalidates: () => [ProductsTag]
}) {
  static readonly layer = this.toLayer(
    Effect.gen(function*() {
      const api = yield* AnchorApi
      return (input) =>
        api.createProduct(input).pipe(
          Effect.match({
            onSuccess: () => ProductsPageMessage.SucceededCreateProduct(),
            onFailure: (error) => ProductsPageMessage.FailedCreateProduct({ message: error.message })
          })
        )
    })
  )
}

const NewProduct = Schema.Struct({
  open: Schema.Boolean,
  name: Schema.String,
  description: Schema.String,
  submitting: Schema.Boolean,
  error: Schema.NullOr(Schema.String)
})

const closedNewProduct = { open: false, name: "", description: "", submitting: false, error: null }

const ProductsPageNoDeps = Html.component({
  name: "ProductsPage",
  model: Schema.Struct({
    text: Schema.String,
    offset: Schema.Number,
    sortBy: ProductSortField,
    sortDirection: SortDirection,
    newProduct: NewProduct
  }),
  message: ProductsPageMessage,
  queries: {
    products: {
      query: SearchProducts,
      debounce: "300 millis",
      args: ({ model }) => ({
        text: model.text,
        offset: model.offset,
        limit: pageSize,
        sortBy: model.sortBy,
        sortDirection: model.sortDirection
      })
    }
  }
}).make({
  init: () => ({
    text: "",
    offset: 0,
    sortBy: "created_at" as const,
    sortDirection: "DESC" as const,
    newProduct: closedNewProduct
  }),
  update: {
    UpdatedSearch: ($, { text }) => $.evolve({ text, offset: 0 }),
    ClickedClearSearch: ($) => $.evolve({ text: "", offset: 0 }),
    ClickedSortColumn: ($, { field }) =>
      $.evolve({
        sortBy: field,
        sortDirection: $.model.sortBy === field && $.model.sortDirection === "ASC" ? "DESC" : "ASC",
        offset: 0
      }),
    ClickedPreviousPage: ($) => $.evolve({ offset: (offset) => Math.max(0, offset - pageSize) }),
    ClickedNextPage: ($) => $.evolve({ offset: (offset) => offset + pageSize }),
    RequestedOpenCreateDialog: ($) => $.evolve({ newProduct: { ...closedNewProduct, open: true } }),
    RequestedCloseCreateDialog: ($) =>
      $.model.newProduct.submitting ? $.model : $.evolve({ newProduct: closedNewProduct }),
    UpdatedNewProductName: ($, { value }) =>
      $.evolve({ newProduct: { ...$.model.newProduct, name: value, error: null } }),
    UpdatedNewProductDescription: ($, { value }) =>
      $.evolve({ newProduct: { ...$.model.newProduct, description: value } }),
    SubmittedCreateProduct: ($) => {
      const { description, name, submitting } = $.model.newProduct
      if (submitting) return $.model
      if (name.trim() === "") {
        return $.evolve({ newProduct: { ...$.model.newProduct, error: "Name is required" } })
      }
      return $.evolve({ newProduct: { ...$.model.newProduct, submitting: true, error: null } })
        .run(CreateProduct.make({ name: name.trim(), description: description.trim() }))
    },
    SucceededCreateProduct: ($) => $.evolve({ newProduct: closedNewProduct, offset: 0 }),
    FailedCreateProduct: ($, { message }) =>
      $.evolve({ newProduct: { ...$.model.newProduct, submitting: false, error: message } })
  },
  head: () => [Head.title("Products")],
  view: (h, model) => {
    const sortLabel = (field: ProductSortField, label: string) =>
      TableHead(h).attr(
        "aria-sort",
        model.sortBy === field ? (model.sortDirection === "ASC" ? "ascending" : "descending") : undefined
      ).children(
        h.button.attr("type", "button").class("inline-flex items-center gap-1 font-medium")
          .on("click", ProductsPageMessage.ClickedSortColumn({ field }))
          .children(label)
      )

    const emptyState = () =>
      model.text === ""
        ? TableEmpty(h, { colSpan: 4 }).children("No products yet")
        : TableEmpty(h, { colSpan: 4 }).children(
          h.div.class("flex flex-col items-center gap-3").children(
            h.p.children("No products match your search"),
            Button(h, { size: "sm" }).on("click", ProductsPageMessage.ClickedClearSearch()).children("Clear search")
          )
        )

    const row = (product: Product) =>
      TableRow(h).key(product.id).children(
        TableCell(h).children(
          h.span.class("block max-w-[16rem] min-w-0 break-words whitespace-normal").children(product.name)
        ),
        TableCell(h, { tone: "muted" }).children(
          h.span.class("block max-w-[200px] truncate").children(product.description || "No description")
        ),
        TableCell(h).children(formatDate(product.created_at)),
        TableCell(h).children(formatDate(product.updated_at))
      )

    const table = (page: ProductPage, waiting: boolean) => {
      const pageCount = Math.max(1, Math.ceil(page.total / pageSize))
      const currentPage = Math.floor(model.offset / pageSize) + 1
      return h.div.class("flex flex-col gap-4").children(
        Table(h, { material: "glass" }).attr("aria-busy", waiting ? "true" : "false").children(
          TableHeader(h).children(
            TableRow(h).children(
              sortLabel("name", "Name"),
              TableHead(h).children("Description"),
              sortLabel("created_at", "Created At"),
              sortLabel("updated_at", "Updated At")
            )
          ),
          TableBody(h).children(page.items.length === 0 ? emptyState() : page.items.map(row))
        ),
        h.nav.attr("aria-label", "Pagination").class("flex items-center justify-end gap-2 text-sm").children(
          h.span.class("text-muted-foreground").children(`Page ${currentPage} of ${pageCount}`),
          Button(h, { size: "sm", disabled: model.offset === 0 })
            .on("click", ProductsPageMessage.ClickedPreviousPage())
            .children("Previous"),
          Button(h, { size: "sm", disabled: model.offset + pageSize >= page.total })
            .on("click", ProductsPageMessage.ClickedNextPage())
            .children("Next")
        )
      )
    }

    const createDialog = Dialog(h, {
      props: { open: model.newProduct.open, modal: true },
      onEmit: {
        RequestedOpen: () => ProductsPageMessage.RequestedOpenCreateDialog(),
        RequestedClose: () => ProductsPageMessage.RequestedCloseCreateDialog(),
        Settled: () => undefined
      },
      view: (dialog) =>
        h.div(
          dialog.trigger(Button(h, { variant: "solid", tone: "brand" })).children("Create Product"),
          dialog.portal(h.dialog)?.children(
            dialog.backdrop(h.div.class(dialogOverlayClass)),
            dialog.popup(h.div.class(dialogPopupStyles({ size: "md" }))).children(
              h.form.attr("novalidate", "").class(dialogBodyClass)
                .on("submit", ProductsPageMessage.SubmittedCreateProduct())
                .children(
                  h.div.class(dialogHeaderClass).children(
                    dialog.title(h.h2.class(dialogTitleClass)).children("Create Product")
                  ),
                  h.div.class("space-y-2").children(
                    Label(h).attr("for", "new-product-name").children("Name"),
                    Input(h, { material: "glass" }).attrs({ id: "new-product-name", autocomplete: "off" })
                      .value(model.newProduct.name)
                      .disabled(model.newProduct.submitting)
                      .on("input", ({ value }) => ProductsPageMessage.UpdatedNewProductName({ value }))
                  ),
                  h.div.class("space-y-2").children(
                    Label(h).attr("for", "new-product-description").children("Description"),
                    Input(h, { material: "glass" }).attrs({ id: "new-product-description", autocomplete: "off" })
                      .value(model.newProduct.description)
                      .disabled(model.newProduct.submitting)
                      .on("input", ({ value }) => ProductsPageMessage.UpdatedNewProductDescription({ value }))
                  ),
                  model.newProduct.error === null ? null : FieldError(h).children(model.newProduct.error),
                  h.div.class(dialogFooterClass).children(
                    dialog.close(Button(h)).children("Cancel"),
                    Button(h, { variant: "solid", tone: "brand", type: "submit", loading: model.newProduct.submitting })
                      .children(model.newProduct.submitting ? "Creating..." : "Create")
                  )
                )
            )
          )
        )
    })

    return h.div.class("mx-auto flex w-full flex-col gap-6 p-6").children(
      PageHeader(h, {
        title: "Products",
        description: "Manage and review products. Search, filter, and take actions."
      }),
      h.div.class("flex items-center justify-between gap-4").children(
        createDialog,
        Input(h, { material: "glass" }).attrs({ type: "text", placeholder: "Search products", "aria-label": "Search products" })
          .class("max-w-xs")
          .value(model.text)
          .on("input", ({ value }) => ProductsPageMessage.UpdatedSearch({ text: value }))
      ),
      AsyncResult.match(h.queries.products, {
        onInitial: () => h.p.attr("role", "status").class("text-sm text-muted-foreground").children("Loading products…"),
        onFailure: ({ cause }) =>
          h.div.attr("role", "alert").class("text-sm text-destructive").children(
            apiErrorOf(cause)?.message ?? "Products could not be loaded."
          ),
        onSuccess: ({ value, waiting }) => table(value, waiting)
      })
    )
  }
})

export const ProductsPage = ProductsPageNoDeps.pipe(
  Component.provide(Layer.mergeAll(SearchProducts.layer, CreateProduct.layer).pipe(Layer.provide(AnchorApi.layer)))
)
