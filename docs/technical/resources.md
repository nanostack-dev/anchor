# Resources

A Product registers **resources**, references to its own projects, folders or documents, so a role can later be held on one of them and inherited by everything under it ([ADR-0022](../adr/0022-resource-types-scope-roles-and-permissions.md) B1–B4, G3). Anchor stores a reference only: its type, the Product's identifier, a display name and its one parent. Access decisions do not read resources yet.

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
| `DELETE /{resource_type}/{resource_id}` | Deletes it. A resource with registered children answers `409 RESOURCE_IN_USE` with `children` (direct children) and `role_assignments` (always 0 until role assignments on resources exist), unless `?cascade=true`, which deletes its whole subtree in one transaction. | Product API key with `resource:delete` |
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

- A cascade deletes the resource row; the parent foreign key removes its descendants in the same statement. One `resource.deleted` is emitted per deleted resource, deepest first.
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

`resource.created`, `resource.updated` and `resource.deleted` carry `organization_id`, `resource_type`, `resource_id` (the Product's identifier) and `resource_row_id` (Anchor's row, which names the object in the relationship store; only an undelivered event may carry it). An unchanged `PUT` emits nothing. They are emitted in the write's transaction and stay inside Anchor (G3): their catalog definitions are marked `Undelivered`, so the emitter queues nothing for them, the product's event catalog does not list them, and an endpoint can't subscribe to them (`400 UNKNOWN_EVENT_TYPE`). The transaction observers still see them, which is how the relationship store follows the registry.

## Relationship store

Each resource is mirrored as one tuple, `rt_<slug>:<row id> # parent @ rt_<parent slug>:<parent row id>`, or `@ organization:<organization_id>` at the top ([Authorization engine](authorization-engine.md#tuples)). The `authz` marker turns each `resource.*` event into a mark on that resource's object in `authz_resource_marks`, and the request's replay after commit runs a resource sync of exactly those objects: their rows give the desired tuples, a read of each object gives the stored ones, and the diff is applied. A registration, a rename that moves, a move and a delete therefore touch their own object only; a cascade touches each deleted resource once. A move leaves its descendants' tuples alone, since each still names its own parent. Organization syncs never read or write resource tuples.

## Tests

`TestPutResource`, `TestPutResourceRefusals`, `TestGetResource`, `TestDeleteResource`, `TestDeleteOwnersOfResources`, `TestResourceScopes`, `TestSearchResources`, `TestSearchResourcesLimits`, `TestResourceEvents`, `TestResourceTypesInUseByResources` and `TestConcurrentResourceWrites` in `apps/anchor/cmd/it/ct` cover every rule, limit and code above; `searchResources` has a case in `TestCursorSearches`. `TestResourceMirror` and `TestOrganizationSyncWithManyResources` in `apps/anchor/cmd/it/service` cover the marks and parent tuples of a registration, a move, a delete and a cascade, the Organization sync that leaves resources alone, the sweeper, the reconcile after an Organization delete, and the cost of a replay.
