import { AuthSession, sessionFromToken } from "#domain/Session"
import { Context, Effect, Layer, Ref } from "effect"

const storageKey = "anchor_auth_state"

const readStored = (): AuthSession | null => {
  try {
    const raw = localStorage.getItem(storageKey)
    if (raw === null) return null
    const parsed: unknown = JSON.parse(raw)
    const token = typeof parsed === "object" && parsed !== null ? (parsed as { token?: unknown }).token : undefined
    return typeof token === "string" ? sessionFromToken(token) : null
  } catch {
    return null
  }
}

export class SessionStore extends Context.Service<SessionStore, {
  readonly current: Effect.Effect<AuthSession | null>
  save(session: AuthSession): Effect.Effect<void>
  readonly clear: Effect.Effect<void>
}>()("anchor/SessionStore") {
  static readonly layerBrowser = Layer.effect(
    SessionStore,
    Effect.gen(function*() {
      const ref = yield* Ref.make<AuthSession | null>(readStored())
      return SessionStore.of({
        current: Ref.get(ref),
        save: (session) =>
          Ref.set(ref, session).pipe(
            Effect.andThen(Effect.sync(() => localStorage.setItem(storageKey, JSON.stringify({ token: session.token }))))
          ),
        clear: Ref.set(ref, null).pipe(Effect.andThen(Effect.sync(() => localStorage.removeItem(storageKey))))
      })
    })
  )
}
