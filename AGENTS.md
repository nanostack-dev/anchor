# Anchor Agent Guide

This repository owns its agent rules and documentation and works as a standalone clone. A Nanostack workspace and installed shared skills are optional helpers; do not require their files, sibling checkouts or bootstrap to implement a task here. All paths below are repository-relative.

## Read when

- Exploring domain behavior or naming a concept: read [CONTEXT.md](CONTEXT.md), relevant [ADRs](docs/adr/) and [architecture](docs/technical/architecture.md).
- Setting up this checkout or an isolated runtime: read [setup](docs/development/setup.md); selecting validation or changing API assertions: read [testing](docs/development/testing.md).
- Before implementation: read [implementation rules](docs/development/agent-rules.md), [engineering practices](docs/engineering-best-practices.md), [Go conventions](docs/development/go-conventions.md) and [delivery workflow](docs/development/agent-workflow.md). They own contract-first generation, tenant scope, background durability, test conventions and completion gates.
- Browser-visible work (including a backend change affecting a UI journey): read [anchor-ui/AGENTS.md](anchor-ui/AGENTS.md), the app browser guide and [.claude/skills/anchor-feature-review/SKILL.md](.claude/skills/anchor-feature-review/SKILL.md). The adapter and required review procedure are repo-owned.
- Changing a shared library, generated client or deployment input: read [dependency ownership](docs/technical/dependencies.md), then the owning repository's current guide and source. Links work without sibling clones; use published module/package pins for normal builds.
- Diagnosing setup/test problems: read [troubleshooting](docs/development/troubleshooting.md). Releasing or recovering a release: read [deployment](docs/runbooks/deployment.md) or [rollback](docs/runbooks/rollback.md).
- Finding capability, component, issue-tracker or triage guides: start at [docs/README.md](docs/README.md).

## Work and documentation

Fetch this repository and implement in an isolated worktree from `origin/main`; preserve primary-checkout changes. Run Go commands in `apps/anchor` and frontend commands in `anchor-ui`. Generated files change only through their owning generators.

Update authoritative docs in the same PR for changed behavior, a verified reusable fix, resolved vocabulary or a consequential architectural choice. Preserve ADR history and component-local docs, link new documents from the index, and verify links/command provenance. Record evidence rather than hypotheses or sensitive logs. The [delivery workflow](docs/development/agent-workflow.md) supplies the local procedure even when no skill is installed.

Follow [.github/pull_request_template.md](.github/pull_request_template.md), preserving every section and the preview marker. Commit/push from an isolated worktree are allowed; complete current-head CI and applicable review before calling work ready. A merged PR alone does not establish deployment.
