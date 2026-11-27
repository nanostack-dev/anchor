# anchor-ui-effect (experiment)

Status: spike, not deployed, not built by CI. It answers one question: what would it take to move `anchor-ui` to [Effect UI](https://ui.effect.website/)?

## What was ported

A vertical slice of `anchor-ui`, rewritten (not converted) in Effect UI:

- Root app: typed routes, the auth guard (guest to `/login?redirect=…`, signed-in user away from `/login`), session restore from `localStorage`, and the sidebar shell with the user menu (`@effect/ui-kit` `Menu`) and Log out.
- Login page: validated form (`Validation`), login Command, redirect back after login.
- Products page: server-side search (debounced query), sort by column, offset pagination, empty and no-match states with Clear search, and Create Product in a `@effect/ui-kit` `Dialog` whose Command invalidates the list.

Every other route renders "Page not found".

Styling comes from the design system's `effect` branch ([nanostack-design-system `effect`](https://github.com/nanostack-dev/nanostack-design-system/tree/effect), `docs/technical/effect-ui.md`). That branch moves the class recipes out of the React components into `src/recipes` (no React) and adds Effect UI view functions over them, so both stacks render from the same class strings and tokens.

## Result

| Check | Result |
| --- | --- |
| `anchor-ui/e2e/smoke/login.e2e.ts`, unchanged, run against this build and a local disposable backend | 2 of 2 pass (guest redirect; log in, Products, search, no match, Clear search, reload keeps session, log out) |
| Same spec against the React build, same backend | 2 of 2 pass |
| Create Product end to end (dialog, POST, list refresh) | Works (manual Playwright script) |
| `pnpm typecheck` (`strict`, `exactOptionalPropertyTypes`) | Passes |
| Design-system component tests touched by the recipe move | 194 of 194 pass |

Size: this slice is 921 lines of TypeScript excluding the generated types. The React code it replaces (`LoginForm`, `ProductDatatable`, `ProductCreateDialog`, `AnchorDataTable`) is about 1,500 lines, but it also does bulk actions, column visibility, faceted filters, row selection and toasts, which the slice leaves out. The production bundle is 182 kB gzip (520 kB raw) for three screens, most of it the Effect and Effect UI runtime. The full React app is 675 kB gzip, so the two numbers do not compare.

## Findings

What worked well:

- The model is predictable. Every state change is a named message, side effects are Commands, and fetched data stays out of the Model. The auth guard, which is spread over a context, a flow manager and `beforeLoad` hooks in React, is one function and two `update` cases here.
- Queries with `debounce`, `invalidates` tags and kept results replace TanStack Query cleanly. `ui-kit` Dialog and Menu gave correct roles, focus handling, Escape and top-layer rendering with no extra work.
- The unchanged Playwright smoke spec passed against the new build, so the accessible roles and names carried over.

What blocks a full migration:

1. **Preview software.** `@effect/ui` is not on npm. It installs from pkg.pr.new snapshots (pinned here to `290db1a`), and `@effect/ui-kit` needs `effect` 4.1.6, which was a day old (the repository's 7-day release-age rule had to exempt it). The README calls the packages experimental and the API is still changing.
2. **The generated API client does not compile.** Effect UI requires `exactOptionalPropertyTypes`, and the `@hey-api/client-fetch` runtime fails it. This slice keeps hey-api for the request and response *types* only, calls the API with `effect/http`, and hand-writes Effect Schemas for each response. URLs are hand-written and not checked against the contract. A full migration needs an OpenAPI to Effect Schema/HttpClient generator in place of hey-api.
3. **Everything is a rewrite.** `anchor-ui` is about 70,000 lines in 327 files on React 19, TanStack Router, Query, Form and Table, `@xyflow/react`, Monaco and Recharts. None of these run in Effect UI. The data table alone is 805 lines of TanStack Table. Flow diagrams, the code editor and charts would need mounts around the vanilla libraries or SVG/canvas rewrites.
4. **The design system needs a second implementation.** Recipes are shared, but each component's markup, ARIA and behaviour is written again on `ui-kit` parts. The `effect` branch covers about 14 of the 66 components and blocks it would need (button, input, label, field error, heading, text, text link, card, table, dialog and menu styling, sidebar item, app shell, page header), and its `AppShell` has no collapse, mobile sheet or icons (Phosphor icons are React components).
5. **Tooling.** Storybook, the `check:ui` contract, e2e impact selection and coverage tooling all assume React and would need replacements. Effect UI's own `Story`/`Scene` tests would replace component tests.

Not covered by the slice: token refresh (a 401 shows an error instead of refreshing), tenant initialisation (`/init`), registration, toasts and mobile layout.

Recommendation: do not migrate now. Revisit when Effect UI is on npm with a stable API and an OpenAPI generator for Effect exists. The `src/recipes` split in the design system is useful on its own, so it could merge independently.

## Run it

Use the repository's disposable backend (Docker required), then serve this build against it:

```sh
(cd ../anchor-ui && node scripts/e2e-runtime.mjs start --detach)   # prints apiURL; see anchor-ui/e2e/guide
pnpm install && pnpm build
ANCHOR_API_URL=<apiURL> pnpm exec vite preview --port 3020 --host 127.0.0.1
```

A fresh backend has no tenant. Register the first owner with `POST /v1/auth/register` (`email`, `password`, `tenant_name`), then run the smoke spec from `../anchor-ui` with `E2E_BASE_URL=http://127.0.0.1:3020` and that account in `E2E_EMAIL` and `E2E_PASSWORD`. Stop the backend with `node scripts/e2e-runtime.mjs stop`.

`pnpm openapi-ts` regenerates `src/client/types.gen.ts` from `../apps/anchor/cmd/http/openapi.yaml`.
