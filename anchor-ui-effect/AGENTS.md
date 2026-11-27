# Agent Guide: anchor-ui-effect

Experimental Effect UI rewrite of a slice of `anchor-ui`. Read [README.md](README.md) for scope, findings and how to run it. It is not deployed and CI does not build it.

- It is a standalone pnpm project (own `pnpm-workspace.yaml` and lockfile), outside the repository workspace. Run commands in this folder.
- Verify with `pnpm typecheck` and `pnpm build`, then run `../anchor-ui/e2e/smoke/login.e2e.ts` against a preview of this build (README, Run it).
- Effect UI's own guide ships in `node_modules/@effect/ui/AGENTS.md`; follow it for naming and component structure.
- `src/client/types.gen.ts` is generated (`pnpm openapi-ts`); do not edit it. API calls live in `src/services/AnchorApi.ts` with hand-written response Schemas in `src/domain/`.
- Styling comes only from `@nanostackorg/design-system/effect`, pinned to a commit of the design system's `effect` branch.
