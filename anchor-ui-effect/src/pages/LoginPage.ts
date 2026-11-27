import { type AuthSession, AuthSession as AuthSessionSchema, sessionFromToken } from "#domain/Session"
import { AnchorApi } from "#services/AnchorApi"
import { SessionStore } from "#services/SessionStore"
import { Command, Component, Head, UiSchema, Validation } from "@effect/ui"
import { Html } from "@effect/ui/html"
import { Button, Card, CardContent, FieldError, Heading, Input, Label, Text, TextLink } from "@nanostackorg/design-system/effect"
import { Effect, Layer, Option, Redacted, Schema } from "effect"

const LoginValidation = Validation.make({
  email: {
    required: "Invalid email address",
    rules: [Validation.Rule.email("Invalid email address")]
  },
  password: {
    value: Schema.Redacted(Schema.String),
    required: "Password is required"
  }
}, { mode: "onChange" })

export const LoginPageMessage = UiSchema.TaggedEnum({
  UpdatedEmail: { value: Schema.String },
  UpdatedPassword: { value: Schema.Redacted(Schema.String) },
  SubmittedLogin: {},
  SucceededLogIn: { session: AuthSessionSchema },
  FailedLogIn: { message: Schema.String }
})

export const LoginPageEvents = UiSchema.TaggedEnum({
  LoggedIn: { session: AuthSessionSchema }
})

export class LogIn extends Command.Service<LogIn>()("auth/LogIn", {
  args: { email: Schema.String, password: Schema.Redacted(Schema.String) },
  success: LoginPageMessage
}) {
  static readonly layer = this.toLayer(
    Effect.gen(function*() {
      const api = yield* AnchorApi
      const sessions = yield* SessionStore
      return ({ email, password }) =>
        api.login(email, Redacted.value(password)).pipe(
          Effect.flatMap((token): Effect.Effect<typeof LoginPageMessage.Type> => {
            const session = sessionFromToken(token)
            return session === null
              ? Effect.succeed(LoginPageMessage.FailedLogIn({ message: "Failed to decode login token." }))
              : sessions.save(session).pipe(Effect.as(LoginPageMessage.SucceededLogIn({ session })))
          }),
          Effect.catch((error) => Effect.succeed(LoginPageMessage.FailedLogIn({ message: error.message })))
        )
    })
  )
}

const LoginPageNoDeps = Html.component({
  name: "LoginPage",
  props: Schema.Struct({ redirect: Schema.optional(Schema.String) }),
  model: Schema.Struct({
    form: LoginValidation.State,
    submitting: Schema.Boolean,
    error: Schema.NullOr(Schema.String)
  }),
  message: LoginPageMessage,
  events: LoginPageEvents
}).make({
  init: () => ({
    form: LoginValidation.init({ email: "", password: Redacted.make("") }),
    submitting: false,
    error: null
  }),
  update: {
    UpdatedEmail: ($, { value }) =>
      $.model.submitting ? $.model : $.evolve({ form: (form) => LoginValidation.change(form, "email", value) }),
    UpdatedPassword: ($, { value }) =>
      $.model.submitting ? $.model : $.evolve({ form: (form) => LoginValidation.change(form, "password", value) }),
    SubmittedLogin: ($) => {
      const form = LoginValidation.submit($.model.form)
      return Option.match(LoginValidation.validate(form).value, {
        onNone: () => $.evolve({ form }),
        onSome: (credentials) =>
          $.model.submitting
            ? $.model
            : $.set({ form, submitting: true, error: null }).run(LogIn.make(credentials))
      })
    },
    SucceededLogIn: ($, { session }) =>
      $.evolve({ submitting: false }).emit(LoginPageEvents.LoggedIn({ session })),
    FailedLogIn: ($, { message }) => $.evolve({ submitting: false, error: message })
  },
  head: () => [Head.title("Login")],
  view: (h, model) => {
    const { fields, value } = LoginValidation.validate(model.form)
    const edited = model.form.values.email !== "" || Redacted.value(model.form.values.password) !== ""
    const canSubmit = edited && Option.isSome(value) && !model.submitting
    const fieldErrors = (field: Validation.Field<unknown>) =>
      field._tag === "Invalid" ? FieldError(h).children(field.errors[0] ?? "") : null

    return h.div.class("grid min-h-svh lg:grid-cols-2").children(
      h.div.class("relative flex flex-col items-center justify-center gap-8 p-6 md:p-10").children(
        h.div.class("self-start").children(
          TextLink(h, { href: "/" }).attr("aria-label", "Anchor home").children("Anchor")
        ),
        h.div.class("grid w-full max-w-lg flex-1 content-center").children(
          Card(h, { variant: "outline", material: "glass" }).children(
            CardContent(h).children(
              h.div.class("flex flex-col gap-6").children(
                h.div.class("mb-6").children(
                  Heading(h, { level: 1 }).children("Login to your account"),
                  Text(h, { tone: "muted" }).children("Enter your email below to login to your account")
                ),
                h.form.attr("novalidate", "").class("space-y-6").on("submit", LoginPageMessage.SubmittedLogin())
                  .children(
                    h.div.class("space-y-2").children(
                      Label(h).attr("for", "login-email").children("Email"),
                      Input(h).attrs({ id: "login-email", type: "email", placeholder: "m@example.com" })
                        .value(fields.email.value)
                        .disabled(model.submitting)
                        .on("input", ({ value }) => LoginPageMessage.UpdatedEmail({ value })),
                      fieldErrors(fields.email)
                    ),
                    h.div.class("space-y-2").children(
                      h.div.class("flex items-center justify-between").children(
                        Label(h).attr("for", "login-password").children("Password"),
                        TextLink(h, { href: "/login" }).children("Forgot your password?")
                      ),
                      Input(h).attrs({ id: "login-password", type: "password", placeholder: "••••••••" })
                        .value(Redacted.value(fields.password.value))
                        .disabled(model.submitting)
                        .on("input", ({ value }) => LoginPageMessage.UpdatedPassword({ value: Redacted.make(value) })),
                      fieldErrors(fields.password)
                    ),
                    model.error === null ? null : FieldError(h).children(model.error),
                    Button(h, {
                      variant: "solid",
                      tone: "brand",
                      type: "submit",
                      width: "fill",
                      size: "lg",
                      disabled: !canSubmit,
                      loading: model.submitting
                    }).children(model.submitting ? "Logging in..." : "Login")
                  ),
                h.div.class("mt-6 text-center text-sm").children(
                  "Don't have an account? ",
                  TextLink(h, {
                    href: h.props.redirect === undefined
                      ? "/register"
                      : `/register?${new URLSearchParams({ redirect: h.props.redirect })}`
                  }).children("Sign up")
                )
              )
            )
          )
        )
      ),
      h.div.class("relative hidden overflow-hidden bg-primary/10 lg:block")
    )
  }
})

export type { AuthSession }

export const LoginPage = LoginPageNoDeps.pipe(
  Component.provide(LogIn.layer.pipe(Layer.provide(AnchorApi.layer)))
)
