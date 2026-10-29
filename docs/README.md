# Anchor documentation

Start with [AGENTS.md](../AGENTS.md) for task-specific pointers and [CONTEXT.md](../CONTEXT.md) for canonical domain vocabulary. This index and all required procedures are usable from a standalone clone; the Nanostack workspace only helps locate repositories and optional skills.

| Need | Owner |
| --- | --- |
| Current system and dependency boundaries | [Architecture](technical/architecture.md), [dependency ownership](technical/dependencies.md) |
| Consequential decisions and rationale | [Existing ADR history](adr/) |
| Local prerequisites/runtime and native agent discovery | [Setup](development/setup.md) |
| Test selection, generators and CI | [Testing](development/testing.md), [deployed flow suites](development/flow-suite-testing.md) |
| Engineering and delivery requirements | [Implementation rules](development/agent-rules.md), [Go conventions](development/go-conventions.md), [engineering practices](engineering-best-practices.md), [agent workflow](development/agent-workflow.md), [review standards](../CODING_STANDARDS.md) |
| Verified developer repairs | [Troubleshooting](development/troubleshooting.md) |
| Release and recovery | [Deployment](runbooks/deployment.md), [rollback](runbooks/rollback.md) |
| Frontend/component/browser detail | [anchor-ui/AGENTS.md](../anchor-ui/AGENTS.md), [anchor-ui documentation](../anchor-ui/docs/) and [browser test guide](../anchor-ui/e2e/guide/README.md) |
| Issues, triage and vocabulary maintenance | [Issue tracker](agents/issue-tracker.md), [triage labels](agents/triage-labels.md), [domain docs](agents/domain.md) |

## Capability and reference documents

Existing authoritative paths remain valid; new current-behavior documents belong in `technical/`. Some older specs contain proposals as well as shipped behavior, so check their status and owning source before asserting that a feature exists.

- [api key permission cache](api-key-permission-cache.md)
- [Access evaluation](technical/access-evaluation.md): the AuthZEN-shaped call a product makes to decide one request, its reasons and freshness
- [Cursor-paginated searches](technical/cursor-pagination.md): the cursor contract, every search with its order, keys and index, the deprecated lists and the tests
- [Clerk reconcile scheduler](technical/clerk-reconcile-scheduler.md): one self-rescheduling chain, duplicate collapse and the per-instance lock
- [api key prefix config](api-key-prefix-config.md)
- [case insensitive identifiers](case-insensitive-identifiers.md)
- [Distributed tracing](technical/tracing.md): OpenTelemetry wiring, OTEL_* configuration, span naming and which outbound calls forward trace context
- [Product-role permission updates](technical/role-permission-updates.md): omission, empty replacement and generated SDK semantics
- [Resource types](technical/resource-types.md): a product's tree of resource types, the scope of every role and permission, and the scope rollout
- [Managed and custom roles](technical/managed-and-custom-roles.md): roles the Product manages, an Organization's own roles, who may hold each, and their rollout
- [Authorization engine](technical/authorization-engine.md): OpenFGA running inside Anchor, each Product's store, and the steps that move decisions onto it; [sync flow](technical/authorization-sync-flow.html), a step-by-step view of the tables one membership write changes; [authorization sync](technical/authorization-fgasync.md), the sync engine and how to add a source
- [engineering best practices](engineering-best-practices.md)
- [organization workspaces](organization-workspaces.md)
- [product events spec](product-events-spec.md)

Postmortems: [2026-10-07 duplicate Clerk reconcile](postmortems/2026-10-07-duplicate-clerk-reconcile.md), [2026-10-16 concurrent API key creates answered 500](postmortems/2026-10-16-api-key-name-race-500.md). Each has its `.json` source and the standalone `.html` report beside it.

[Research](research/) contains sourced unresolved proposals; read each document's status before treating it as implementation.

## Maintain alongside implementation

Update the owning page in the same PR when behavior, setup, validation, a reusable fix or domain language changes. Keep component documentation beside its owner and link it here. Preserve ADR numbering/history; separate current technical behavior from rationale, proposals and incidents. Verify links and commands against final source and retain no credentials or sensitive raw logs. The [local workflow](development/agent-workflow.md) defines completion without a required external skill.

- [Domain glossary](../GLOSSARY.md)
