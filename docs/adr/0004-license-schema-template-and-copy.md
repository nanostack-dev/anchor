# ADR-0004: License schema, template, and per-organization copy

**Status:** Accepted, partially superseded by [ADR-0017](0017-license-follows-its-template.md)

[ADR-0017](0017-license-follows-its-template.md) reverses the copy-not-pointer decision below: a license is still stored as its own copy, but the copy now follows its template — a template value update is propagated onto every license instantiated from it, except on the fields adjusted for that organization. The three-layer shape (schema → template → license), the structured validation rules, and rules-constrain-decisions all still govern.

## Context

Licensing needs three separable things: a declaration of what a license *may* contain, a reusable set of values, and one organization's actual grant. Collapsing them produces either free-form key/value pairs with no validation, or a rigid column-per-feature schema requiring a migration for every new field.

Anchor already has this exact skeleton one floor down: `product_permissions` (catalog) → `product_roles` (bundle) → role assignment (per organization). It also already ships a declared-field-schema stored as JSONB, in `email.VariableSchema`, used to render a structured form in the UI.

## Decision

Three layers, mirroring the existing RBAC skeleton:

```
license schema     per Product. declares every possible field:
                   name, type, required, validation rules
      ↓ validates
license template   a named set of values satisfying the schema
      ↓ instantiates (by copy)
license            every Organization has its own
```

**An organization's license is a copy**, not a pointer. Instantiation materializes the template's values onto the organization. `template_id` and `instantiated_at` are retained as provenance.

**Templates are mutable and unversioned.** No `DRAFT`/`PUBLISHED`/`ARCHIVED` lifecycle, no `template_version`.

**Validation rules are structured data**, stored as JSONB alongside the field declaration, extending the shape of `email.VariableSchema`:

```json
{ "name": "flows", "type": "limit", "required": true,
  "rules": { "min": 0, "max": 100000 } }
```

Not encoded as validator tag strings (`"gte=0,lte=100000"`). Anchor's API is public with a generated Go client and an OpenAPI contract; a tag string would put a Go library's DSL into that contract, forcing every non-Go consumer to reimplement a parser. Structured rules also validate when the schema is *written* rather than failing later when a value is first checked, and they render as a form.

**Rules constrain decisions, not observations.** They apply when a limit is *set* — `flows = 500` must satisfy `{min: 0, max: 100000}` — and never to a usage report. Rejecting an over-limit usage report would make the `exceeded` status unreachable: an organization that genuinely has 150,000 flows would have its report refused, and Anchor would keep serving a stale value that reads `within_limit`.

## Consequences

**Good.** Copy semantics mean editing a template cannot break a live organization, because no live organization reads it. The organization's own row *is* the historical record of what the template looked like when they were stamped.

**Good.** Per-organization deviation is already built. Giving one customer +50 flows is editing their license — no override layer needed. This is why templates do not need versioning: the protection versioning would provide is already provided by the copy.

**Good.** Templates are consulted once, at instantiation. Unlike `email_template_versions`, which resolves at send time and is therefore a live dependency, a license template is a stamp. Versioning it would add a state machine to solve a problem that does not exist here.

**Cost.** No template history. "Show me Pro as it looked in January" is unanswerable. Buy it back later with an append-only template change log — additive, no redesign.

**Cost.** Drift between an organization and its template is a field-by-field diff rather than a version comparison. This is arguably better — it names which fields differ — but it is more work to compute and to render.

**Required regardless.** The organization license needs its own append-only change history, following the `integration_audit_logs` idiom. "This organization went Free → Pro on March 3, and someone raised `flows` to 800 on March 9" is a first-class requirement, not a nice-to-have.

**Note.** Migration 000014 added `CONSTRAINT chk_email_template_versions_status CHECK (...)`, predating the repository's no-CHECK invariant. License migrations must not copy that pattern; status enums stay in the service layer.
