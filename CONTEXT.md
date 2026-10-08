# Anchor — domain context

Anchor is Organization-as-a-Service: the foundation work common to every SaaS — hierarchy, identity, RBAC, tenancy — offered to the products built on top of it.

This file is the project's glossary. When output names a domain concept — an issue title, a test name, a type, an endpoint — it uses the term as defined here, and avoids the synonyms each entry rules out.

## Core hierarchy

| term | means |
| --- | --- |
| **Platform Tenant** | The top-level instance. Currently one per deployment. |
| **Product** | An application built on Anchor. Owns its own permission catalog, roles, email templates, and license schema. |
| **Organization** | A Product's customer. The unit a license attaches to. |
| **Workspace** | A sub-unit within an Organization — a team or project. |
| **Platform User** | An administrator of the platform. Authenticates with a bearer token. |
| **Product User** | An end user of a Product. Directory-only; authentication comes from an external IdP. |

## Licensing

The licensing subsystem lets a Product declare what its customers are allowed, record what they are actually using, and keep the history of both. Anchor stores and derives. It never blocks.

> **Anchor validates but never gates.**

That sentence is the boundary. The two verbs are deliberately distinct, because "enforce" was doing both jobs and the ambiguity caused real confusion during design.

| term | who does it | means |
| --- | --- | --- |
| **gate** | the consumer product, only | Block an action because a limit is reached. Anchor never does this. |
| **validate** | Anchor, always | Reject a malformed write. Ordinary input validation. |

### Vocabulary

| term | means | not |
| --- | --- | --- |
| **licensing** | The subsystem as a whole. Never a synonym for *license* — that word names one Organization's grant, and only that. | Not "the license system". |
| **license schema** | Per-Product declaration of every field a license may carry: name, type, and validation rules. | Not "plan schema". |
| **license field** | One declared field within the schema. Every declared field is mandatory: a license template must set all of them ([ADR-0009](docs/adr/0009-every-license-field-is-mandatory.md)). | Not "entitlement" — see below. Not "feature flag". |
| **limit** | A license field of numeric type. Limits are the only fields that carry usage and a status. | Not "quota". |
| **license template** | A named, validated set of values for every field its schema declares, instantiated into organization licenses. | Not "plan" — see below. |
| **archive** | Withdraw a template. It stops being offered, its row is kept so the licenses naming it keep resolving, and its name is freed ([ADR-0010](docs/adr/0010-license-templates-are-archived.md)). | Not "delete" — a template row is never removed. |
| **license** | One Organization's own copy of a template's values, kept in step with that template except on its adjusted fields ([ADR-0018](docs/adr/0018-license-follows-its-template.md)). Every Organization has exactly one. | Not "subscription". |
| **instantiate** | Copy a template's values onto an Organization, creating its license. It happens on the license route, or in the same transaction as the Organization itself ([ADR-0016](docs/adr/0016-an-organization-can-be-created-licensed.md)). | Not "assign" — nothing is pointed at, even though the copy follows the template afterwards. |
| **adjust** | Edit one Organization's license without touching its template. Every field an adjustment moves becomes an *adjusted field*. | Not "override" — there is no override layer ([ADR-0004](docs/adr/0004-license-schema-template-and-copy.md)). |
| **adjusted field** | A license field recorded on the license row as bespoke to that Organization. A template sync leaves it alone; a `DISCARD` migrate clears the record ([ADR-0018](docs/adr/0018-license-follows-its-template.md)). | Not "pinned field" — the record is the field name, not a frozen value. |
| **template sync** | The automatic propagation of a template value update onto every license instantiated from it, except on adjusted fields. Durable, asynchronous, recorded as `TEMPLATE_SYNCED` ([ADR-0018](docs/adr/0018-license-follows-its-template.md)). | Not "re-sync" — that names the operator's `DISCARD` migrate onto the same tier, which also clears adjusted fields. Not "migration" — provenance does not move. |
| **migrate** | Move a set of Organizations onto a license template: take a fresh copy of its values and restamp the provenance. A tier change, recorded as one entry per Organization ([ADR-0014](docs/adr/0014-organization-licenses-are-migrated-in-bulk.md)). | Not "re-sync" — that names recomputing a license from the template it already holds, and it is not what this is for. Not "upgrade" — a migration moves in either direction, and price is not Anchor's word. |
| **deviation** | A value on a license that differs from its template because someone adjusted it for that customer. The state *adjust* produces. | Not "override", for the same reason. |
| **diff** | How an Organization's license differs from its template today, license field by license field. Usually a deviation, since a template edit is otherwise propagated; a template sync still in flight, or one refused by validation, also shows here. | Not "drift" — that word names Terraform's own comparison. |
| **usage report** | What a consumer POSTs: an absolute snapshot of current usage. | Not "usage event" — an event implies a delta, and Anchor does not accept deltas. |
| **observation** | One stored raw usage report row. | |
| **usage shape** | Whether a limit's usage is a *gauge* or a *windowed counter*. Declared once on the license field and checked against every report made against it ([ADR-0013](docs/adr/0013-usage-shape-is-declared-not-inferred.md)). | Not chosen per report — a report whose window presence disagrees with its field's declared shape is refused. |
| **gauge** | A limit whose usage shape is `GAUGE`: a usage report with no window, a number that rises and falls, such as "37 flows exist right now". | |
| **windowed counter** | A limit whose usage shape is `WINDOWED_COUNTER`: a usage report carrying a half-open window `[from, to)`, a number that accumulates within a period and resets when a new window starts. `to` omitted means now, and a window cannot span more than a year. | Not "counter" on its own — the window is what makes the reset unambiguous. |
| **bucket** | A time-aggregated set of observations, produced by TimescaleDB's `time_bucket`. | |
| **status** | Derived per limit: `within_limit`, `at_limit`, `exceeded`, or `stale`. Computed on read, never stored. A limit with no observation on record reads `stale`. | |

### Words this project does not use

**"Entitlement"** is struck. It is a synonym for *license field*, and it collides conceptually with **permission**, which Anchor already has and means something else entirely (an RBAC grant on an action). Keeping both would invite "is SSO a permission or an entitlement?" in every design conversation. There is one word: license field.

**"Quota"** is struck. It is a synonym for *limit*.

**"Plan"** is a billing word. Billing lives outside Anchor ([ADR-0002](docs/adr/0002-anchor-owns-entitlement-state-not-billing.md)), so a plan is something a billing system knows about. Inside Anchor the equivalent concept is a *license template*. If a design document says "plan", it is either talking about the billing system or using the wrong word.

## Identity and credentials

| term | means |
| --- | --- |
| **Product API key** | An Anchor *management* credential held by a Product's backend. Fixed prefix `anchor_prd_apikey_`. |
| **Organization API key** | A credential scoped to one Organization, issued by a Product to its customer. Configurable per-Product prefix, `*_org_apikey_`. |
| **permission** | An RBAC grant naming an action, in `resource:action` form. Belongs to a Product's catalog. Unrelated to licensing. |
| **role** | A named bundle of permissions, assignable to a member at Organization or Workspace level. |

## Membership

| term | means | not |
| --- | --- | --- |
| **member** | A Product User who belongs to an Organization. A member holds exactly one role in that Organization. | Not "user" — a Product User exists in the directory whether or not it belongs to any Organization. |
| **invitation** | An offer to become a member of one Organization, with a named role. It is addressed to an email address, not to a Product User, so the person can be invited before they have an account. The email never changes: a wrong address means deleting the invitation and creating a new one. Anchor stores it and its lifecycle. The Product decides who may invite and when, and tells the person itself: Anchor never sends the invitation email and issues no token ([ADR-0019](docs/adr/0019-organization-invitations-live-in-anchor.md)). | Not "invite" as a noun. Not a Platform invitation — that one makes a Platform User and is unrelated. |
| **accept** | Turn a pending invitation into a membership for an existing Product User, by the invitation id. The Product performs it after it finds the invitation by a verified email address of the signed-in person. The person never talks to Anchor. | Not "join". |
| **invitation status** | `pending`, `accepted`, or `expired`. An invitation reads `expired` once its expiry passes while still pending. Only a pending invitation can be accepted. | Not "revoked" — there is no revoke. Withdrawing an invitation means deleting it. |

At most one pending invitation exists per email address per Organization, and an existing member cannot be invited. Every other rule on an invitation can be changed by the Product through the API, so a Product that runs its own flow is never blocked by Anchor's.

## Product events

A Product stays current by receiving **events** from Anchor. The subscriber is the Product backend only. An Organization never registers an endpoint here.

Inbound Clerk callbacks stay **integration webhooks**. They are a different path.

The catalog is the Product SDK surface: organizations, members, organization invitations, workspaces, organization API keys, product users, licenses. Admin writes (products, platform users, platform invitations, permission and role catalog) do not emit.

| term | means | not |
| --- | --- | --- |
| **event** | A record that something happened to a Product-scoped resource. It has a stable id that does not change across retries. | webhook, notification, message, callback |
| **event type** | Hierarchical name in `resource.action` form (`organization.created`). It names the schema of `data`. | **permission** — those use `resource:action` |
| **endpoint** | An HTTPS URL a Product registers to receive deliveries. Tracer: one URL in Product config. Later: many endpoints through the Product API, each with an event-type filter. | integration webhook |
| **delivery** | One HTTP POST of an event to an endpoint. | |
| **thin payload** | `data` carries identifiers of the subject. The Product fetches current state from the API. | snapshot, full payload |

Delivery follows [Standard Webhooks](https://github.com/standard-webhooks/standard-webhooks/blob/main/spec/standard-webhooks.md) (Svix): headers `webhook-id`, `webhook-timestamp`, `webhook-signature`; body `{type, timestamp, data}` with a thin `data`. Not CloudEvents. See [ADR-0017](docs/adr/0017-product-events-use-standard-webhooks.md).

Membership events are `created` (AddMember), `updated` (role change), and `deleted` (RemoveMember). Invitation events are `created`, `updated`, `deleted` (delete, or a role delete that removes an accepted or expired invitation) and `accepted`. Accept also emits the membership `created` event.

## Workflows

A Product automates its own resources with **workflows**. See [ADR-0020](docs/adr/0020-product-workflows-react-to-product-events.md).

| term | means | not |
| --- | --- | --- |
| **workflow** | A Product's own automation: a trigger, conditions, and ordered steps that run as the Product after each matching event. | Not "flow" — that is Echopoint's word for an API test graph. Not "rule". |
| **trigger** | The event type that starts a run of a workflow. | Not "hook". |
| **step** | One action of a workflow, with its parameters and its own optional conditions. Its output is readable by later steps as `steps.<id>.*`. | |
| **action** | What a step does to a Product resource, from a fixed catalog (`workspace.create`, `member.add`…). | Not "task". |
| **run** | One execution of a workflow against one event, recorded with every step's result. At most one run per workflow and event. | Not "job" — that is the queue entry. |
| **dry run** | A run of an unsaved workflow against sample event data that reads for real and writes nothing. It is not stored. | Not "test run". |
| **custom event** | An event a workflow step emits (`custom.<name>`) to start other workflows. It never leaves Anchor. | Not a catalog event: it is never delivered to the endpoint. |
| **field type** | What an event field or a step output holds (`organization`, `email`, `number`…), so a parameter is offered the fields that fit it. A custom event's sender declares the types of its data; every sender gives a field the same type. | Not a validation rule: values are still strings at run time. |
| **custom action** | A step that calls the Product's own backend, signed like an event delivery, and passes the answer to later steps. | Not "webhook" — the call is a step, and its answer matters. |
| **causation** | The chain of workflows that led to an event. A workflow already in it does not run again, and a chain stops after 5 runs. | Not "trace". |
| **loop** | A set of enabled workflows through which one can start itself again. Anchor refuses to save one, and refuses to run one. | |

## Decisions

Hard-to-reverse decisions live in [`docs/adr/`](docs/adr/). Read the ones touching the area before working in it.
