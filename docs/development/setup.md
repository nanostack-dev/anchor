# Local setup

Clone Anchor alone and read [AGENTS.md](../../AGENTS.md). No shared workspace, external skill pack or sibling repository is required. For implementation, fetch `origin` and create an isolated worktree from `origin/main`.

## Agent harness discovery

Codex, OpenCode and Grok Build read this repository's `AGENTS.md` directly. Claude Code's repo-owned [.claude/settings.json](../../.claude/settings.json) adds a read-only `SessionStart` hook: Git resolves the current project's root, then the hook prints that root's `AGENTS.md` as session context. It works from nested directories and does not call workspace bootstrap or require Python/Node. Existing hooks/preferences remain additive; developer overrides belong in ignored `.claude/settings.local.json`. Two more hooks call [scripts/agent-hooks](../../scripts/agent-hooks): after an Edit or Write, `format-edited.sh` formats that one file (the pinned golangci-lint for Go, Biome for `anchor-ui` sources) and never blocks; before a Bash call, `deny-ci-polling.sh` refuses a `sleep` loop around `gh` checks and names the non-blocking alternative.

Approve the repository through the client's normal one-time trust flow before project hooks run. After pulling a changed hook or guide, start/reload the session so the current instructions are loaded. Login, trust and personal/global settings remain developer-controlled.

## Prerequisites and install

Use Go 1.27 (`apps/anchor/go.mod`), Node 24 and pnpm 11.1.0 (`anchor-ui/package.json` and the [application E2E workflow](../../.github/workflows/e2e-app.yml)). Docker with Compose and an accessible daemon are required for integration/browser runtime. Database model generation (`go tool nanostack gen`) needs Docker only when the migrations or the framework version changed, or a server named by `NANO_GEN_DATABASE_URL`.

```sh
# From the repository root:
pnpm install --frozen-lockfile
cd anchor-ui
pnpm exec playwright install chromium
```

The root `pnpm-workspace.yaml` and `pnpm-lock.yaml` own frontend resolution. Run backend Go commands from `apps/anchor`; the repository root is not a Go module. The repository lint script resolves the module automatically. [Dependency ownership](../technical/dependencies.md) explains published library pins and the in-repo Go client replacement.

## Go linter

golangci-lint is a Go tool dependency of its own module, [`apps/anchor/tools/golangci-lint/go.mod`](../../apps/anchor/tools/golangci-lint/go.mod), so its dependencies never change the application's. Nothing to install: Go builds the pinned version on first use and keeps it in the build cache.

```sh
# From the repository root:
make lint        # same as ./scripts/lint.sh backend
# The pinned linter directly:
(cd apps/anchor && go tool -modfile=tools/golangci-lint/go.mod golangci-lint version)
```

The lint script, the edit hook and CI (`dag://anchor/lint`) all run that build, with the toolchain the tool module names; a global `golangci-lint` on `PATH` is never used. It is not interchangeable with the release binary, which is built with `-trimpath`: on the same code and version, the release reported 0 revive `time-naming` findings where this build reported 2. To change the version, run `go get -tool github.com/golangci/golangci-lint/v2/cmd/golangci-lint@<version>` in a fresh tool module (a downgrade in place keeps newer dependencies, and the build fails). The default checks without editing source; fixes are explicit. [Go lint](testing.md#go-lint) owns full/iteration commands, comparison bases, fallbacks and failure handling.

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

The backend binds its typed configuration (`apps/anchor/cmd/app.Config`) from the environment, then the YAML file of `CONFIG_PATH` (or [apps/anchor/application.yaml](../../apps/anchor/application.yaml)), then the defaults. [apps/anchor/cmd/app/CONFIG.md](../../apps/anchor/cmd/app/CONFIG.md) lists every setting with its env variable and default. A config file holds no secret and no `${...}` placeholder: set `POSTGRES_PASSWORD`, `REDIS_PASSWORD`, `ADMIN_JWT_SECRET` and `APP_ENCRYPTION_KEY` in the environment, or their `*_FILE` variables with a path to a file. Provide the nonsecret connection settings (`POSTGRES_HOST`, `POSTGRES_USER`, `POSTGRES_DB`, ...) when using an independently provisioned stack. Startup owns migrations from `apps/anchor/migrations`.

The legacy root `.env.example` and `docker-compose.yml` are older generic samples, not the complete current setup contract. Use the managed runtime above for newcomer/agent work rather than assuming their old inline-secret examples start current Anchor. A shared workspace can help locate repositories but adds no prerequisite to this procedure.
