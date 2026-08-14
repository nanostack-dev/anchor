# Domain Docs

How the engineering skills should consume this repo's domain documentation when exploring the codebase.

**Layout: single-context.** One `CONTEXT.md` + `docs/adr/` at the repo root. `pnpm-workspace.yaml` lists a single package (`anchor-ui`) alongside the Go service in `apps/anchor` — not enough separation to warrant a `CONTEXT-MAP.md`.

## Before exploring, read these

- **`CONTEXT.md`** at the repo root
- **`docs/adr/`** — read ADRs that touch the area you're about to work in.

If any of these files don't exist, **proceed silently**. Don't flag their absence; don't suggest creating them upfront. The `/domain-modeling` skill (reached via `/grill-with-docs` and `/improve-codebase-architecture`) creates them lazily when terms or decisions actually get resolved.

## File structure

```
/
├── CONTEXT.md
├── docs/adr/
│   ├── 0001-....md
│   └── 0002-....md
├── apps/anchor/          ← Go OaaS service
└── anchor-ui/            ← frontend
```

## Use the glossary's vocabulary

When your output names a domain concept (in an issue title, a refactor proposal, a hypothesis, a test name), use the term as defined in `CONTEXT.md`. Don't drift to synonyms the glossary explicitly avoids.

If the concept you need isn't in the glossary yet, that's a signal — either you're inventing language the project doesn't use (reconsider) or there's a real gap (note it for `/domain-modeling`).

## Flag ADR conflicts

If your output contradicts an existing ADR, surface it explicitly rather than silently overriding:

> _Contradicts ADR-0007 (event-sourced orders) — but worth reopening because…_

## Renumber on a numbering collision, not on a content conflict

A parallel PR can land its own ADR at the same number yours claims, independently — different filenames, so git won't flag it as a conflict on rebase, but two decisions can't share one number. This is not the content conflict the section above is about; it's purely two authors picking the same next-free number at the same time. When it happens: renumber yours to the actual next-free number, and update every reference — the ADR's own title, `CONTEXT.md` glossary links, and any code, migration, or OpenAPI comment pointing at the old path.

## Existing docs worth reading alongside

These predate this setup and are not ADRs, but they carry real decisions:

- `docs/engineering-best-practices.md` — cross-repo rules, kept identical with echopoint
- `docs/api-key-permission-cache.md`
- `docs/api-key-prefix-config.md`
- `docs/case-insensitive-identifiers.md`
- `docs/organization-workspaces.md`
