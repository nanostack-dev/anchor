# Resources

A Product registers **resources**, references to its own projects, folders or documents, so a role can later be held on one of them and inherited by everything under it ([ADR-0022](../adr/0022-resource-types-scope-roles-and-permissions.md) B1–B4, G3). Anchor stores a reference only: its type, the Product's identifier, a display name and its one parent. Members of the resource's Organization hold roles on it through [role assignments](#role-assignments). [Access evaluation](access-evaluation.md#resources) checks a registered resource under its stored chain, and an unregistered one under the parent the request names.

The registry is sparse (B1): a Product registers a resource only when a role is held on it, together with the ancestors it hangs from. Any other item is checked by naming its nearest registered ancestor.

Code: `internal/resource` (module, repository, service), `internal/domain/resource`, `internal/api/resource_api.go`. Table: `resources`, migration `000072`.

## Identity

A resource is named by `(Organization, type, id)`, where `id` is the Product's own identifier (B2). The Product never stores an Anchor ID, and responses never show one.

| Field | Rule | Refusal |
| --- | --- | --- |
| `id` | 1 to 200 characters from `A–Z a–z 0–9 . _ ~ -`, the URL unreserved set, so it is a path segment as is; `.` and `..` alone are refused. Unique per Organization and type. | `400 INVALID_RESOURCE_ID` |
| `type` | A type the Product declared, other than `organization`, which is the root of every chain and never a registered resource. The same answers hold on every method. | `404 RESOURCE_TYPE_NOT_FOUND`, `404 RESOURCE_TYPE_NOT_REGISTRABLE` |
| `name` | Required, 1 to 200 characters, not unique. | `400 VALIDATION_ERROR` |
| `parent` | Required on every call, `{ "type", "id" }`. The Organization is `{ "type": "organization", "id": "<organization_id>" }`; Anchor never defaults to it. | `400 VALIDATION_ERROR` |

Anchor's own row ID (`res_…`) names the resource in the relationship store, because the Product's identifier is unique only in its Organization and type.

## Routes

All under `/v1/products/{product_id}/organizations/{organization_id}/resources`. An Organization the Product does not have answers `404 ORGANIZATION_NOT_FOUND`. A registration or a delete reads the Organization `FOR KEY SHARE` first, so an Organization deleted while the call runs answers the same 404, never a foreign key error.

| Route | Does | Credential |
| --- | --- | --- |
| `PUT /{resource_type}/{resource_id}` | Creates, renames or moves the resource and answers `200` with it. The same body twice changes nothing and emits nothing the second time. | Product API key with `resource:write` |
| `GET /{resource_type}/{resource_id}` | Reads it; `404 RESOURCE_NOT_FOUND` when nothing is registered under that identity in the Organization. | Platform bearer, or Product API key with `resource:read` |
| `DELETE /{resource_type}/{resource_id}` | Deletes it. A resource with registered children or role assignments answers `409 RESOURCE_IN_USE` with `children` (direct children) and `role_assignments` (the assignments held on it), unless `?cascade=true`, which deletes its whole subtree and every role assignment on it in one transaction. | Product API key with `resource:delete` |
| `POST /search` | Cursor search. Filters: `resource_types`, `parent` (direct children; the Organization means the top level), `ids`, `name_contains` (case-insensitive, `%` and `_` literal). A slug or a parent the Organization lacks matches nothing. Sort by `created_at` (default, ascending) or `name`; every order ends with `id`, then Anchor's row. Indexes `(organization_id, created_at, external_id, id)` and `(organization_id, name, external_id, id)`. At most 50 `resource_types`, 100 `ids` and 200 characters of `name_contains`. | Platform bearer, or Product API key with `resource:read` |

## Parent rules

Every rule runs in the service, inside the write's transaction (B3, A4):

| Rule | Refusal |
| --- | --- |
| The parent's type is one of the type's `parent_types`, or the type itself when it nests in itself. | `400 INVALID_RESOURCE_PARENT` with `allowed_parent_types` |
| The parent is registered in the same Organization. A parent registered elsewhere, or another Organization named as parent, is not registered here. | `409 PARENT_NOT_REGISTERED` with `parent` |
| The new parent is not the resource itself nor one of its descendants. | `409 RESOURCE_CYCLE` with `chain`, from the new parent up to the resource |
| The chain, counting the Organization, is at most 15 levels (`resource.MaxDepth`), measured at the moved resource's deepest descendant. | `409 RESOURCE_TOO_DEEP` with `max_depth` and `depth` |

`PARENT_NOT_REGISTERED` is a 409, against the house rule that an entity named in the body and not found is a 400. ADR-0022 B3 chose it: the request is well-formed and the same call succeeds once the Product registers the parent, since parents register before their children. That is the "a later request can succeed" row of the status table.

A move carries its whole subtree: children keep their parent row. No depth is stored. The checks walk the chain with recursive queries: the new parent's ancestors (at most 15 rows), which give the cycle and the parent's depth, and `max(distance)` over the moved resource's subtree, which gives its height. The checks run only when the parent changes, so a rename reads nothing more.

## Locks

- **The Organization row, `FOR KEY SHARE`.** It keeps the Organization from being deleted until the write commits.
- **Catalog, in share mode.** A registration, a move and a delete read the tree through `resourcetype` `LockedTree`, so no type change can remove an allowed parent type before the write commits.
- **The Organization's resources.** Then each takes a transaction advisory lock on `resources:<organization_id>`. Every registration, move and delete in one Organization runs one at a time, and the chain it reads after the lock is the one the previous write left. Two moves that would put two resources under each other therefore can't both pass the cycle check, concurrent registrations of one new resource all answer `200` with one row, a registration racing the removal of its parent type never leaves the resource under it, and registrations racing their Organization's delete answer 200 or 404 (`TestConcurrentResourceWrites`). Locking only the moved resource's subtree root would not be enough: the two moves of a cycle lock two different roots.
- Writes in different Organizations never wait for each other.

## Deletion

- A cascade deletes the resource row; the parent foreign key removes its descendants in the same statement, and the resource foreign key their role assignments. One `resource.role_assignment.deleted` is emitted per assignment removed, then one `resource.deleted` per deleted resource, deepest first.
- Deleting an Organization or a Product removes its resources through their foreign keys, with no `resource.deleted` events and so no resource marks. `parent_resource_id` and `resource_type_id` cascade so those deletes never trip on the order in which rows go. The resources' tuples stay in the store until the Product's next rebuild removes them; nothing reads them meanwhile, since no row names their objects and row IDs are never reused.
- Deleting a resource type is refused while resources use it (below); the type's cascade only serves a Product delete.

## Resource types in use

Resources extend the resource type rules of ADR-0022 A5, in `internal/resourcetype/service`, for the single-type routes and the tree `PUT` alike, under the exclusive catalog lock:

| Change | Refused while | Refusal |
| --- | --- | --- |
| Delete a type, or leave it out of a published tree | resources of the type are registered | `409 RESOURCE_TYPE_IN_USE`, `metadata.blocked_by.resources` |
| Remove a parent type | a resource of the type sits under one of that parent type (`organization`: at the top) | `409 RESOURCE_TYPE_IN_USE`, `metadata.parent_type` and `metadata.blocked_by.resources` |
| Turn `nests_in_itself` off | a resource of the type sits under one of its own type | the same, with `parent_type` the type itself |

`blocked_by.resources` appears only when it is above zero, so the counts of a delete refused for child types, roles or permissions keep their earlier shape.

## Events

`resource.created`, `resource.updated` and `resource.deleted` carry `organization_id`, `resource_type`, `resource_id` (the Product's identifier) and `resource_row_id` (Anchor's row, which names the object in the relationship store; the emitter drops it from any event it delivers). An unchanged `PUT` emits nothing. They are emitted in the write's transaction and stay inside Anchor (G3): their catalog definitions are marked `Undelivered`, so the emitter queues nothing for them, the product's event catalog does not list them, and an endpoint can't subscribe to them (`400 UNKNOWN_EVENT_TYPE`). The transaction observers still see them, which is how the relationship store follows the registry.

## Relationship store

Each resource is mirrored as one tuple, `rt_<slug>:<row id> # parent @ rt_<parent slug>:<parent row id>`, or `@ organization:<organization_id>` at the top ([Authorization engine](authorization-engine.md#tuples)). The `authz` marker turns each `resource.*` event into a mark on that resource's object in `authz_resource_marks`, and the request's replay after commit runs a resource sync of exactly those objects: their rows give the desired tuples, a read of each object gives the stored ones, and the diff is applied. A registration, a rename that moves, a move and a delete therefore touch their own object only; a cascade touches each deleted resource once. A move leaves its descendants' tuples alone, since each still names its own parent. Organization syncs never read or write resource tuples.

## Role assignments

A member of the Organization holds a role on a registered resource (ADR-0022 D3, D4). Code: `internal/roleassignment` (module, repository, service), `internal/domain/roleassignment`, `internal/api/resource_member_api.go`. Table: `resource_role_assignments`, migration `000073`, keyed by `(resource_id, product_user_id, role_id)`, so a member can later hold several roles on one resource without a new key.

The contract is list-shaped: a request sends `role_ids`, and responses and searches answer `roles: [{ id, name }]`. A Product allows one role per member per resource (`roles_per_resource: single`, Q-D3.2). That setting doesn't exist yet: `single` is the only value, so a request names exactly one role.

### Routes

Under `/v1/products/{product_id}/organizations/{organization_id}/resources/{resource_type}/{resource_id}/members`. The Organization, the type and the resource answer as on the resource routes: `404 ORGANIZATION_NOT_FOUND`, `404 RESOURCE_TYPE_NOT_FOUND`, `404 RESOURCE_TYPE_NOT_REGISTRABLE`, `404 RESOURCE_NOT_FOUND`.

| Route | Does | Credential |
| --- | --- | --- |
| `PUT /{product_user_id}` | Replaces the member's roles on the resource with `role_ids` and answers `200` with `{ product_user_id, roles, created_at, updated_at }`. The same role twice changes nothing and emits nothing. A replaced role keeps `created_at`, when the member first held a role there. | Product API key with `resource_member:write` |
| `GET /{product_user_id}` | Reads the roles held on this resource only; inherited ones are not listed. `404 RESOURCE_MEMBER_NOT_FOUND` when the member holds none here. | Platform bearer, or Product API key with `resource:read` |
| `DELETE /{product_user_id}` | Removes every role the member holds on the resource; `204`. The membership and the roles on other resources stay. `404 RESOURCE_MEMBER_NOT_FOUND` as on the read. | Product API key with `resource_member:write` |
| `POST /search` | Cursor search over the resource's members. Filters: `product_user_ids` (at most 100) and `role_ids` (at most 50; a template's ID names the Organization's copy). Sort by `created_at` (default, ascending) or `product_user_id`; every order ends with `product_user_id`. Each member is paged by one assignment, the one with the lowest role row, then reads all their roles; while a member holds one role per resource that row is their only one. `roles_per_resource: multiple` must change the `created_at` key to the member's earliest assignment. Index `(resource_id, created_at, product_user_id)`. | Platform bearer, or Product API key with `resource:read` |

Writes take a Product API key only, as the resource writes do: the Product's backend assigns roles. A platform session answers `401` on them.

### Rules

All run in the service, inside the write's transaction:

| Rule | Refusal |
| --- | --- |
| `role_ids` holds 1 to 10 IDs. | `400 VALIDATION_ERROR` |
| `role_ids` holds exactly one ID (`roleassignment.MaxRolesPerResource`). | `400 SINGLE_ROLE_PER_RESOURCE` with `max_roles` and `role_ids` (the count sent) |
| The product user is a member of the resource's Organization (D4). | `404 ORGANIZATION_MEMBERSHIP_NOT_FOUND` |
| Each ID names a role of the Organization: a template's ID names the Organization's copy, which is made first when missing, as for memberships (D8). Another Organization's role names none. | `400 ROLE_NOT_FOUND_IN_REQUEST` |
| The role is scoped to the resource's type (D4). An Organization-scoped role is held through the membership, never on a resource. | `400 ROLE_SCOPE_MISMATCH` with `role_id`, `role_resource_type` and `resource_type` |

An unknown role is a 400, not a 404: the body names it, and the house status rule answers a body-named entity that doesn't resolve with a 400, as membership writes already do.

### Locks

Every write takes, in this order: the Organization row `FOR KEY SHARE`, the Organization's resource lock `resources:<organization_id>` in **share** mode, a transaction advisory lock on `resource-member:<resource row>:<product user>`, the membership `FOR KEY SHARE`, then each role as `organizationrole` `Resolve` holds it (its template `FOR SHARE`, or a custom role `FOR KEY SHARE`).

- Assignments run side by side, while a registration, a move or a delete, which hold the resource lock exclusively, runs alone. A cascade therefore lists every assignment it removes, and an assignment never lands on a resource being deleted (`TestConcurrentResourceMemberWrites`).
- Concurrent `PUT`s for one member on one resource run one at a time, and each replaces what the previous one left, so the member never ends with two roles.
- A `PUT` and a `DELETE` hold the membership `FOR KEY SHARE`, so the member stays in the Organization until the write commits. A member removal, a product user delete and a Clerk user delete lock the membership `FOR UPDATE` before they list the member's assignments, so they wait for those writes and list what they left: a `PUT` that committed first has its row listed and its deleted event emitted, and a `DELETE` racing the removal deletes and reports each row once. A write that comes after the lock finds no membership: a `PUT` answers `404 ORGANIZATION_MEMBERSHIP_NOT_FOUND`, a `DELETE` `404 RESOURCE_MEMBER_NOT_FOUND`.
- An Organization delete locks the Organization row `FOR UPDATE`, which waits for every write holding it `FOR KEY SHARE`, then takes the resource lock exclusively, so it lists every assignment in the Organization once.
- A template or custom role delete locks the role first, so an assignment racing it either commits before the in-use count, which then refuses the delete, or finds the role gone (`400 ROLE_NOT_FOUND_IN_REQUEST`, `TestRoleTemplateDeleteDuringAnAssignment`).

### Removal

- **The member leaves the Organization.** `resource_role_assignments (organization_id, product_user_id)` references `organization_memberships` `ON DELETE CASCADE`, so the membership's delete removes every assignment of the member in the Organization in the same transaction. Before it, the removal takes the resource lock in share mode, locks the membership `FOR UPDATE`, and emits one `resource.role_assignment.deleted` per assignment, which marks each resource.
- **The resource is deleted.** Without `cascade`, an assignment on it answers `409 RESOURCE_IN_USE`; with it, the events go first, under the exclusive resource lock (above).
- **The Organization or the product user is deleted.** An Organization delete (`organizationService.Delete`), a product user delete through the API (`productUserService.Delete`) and a Clerk user delete (`clerk` `executeDeleteUser`) call `ReleaseOrganization` or `ReleaseUser` first: they lock as above and emit one `resource.role_assignment.deleted` per assignment, which marks each resource, and the foreign keys then remove the rows. A Product delete removes them without events, with the Product's store.
- **The role.** A role held on a resource is in use like one a membership holds; see [Organization roles](organization-roles.md#custom-roles-and-customized-copies).

### Events

`resource.role_assignment.created` (the member held no role on the resource), `.updated` (the role changed) and `.deleted` (one per role removed by a `DELETE`, a member removal or a resource cascade) are delivered to the Product's endpoint and listed in its catalog (G3). They carry `organization_id`, `resource_type`, `resource_id` (the Product's identifier), `product_user_id` and `role_id`: the role held now, or the one removed. Events carry flat strings, so there is no `role_ids` list; with several roles per resource, each role added or removed would get its own event.

A template deleted while custom roles are on turns its held copies into custom roles under their own row ID (D10). The assignments keep their row, so access doesn't change, but their public `role_id` changes from the template's ID to the copy's without an assignment event, as members' `role_id` does: the template delete emits `organization.role.created` for each detached copy, and a consumer re-reads the resource's members after it. Inside Anchor the event also carries `resource_row_id`, which the marker reads; the emitter drops it from what it delivers.

### Relationship store

An assignment is part of its resource's object state ([Authorization engine](authorization-engine.md#resource-tuples)): `rt_<slug>:<row> # assignment @ role_assignment:<row>.<role row>`, and on that assignment object `# role @ role:<role row>` and one `# assignee @ user:<product user>` per member holding the role there. Each assignment event marks its resource, and the resource sync reads the object and every assignment object it links or should link, so a replaced role, a removal and a member who left drop their tuples. The role's grants stay with the Organization's roles.

## Workspaces

The built-in workspaces are resources of the Product's `workspace` type (ADR-0022 A6, phases 1 and 2). The workspace routes are deprecated but keep working; each workspace keeps its row in `workspaces` and gets a reference with the **same ID**, the workspace's name and the Organization as parent. Code: `internal/resource/service/workspaces.go` (the writes) and `workspace_backfill.go` (the migration).

### The workspace type

A Product needs a `workspace` type that may sit under `organization`. `DeclareType` creates `workspace` (name *Workspace*, under `organization`, not nesting) through the resource type service, which emits the usual `product.resource_type.created`. A Product that already declared `workspace` with `organization` among its parent types keeps that type, whatever else it allows. A Product whose `workspace` can't sit under the Organization, or that can't declare another type (`RESOURCE_TYPE_LIMIT_REACHED`), keeps its catalog untouched: the backfill and every workspace write log an error naming the Product, and its workspaces stay unregistered. Their decisions then fall back to the Organization ([access evaluation](access-evaluation.md#workspaces)).

### The migration

On every start, under the `resources:workspace-backfill` advisory lock, `RegisterWorkspaceBackfill` runs `WorkspaceBackfill.Run` over every Product that has a workspace. KSUIDs can't be minted in SQL, so it is code, like the [resource type backfill](resource-types.md#backfill). For each Product it declares the type, then, for each Organization with workspaces, in one transaction:

1. it registers every workspace that has no reference yet. The parent is the Organization by construction: that is the only parent a built-in workspace has, and the one default the migration takes;
2. it moves each `workspace_memberships` row into a role assignment on the workspace's reference and deletes the row, when the row's role resolves in the Organization to a role scoped to the `workspace` type (D4: a role's scope must equal the resource's type). A template's ID names the Organization's copy, as for any assignment. A row whose role is scoped elsewhere, whose user is not a member, or whose member already holds a role on that workspace stays, and is logged at warn level with its workspace, product user, role and the refusing code. Workspace roles have been Organization-scoped since the G1 backfill, so in practice every existing row stays: the migration never invents a role. No route writes `workspace_memberships`, so the table normally holds no row.

It only fills what is missing. A reference the resource routes renamed, moved or gave roles to keeps them, and a moved membership is never assigned again, so a rerun changes nothing and never brings back an assignment a Product removed. Each write emits its usual event (`resource.created`, internal; `resource.role_assignment.created`, delivered) and marks its object. No request replays those marks, so the [sweeper](authorization-engine.md#freshness) brings them to OpenFGA within seconds.

### Writes through the workspace routes

While the built-in Workspace lives, the workspace routes write the reference in the workspace's own transaction:

| Route | Reference |
| --- | --- |
| Create (and an organization created with `workspace`) | Registered with the workspace's ID and name under the Organization. The Product's first workspace declares the type first, outside the transaction. |
| Update | Re-registered with the workspace's name under the Organization: a rename renames it, and a reference the Product moved, renamed or deleted is put back. |
| Delete | Deleted with `cascade`: its registered descendants and every role assignment on them go too, with their events. |

A write the resource rules refuse (no type, or a type that can't sit under the Organization) leaves the workspace unregistered and is logged; the workspace route answers as it did before. The resource routes never create, rename or delete a row in `workspaces`, and a workspace registered only through them has no row there.

**Who wins.** The last write wins: a workspace route rewrites the reference from the workspace row, and the resource routes may change it afterwards. The backfill writes only what is missing, so it never overwrites either side.

**Locks.** A workspace write and the backfill take the locks of a resource write, in the same order: the Organization row (`FOR UPDATE` on create, `FOR KEY SHARE` otherwise), the catalog in share mode, then `resources:<organization_id>`. The backfill reads the workspaces only after the last one. A workspace write that holds the lock first commits before the backfill reads, so the backfill sees its row and its reference. One that waits commits its reference after the backfill, from its own row. A deleted workspace is therefore never registered again, and a new one is registered once (`TestWorkspaceBackfillRacesWorkspaceRoutes`). The type is declared outside the create's transaction because a refused type insert would abort it, and because the catalog lock would otherwise come after the Organization row; an organization created with a workspace declares it before its own transaction for the same reason.

**Rolling deploy.** The previous release writes workspaces without references. The next start's backfill registers the ones it created. A rename it made leaves the old name on the reference until the next write through a workspace route, which only affects display. A workspace it deleted leaves its reference behind; deleting that reference through the resource routes removes it.

## Tests

`TestWorkspaceBackfill`, `TestWorkspaceRoutesWriteTheirResource`, `TestDeprecatedWorkspaceRoutes` and `TestWorkspaceBackfillRacesWorkspaceRoutes` in `apps/anchor/cmd/it/ct/workspace_resource_test.go` cover the migration (type, references, a reused and an incompatible type, moved and kept memberships, reruns), the writes of the workspace routes, the deprecation headers, and the race between the backfill and those routes. `TestWorkspaceBackfillMirror` in `apps/anchor/cmd/it/service` checks the relationship store after the backfill.

`TestPutResourceMember`, `TestPutResourceMemberRefusals`, `TestGetResourceMember`, `TestDeleteResourceMember`, `TestSearchResourceMembers`, `TestResourceMemberEvents`, `TestRemoveMemberRemovesResourceRoles`, `TestDeletesReleaseResourceRoles`, `TestDeleteResourceWithRoleAssignments`, `TestRoleHeldOnAResource`, `TestResourceMemberScopes`, `TestConcurrentResourceMemberWrites` and the serial `TestRoleTemplateDeleteDuringAnAssignment` in `apps/anchor/cmd/it/ct` cover the assignment rules, codes, limits, events and races; `searchResourceMembers` has a case in `TestCursorSearches`, and the end-of-suite mirror check also checks every assignment's tuples. `TestResourceAssignmentMirror` in `apps/anchor/cmd/it/service` covers the tuples after an assignment, a shared role, a replace, a removal, a member removal, a cascade, a stray assignee, a copy made by the assignment itself and the reconcile, and that an assignment's replay touches its resource only.

`TestPutResource`, `TestPutResourceRefusals`, `TestGetResource`, `TestDeleteResource`, `TestDeleteOwnersOfResources`, `TestResourceScopes`, `TestSearchResources`, `TestSearchResourcesLimits`, `TestResourceEvents`, `TestResourceTypesInUseByResources` and `TestConcurrentResourceWrites` in `apps/anchor/cmd/it/ct` cover every rule, limit and code above; `searchResources` has a case in `TestCursorSearches`. `TestResourceMirror` and `TestOrganizationSyncWithManyResources` in `apps/anchor/cmd/it/service` cover the marks and parent tuples of a registration, a move, a delete and a cascade, the Organization sync that leaves resources alone, the sweeper, the reconcile after an Organization delete, and the cost of a replay.
