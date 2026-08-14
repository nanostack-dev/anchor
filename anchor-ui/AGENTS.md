# Agent Guide: anchor-ui

Admin dashboard for the Anchor OaaS platform (Vite + React + TanStack Router/Query).

## Generated client — `src/client/` (do not edit)

`@hey-api/openapi-ts` generates `types.gen.ts`, `zod.gen.ts` (schemas prefixed `z`), and `@tanstack/react-query.gen` from `../apps/anchor/cmd/http/openapi.yaml`. Config: `openapi-ts.config.ts`.

Regenerate after **any** OpenAPI change, before writing frontend code:

```sh
./apps/anchor/generate_anchor.sh   # from the anchor app root
pnpm generate                      # then, inside anchor-ui
```

- Validate forms with the generated Zod schemas — never hand-write validation for a field that exists in the spec. The generated schemas mark every field optional (OpenAPI default), so layer `.superRefine()` on top for required-field rules and UX messages.
- API fields are snake_case, form state is camelCase — map when surfacing field errors.

## Forms & state

- Local form state: `useState` + generated Zod schema. Server state: TanStack Query.
- Write-only fields (passwords, secrets) are never pre-populated from a server response; guard with `useRef` so query invalidation does not reset the form.

## Testing

Canonical strategy: `nanostack-registry/docs/testing-strategy.md`. anchor-ui is an **app**: Storybook component tests for reusable UI + Playwright e2e per feature (same pattern as `echopoint/apps/frontend/e2e/`). Run the mutating e2e suite against a local backend, never prod.

Query by role/accessible name — no CSS/XPath selectors, no snapshot churn.

## Code style

- Avoid comments — name variables and functions clearly instead. Comment only a genuinely complex algorithm.

## UI work

Load the `anchor-ui-design` skill before adding or reshaping components — surface/elevation rules, semantic tokens, and feedback states live there. Light mode only: never author `dark:` classes.
