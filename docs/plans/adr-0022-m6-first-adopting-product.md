# M6 · First adopting Product: plan

Status: proposed. First Product: Echopoint (see [Echopoint](#echopoint)). Milestone [#265](https://github.com/nanostack-dev/anchor/issues/265) of [ADR-0022](../adr/0022-resource-types-scope-roles-and-permissions.md#dr-g4), after M5 (#264).

## Goal

One Product moves from Organization-wide roles to roles on its own resources, using only the generic API that M1 to M5 shipped. It decides every request through Anchor, and nothing in Anchor is specific to it.

**Done when** all of these hold in production for the adopting Product:

1. A role assigned on a container grants the resources below it and nothing above it, end to end through the Product's own UI.
2. Every access decision the Product makes for a resource goes through the evaluation API with the resource's type, with no fallback to an Organization-wide check.
3. The Product's flow suite covers registration, a move, a cascade delete, a role assignment, a revocation and a decision on an unregistered resource named by its parent, and passes against dev and prod.
4. Over the agreed observation period after the switch-over, Anchor's logs show no `authz.decision_drift`, no `authz.mark_stale` and no `ACCESS_DECISIONS_UNAVAILABLE` for the Product.

## What the Product does

| Step | Generic API | Records |
| :-- | :-- | :-- |
| Declare its resource type tree, scoped permissions and the roles it manages | Terraform provider: `anchor_product_resource_type`, `anchor_product_permission` and `anchor_product_role` with `resource_type` | A1–A5, C1, D1, D8, G5 |
| Register each container someone holds a role on, top-down, and each shared item | `PUT …/organizations/{organization}/resources/{type}/{id}` with `name` and `parent` | B1–B3 |
| Re-send the name on rename, the parent on move, and delete with `cascade=true` once an item is gone from its own database | the same `PUT`; `DELETE …?cascade=true` | B2–B4 |
| Assign and revoke roles on a resource | `PUT …/resources/{type}/{id}/members/{product_user}` with `role_ids` (one role while `roles_per_resource` is `single`) | D3, D4 |
| Decide a request | `POST …/access/v1/evaluation` with `resource.type`; `properties.parent` for an unregistered item; `options.explain` for support tools | E1, E2 |
| Let customers write their own roles, if it wants | `custom_roles` on the types it opens | D9, D10 |
| React to access changes | `resource.role_assignment.*` webhooks | G3 |

The Product never stores an Anchor ID: resources are named by its own IDs (B2).

## Slices

Each slice is one PR in the repository it changes, and is one kind: invisible, new behind a switch, or a switch-over.

| # | Repository | Slice | Kind |
| :-- | :-- | :-- | :-- |
| 1 | infra | The Product's catalog in Terraform: types, scoped permissions, managed roles | invisible |
| 2 | Product | A client for the resource and assignment routes, from the published Go client | invisible |
| 3 | Product | Registration of containers that hold roles, behind a Product switch; a backfill for existing ones, top-down and idempotent | new behind a switch |
| 4 | Product | Sharing UI and API that assign roles on a resource, behind the same switch | new behind a switch |
| 5 | Product | Shadow, only when slice 6 can deny someone the current check allows (see below): each request also asks Anchor with the resource's type and logs a difference from the Product's current check; the current check still answers | invisible |
| 6 | Product | Switch: decisions come from Anchor's resource checks; the old check is removed | switch-over |
| 7 | Product | The flow suite cases in "Done when" 3 | invisible |

**When to shadow.** Slice 5 exists to find the users the new check would lock out, such as a member the backfill missed. It is needed only when slice 6 can deny a request the current check allows on resources that people use. When it is needed, slice 6 merges only once slice 5 shows no difference in dev over the agreed period, the same gate as ADR-0022 G4's PR 7. When it isn't, slice 6 follows slice 4 directly, and the flow suite cases of slice 7 are the gate.

## Rollout rules

- **Released clients.** A change that makes Anchor refuse a request a released client sends keeps a server-side default until that client ships (the cross-repo rollout rule in `AGENTS.md`). None of the slices above changes Anchor's contract.
- **Ordering.** Registration is top-down: a parent that isn't registered yet answers `409 PARENT_NOT_REGISTERED`. The backfill walks each Organization's containers from the root.
- **Per-Organization switch.** The Product's switch is per Organization, so one customer can adopt before the rest.

## Risks and their answers

| Risk | Answer |
| :-- | :-- |
| Bulk registration of the Product's existing containers | Each registration syncs only its own object: 6 to 8 ms per write whether the Organization holds 10 or 1,000 resources, and writes in different Organizations, or to different resources, sync side by side. Register top-down, Organization by Organization. |
| The sweeper rebuilds one Product every 5 s, which covers 15-minute rebuilds only up to about 180 Products | Writes reach the store after their commit; the rebuild is a backup. Watch `authz.mark_stale` and raise the rebuild rate before the Product count passes that. |
| A name drifts when the Product forgets to re-send it | It affects display only, never a decision (B2). |
| A deep self-nesting tree hits the limit of 15 levels | `409 RESOURCE_TOO_DEEP` at registration names the limit; the Product flattens below it. |
| A forgotten delete leaves a stale resource with its assignments | `cascade=true` deletes the subtree and its assignments in one transaction and emits their events. |
| The Product's own types count toward the limit of 50, a `workspace` type it declares included: Anchor has no built-in workspaces since A6 phase 4 | Plan the tree within 50 own types, or ask for the limit to be raised before slice 1. |
| Decisions read OpenFGA, which a write reaches before its response, or within seconds when its replay can't run | Watch `authz.mark_stale` and `authz.decision_drift` in Anchor's logs during the shadow slice; both should stay at zero. |

## Echopoint

Echopoint adopts first. Workspaces are its only container type, and almost no customer uses more than the default one.

**Where it starts.** Echopoint already did part of slices 1 to 3 for the `workspace` type, with Organization-wide roles:

- infra declares `anchor_product_resource_type.workspace` under `organization`.
- Echopoint registers each workspace through the SDK resource API, keeps no copy of it, and gives every Organization a "Default" workspace (echopoint#694).
- Echopoint asks every check on `workspace/{X-Workspace-Id}`. Each of its permissions is scoped to `organization`, and the one managed role, `admin`, holds them all, so the parent chain answers (ADR-0022, "a resource type answers a permission scoped above it through its parent").

Anchor decides every request and is the only store of workspaces, their members and their roles. Echopoint keeps no copy, so there is nothing to sync, roll back or repair. See [Workspace membership](#workspace-membership).

**What is left.**

| # | Repository | Slice | Kind |
| :-- | :-- | :-- | :-- |
| 1 | infra | Workspace-scoped permissions for what lives in a workspace, and managed roles on `workspace` that hold them (for example `workspace_viewer` and `workspace_editor`). The Organization role `admin` also holds them, so every current member keeps access. Custom roles open on `workspace` (D9). | invisible |
| 3 | Echopoint | The member who creates a workspace gets a workspace role on it | invisible |
| 4 | Echopoint | Workspace settings: create, rename and delete a workspace; add, change and remove its members, with managed or custom roles. Each change is one SDK call. | new behind a switch |
| 6 | Echopoint | Checks ask the workspace-scoped permissions; the Organization-scoped ones they replace leave the catalog | switch-over |
| 7 | Echopoint | The flow suite cases in "Done when" 3, on workspaces | invisible |

**No shadow slice.** Slice 1 gives `admin` every new permission, so slice 6 denies no current member. A workspace role only adds access for a member who has no Organization role, which a membership allows (D5).

### Workspace membership

Anchor holds each workspace membership. Each Echopoint screen reads Anchor, and each change is one synchronous SDK call:

| Echopoint | Anchor |
| :-- | :-- |
| Members of a workspace | `POST …/resources/workspace/{id}/members/search` gives each member's product user and roles; `POST …/organizations/{organization}/members/search` with `filter.product_user_ids` adds names and emails |
| Workspaces a member can open | `POST …/resources/search` with `resource_types: [workspace]`, then one `POST …/access/v1/evaluations` for all of them |
| Add or change a member's role | `PUT …/resources/workspace/{id}/members/{product_user}` with `role_ids` |
| Remove a member | `DELETE …/resources/workspace/{id}/members/{product_user}` |

Anchor writes a member change to the Product's OpenFGA store before it answers (ADR-0022 E5), so a `2xx` means decisions already reflect it, and the UI shows the result at once. `PUT` and `DELETE` are idempotent, so retrying a failed or timed-out change is safe.

**Everyone in the Organization.** A role on the Organization that holds workspace-scoped permissions grants every workspace in it, present and future, through the parent chain. The member list of a workspace shows only the roles held on that workspace; Organization roles that also reach it come from the Organization's member list, or from `options.explain`.

**Later, if needed.** Resource search can't filter by member, so "workspaces a member can open" asks about every workspace of the Organization. That is one batch call while Organizations hold a few workspaces. If they grow to many, Anchor adds a member filter to resource search.

**Delete.** Echopoint deletes a workspace's own data, then deletes the resource with `cascade=true`. A workspace deleted in Anchor's admin UI reaches Echopoint through a webhook, and Echopoint deletes its data then.

## Open questions

For Echopoint, before slice 1:

- Which permissions move to `workspace`? A proposal: `collections:*`, `environment:*`, `flows:*`, `specs:*`, `status_pages:*` and `webhooks:*` move. `api_key:*`, `invitations:*`, `members:*` and `runner:*` stay on `organization`.
- A permission's scope can't change in place, and replacing one deletes it from every role and Organization API key that holds it. Do the workspace-scoped permissions take new names, or replace the current ones once no Organization API key holds them?
- Which managed roles go on `workspace`? (Custom roles open on it.)
- What observation period after slice 6 closes M6?
