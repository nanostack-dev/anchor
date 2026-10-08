# Local setup

Clone Anchor alone and read [AGENTS.md](../../AGENTS.md). No shared workspace, external skill pack or sibling repository is required. For implementation, fetch `origin` and create an isolated worktree from `origin/main`.

## Agent harness discovery

Codex, OpenCode and Grok Build read this repository's `AGENTS.md` directly. Claude Code's repo-owned [.claude/settings.json](../../.claude/settings.json) adds a read-only `SessionStart` hook: Git resolves the current project's root, then the hook prints that root's `AGENTS.md` as session context. It works from nested directories and does not call workspace bootstrap or require Python/Node. Existing hooks/preferences remain additive; developer overrides belong in ignored `.claude/settings.local.json`.

Approve the repository through the client's normal one-time trust flow before project hooks run. After pulling a changed hook or guide, start/reload the session so the current instructions are loaded. Login, trust and personal/global settings remain developer-controlled.

## Prerequisites and install

Use Go 1.27 (`apps/anchor/go.mod`), Node 24 and pnpm 11.1.0 (`anchor-ui/package.json` and the [application E2E workflow](../../.github/workflows/e2e-app.yml)). Docker with Compose and an accessible daemon are required for integration/browser runtime. API/database generation also requires `migrate` and `openssl` on PATH.

```sh
# From the repository root:
pnpm install --frozen-lockfile
cd anchor-ui
pnpm exec playwright install chromium
```

The root `pnpm-workspace.yaml` and `pnpm-lock.yaml` own frontend resolution. Run Go/lint commands from `apps/anchor`; the repository root is not a Go module. [Dependency ownership](../technical/dependencies.md) explains published library pins and the in-repo Go client replacement.

## Recommended disposable app

The [managed browser runtime](../../anchor-ui/e2e/README.md) builds this worktree and starts real Anchor, TimescaleDB, Redis and Mailpit, generating local credentials and bootstrapping accounts through public APIs. It needs no shared deployment credentials or external identity-provider account.

```sh
# From anchor-ui:
pnpm test:e2e:app
# For an interactive owned preview and test runner:
pnpm test:e2e:ui
```

For repeated local development, `node scripts/serve-e2e-full.mjs` keeps the managed preview at `http://127.0.0.1:3015`; use the browser guide's ownership/bootstrap procedure. Close its owning manager with Ctrl-C, then verify cleanup with `node scripts/e2e-runtime.mjs verify-stopped`. Runtime metadata, generated credentials and reports stay ignored.

## Existing manual configuration

[apps/anchor/application.yaml](../../apps/anchor/application.yaml) is the current backend configuration contract. It reads secret files through `POSTGRES_PASSWORD_FILE`, `ADMIN_JWT_SECRET_FILE`, `APP_ENCRYPTION_KEY_FILE` and `REDIS_PASSWORD_FILE`; a plain password value alone does not satisfy a required `${file:...}` input. Provide approved local files and nonsecret connection/origin settings when using an independently provisioned stack. Startup owns migrations from `apps/anchor/migrations`.

The legacy root `.env.example` and `docker-compose.yml` are older generic samples, not the complete current setup contract. Use the managed runtime above for newcomer/agent work rather than assuming their old inline-secret examples start current Anchor. A shared workspace can help locate repositories but adds no prerequisite to this procedure.
