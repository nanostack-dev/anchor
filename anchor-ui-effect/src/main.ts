import "./styles.css"
import { SessionStore } from "#services/SessionStore"
import { Router } from "@effect/ui"
import { BrowserHost, BrowserRouter } from "@effect/ui/html"
import { Effect, Layer } from "effect"
import { FetchHttpClient } from "effect/http"
import { App } from "./App.ts"
import { AppRoute } from "./routes.ts"

const root = document.getElementById("app")
if (root === null) throw new Error("The #app element is missing from index.html.")

const initialProps = Effect.all({
  route: Router.currentRoute(AppRoute),
  session: Effect.gen(function*() {
    const sessions = yield* SessionStore
    return yield* sessions.current
  })
})

const Main = BrowserHost.mount(App, initialProps, root).pipe(
  Layer.provide([
    BrowserRouter.layer({ routes: AppRoute }),
    SessionStore.layerBrowser,
    FetchHttpClient.layer,
    Layer.succeed(FetchHttpClient.RequestInit, { credentials: "include" })
  ])
)

Layer.launch(Main).pipe(Effect.runFork)
