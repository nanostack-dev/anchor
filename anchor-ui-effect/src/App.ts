import { AuthSession, isUnauthorized } from "#domain/Session"
import { GetCurrentUser } from "#queries/GetCurrentUser"
import { AnchorApi } from "#services/AnchorApi"
import { SessionStore } from "#services/SessionStore"
import { Command, Component, Head, Router, UiSchema } from "@effect/ui"
import { Html, HtmlHead } from "@effect/ui/html"
import { Menu } from "@effect/ui-kit/Menu"
import {
  AppShell,
  Heading,
  menuItemStyles,
  menuPopupStyles,
  menuPositionerClass,
  sidebarMenuButtonStyles,
  SidebarNavItem,
  Text
} from "@nanostackorg/design-system/effect"
import { Effect, Layer, Option, Schema } from "effect"
import { AsyncResult } from "effect/reactivity"
import { LoginPage } from "./pages/LoginPage.ts"
import { ProductsPage } from "./pages/products/ProductsPage.ts"
import { AppRoute } from "./routes.ts"

const isPublic = (route: AppRoute) => route._tag === "Login"

const requiredRoute = (route: AppRoute, session: AuthSession | null): AppRoute | null => {
  if (session === null) {
    return isPublic(route) ? null : AppRoute.Login({ search: { redirect: AppRoute.print(route) } })
  }
  return isPublic(route) ? AppRoute.Index() : null
}

const routeAfterLogin = (redirect: string | undefined): AppRoute =>
  redirect === undefined || !redirect.startsWith("/") || redirect.startsWith("//")
    ? AppRoute.Index()
    : Option.getOrElse(AppRoute.parse(redirect), () => AppRoute.Index())

export const AppMessage = UiSchema.TaggedEnum({
  ChangedRoute: { route: AppRoute },
  LoggedIn: { session: AuthSession },
  LoadedCurrentUser: { unauthorized: Schema.Boolean },
  CompletedClearSession: { redirect: Schema.NullOr(Schema.String) },
  RequestedOpenUserMenu: {},
  RequestedCloseUserMenu: {},
  PickedUserMenuItem: { value: Schema.String }
})

export class ClearSession extends Command.Service<ClearSession>()("auth/ClearSession", {
  args: { redirect: Schema.NullOr(Schema.String) },
  success: AppMessage
}) {
  static readonly layer = this.toLayer(
    Effect.gen(function*() {
      const api = yield* AnchorApi
      const sessions = yield* SessionStore
      return ({ redirect }) =>
        Effect.gen(function*() {
          const session = yield* sessions.current
          yield* sessions.clear
          if (session !== null) yield* api.logout(session.token).pipe(Effect.timeout("2 seconds"), Effect.ignore)
          return AppMessage.CompletedClearSession({ redirect })
        })
    })
  )
}

const redirect = (route: AppRoute, session: AuthSession | null) => {
  const target = requiredRoute(route, session)
  return target === null ? null : Router.replace(AppRoute, target).pipe(Command.asNoop)
}

const userMenuItems = [{ value: "logout", label: "Log out" }]

const AppNoDeps = Html.component({
  name: "App",
  props: Schema.Struct({ route: AppRoute, session: Schema.NullOr(AuthSession) }),
  model: Schema.Struct({
    route: AppRoute,
    session: Schema.NullOr(AuthSession),
    userMenuOpen: Schema.Boolean
  }),
  message: AppMessage,
  queries: {
    user: {
      query: GetCurrentUser,
      args: ({ model }) => (model.session === null ? undefined : { userId: model.session.userId }),
      onChange: (result) => AppMessage.LoadedCurrentUser({ unauthorized: isUnauthorized(result) })
    }
  }
}).make({
  init: ($) => {
    const model = { route: $.props.route, session: $.props.session, userMenuOpen: false }
    const command = redirect(model.route, model.session)
    return command === null ? model : $.set(model).run(command)
  },
  update: {
    ChangedRoute: ($, { route }) => {
      const command = redirect(route, $.model.session)
      const next = $.evolve({ route, userMenuOpen: false })
      return command === null ? next : next.run(command)
    },
    LoggedIn: ($, { session }) => {
      const redirectParam = $.model.route._tag === "Login" ? $.model.route.search.redirect : undefined
      return $.evolve({ session }).run(
        Router.replace(AppRoute, routeAfterLogin(redirectParam)).pipe(Command.asNoop)
      )
    },
    LoadedCurrentUser: ($, { unauthorized }) =>
      unauthorized && $.model.session !== null
        ? $.evolve({ session: null, userMenuOpen: false }).run(
          ClearSession.make({ redirect: AppRoute.print($.model.route) })
        )
        : $.model,
    CompletedClearSession: ($, { redirect }) =>
      $.run(
        Router.replace(AppRoute, AppRoute.Login(redirect === null ? {} : { search: { redirect } })).pipe(
          Command.asNoop
        )
      ),
    RequestedOpenUserMenu: ($) => $.evolve({ userMenuOpen: true }),
    RequestedCloseUserMenu: ($) => $.evolve({ userMenuOpen: false }),
    PickedUserMenuItem: ($, { value }) =>
      value === "logout"
        ? $.evolve({ session: null, userMenuOpen: false }).run(ClearSession.make({ redirect: null }))
        : $.model
  },
  subscriptions: ({ model }) => [
    Router.subscription(AppRoute, model.route, (route) => AppMessage.ChangedRoute({ route }))
  ],
  head: () => [Head.titleTemplate((title) => `${title} · Anchor`), HtmlHead.lang("en")],
  view: (h, model) => {
    const route = model.route
    if (requiredRoute(route, model.session) !== null) {
      return h.div.attr("role", "status").class("sr-only").children("Redirecting…")
    }
    if (route._tag === "Login") {
      return LoginPage(h, {
        key: "login",
        props: route.search.redirect === undefined ? {} : { redirect: route.search.redirect },
        onEmit: { LoggedIn: ({ session }) => AppMessage.LoggedIn({ session }) }
      })
    }

    const email = AsyncResult.match(h.queries.user, {
      onInitial: () => "Loading account…",
      onFailure: () => "Account unavailable",
      onSuccess: ({ value }) => value.email
    })

    const userMenu = Menu(h, {
      props: { open: model.userMenuOpen, items: userMenuItems },
      onEmit: {
        RequestedOpen: () => AppMessage.RequestedOpenUserMenu(),
        RequestedClose: () => AppMessage.RequestedCloseUserMenu(),
        Picked: ({ value }) => AppMessage.PickedUserMenuItem({ value }),
        Highlighted: () => undefined,
        RequestedQuery: () => undefined,
        Settled: () => undefined
      },
      view: (menu) =>
        h.div(
          menu.trigger({ element: h.button.class(sidebarMenuButtonStyles({ size: "lg" })) }).children(
            h.span.class("flex min-w-0 flex-col text-left").children(
              h.span.class("truncate text-sm font-medium").children(email),
              h.span.class("truncate text-xs text-muted-foreground").children(email)
            )
          ),
          menu.positioner(h.div.class(menuPositionerClass)).children(
            menu.popup(h.div.class(menuPopupStyles({ width: "sm" }))).children(
              menu.item({ element: h.div.class(menuItemStyles({ tone: "neutral" })), value: "logout" }).children(
                "Log out"
              )
            )
          )
        )
    })

    const sidebar = [
      h.div.class("px-3 py-2").children(Heading(h, { level: 4 }).children("Anchor")),
      h.nav.attr("aria-label", "Platform").class("flex flex-1 flex-col gap-1").children(
        SidebarNavItem(h, { href: AppRoute.print(AppRoute.Index()), active: route._tag === "Index" }).children(
          h.span("Dashboard")
        ),
        SidebarNavItem(h, { href: AppRoute.print(AppRoute.Products()), active: route._tag === "Products" })
          .children(h.span("Products"))
      ),
      userMenu
    ]

    const main = AppRoute.match(route, {
      Index: () =>
        h.div.class("flex flex-col gap-2 p-6").children(
          Heading(h, { level: 1 }).children("Dashboard"),
          Text(h, { tone: "muted" }).children(`Welcome back, ${email}`)
        ),
      Products: (products) => ProductsPage(h, { key: AppRoute.printPath(products) }),
      Login: () => null,
      NotFound: () =>
        h.div.class("flex flex-col gap-2 p-6").children(
          Heading(h, { level: 1 }).children("Page not found"),
          Text(h, { tone: "muted" }).children("This page has not been migrated to Effect UI yet.")
        )
    })

    return AppShell(h, { material: "glass", sidebar, main: [main] })
  }
})

export const App = AppNoDeps.pipe(
  Component.provide(Layer.mergeAll(GetCurrentUser.layer, ClearSession.layer).pipe(Layer.provide(AnchorApi.layer)))
)
