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
| **license** | One Organization's own copy of a template's values. Every Organization has exactly one. | Not "subscription". |
| **instantiate** | Copy a template's values onto an Organization, creating its license. | Not "assign" — nothing is pointed at. |
| **adjust** | Edit one Organization's license without touching its template. The act. | Not "override" — there is no override layer ([ADR-0004](docs/adr/0004-license-schema-template-and-copy.md)). |
| **deviation** | A value on a license that differs from its template because someone adjusted it for that customer. The state *adjust* produces. | Not "override", for the same reason. |
| **diff** | How an Organization's license differs from its template today, license field by license field. A difference is either a deviation or the template moving after the copy was taken — the diff alone does not say which. | Not "drift" — that word names Terraform's own comparison. |
| **usage report** | What a consumer POSTs: an absolute snapshot of current usage. | Not "usage event" — an event implies a delta, and Anchor does not accept deltas. |
| **observation** | One stored raw usage report row. | |
| **bucket** | A time-aggregated set of observations, produced by TimescaleDB's `time_bucket`. | |
| **status** | Derived per limit: `within_limit`, `at_limit`, `exceeded`, or `stale`. Computed on read, never stored. | |

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

## Decisions

Hard-to-reverse decisions live in [`docs/adr/`](docs/adr/). Read the ones touching the area before working in it.
