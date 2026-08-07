# ADR-0009: Every license field is mandatory in a template

**Status:** Accepted

Amends [ADR-0004](0004-license-schema-template-and-copy.md), whose example declaration carries a `"required": true` member, and supersedes the per-field optionality described there.

## Context

[ADR-0004](0004-license-schema-template-and-copy.md) gave each license field a `required` flag: a template had to set the required ones and could omit the rest. That is the shape migration `000024` stored and the shape the first slice shipped.

Optionality moves a question rather than answering it. If a template may omit `support_tier`, then everything downstream has to decide what the omission means, and each one decides separately:

- **Instantiation** copies the template onto an Organization's license. Does the absent field land as absent, or as some default?
- **Status derivation** compares usage against a limit. An absent limit is not obviously "unlimited", "zero", or "not applicable".
- **The admin UI** renders a template. An absent field is not obviously "not granted" — it is equally "nobody has filled this in yet".
- **A consuming product** reads its license through the SDK. It writes `if license.Flows > n`, and an absent `Flows` decides for it.

None of those readings is wrong, which is the problem: nothing in the data says which one was meant. The declaration is the only place that could say, and a `required: false` flag says only that the field may be missing, not what missing means.

## Decision

**Every license field a schema declares must be set by every template of that schema.** The `required` flag is removed from the declaration — from the API contract, the domain type, and the `license_schema_fields` table (migration `000026`).

A template that omits a declared field is refused with `LICENSE_FIELD_MISSING`, naming the field. A template carrying a key the schema does not declare is refused with `LICENSE_FIELD_UNKNOWN`, as before.

A field that genuinely has an "off" state expresses it in its own type and rules — a `BOOLEAN` set to `false`, a `LIMIT` set to `0`, an `ENUM` declaring a `none` value. That is a stated grant rather than an inferred one, and it is readable without a convention.

## Consequences

**Good.** Reading a template answers what a customer has, for every field, with no convention to look up and no reader inventing its own default. The same holds for an Organization's license, since it is a copy.

**Good.** The absent-value branch disappears from every consumer that would otherwise have grown one — instantiation, status derivation, the SDK facade, the admin UI. It is one refused write instead of four independent interpretations.

**Cost.** Adding a field to a schema invalidates every existing template until each one sets it. The schema write is **not** refused for this: Anchor validates but never gates, and a schema edit that could be blocked by an unrelated template would make the declaration hostage to its consumers. A template keeps serving instantiation with the values it has; the next edit to that template is refused until the new field is set.

**Cost.** Widening a schema is therefore a two-step operator task — add the field, then fix each template — with a window in between where templates are stale rather than broken. An operator who wants a smooth widening declares the new field with a permissive rule set and updates the templates promptly.

**Cost.** This is a breaking change to a contract already merged (PR #78). It is taken now because nothing consumes the licensing API yet: the SDK facade, the Terraform resources, and the admin UI are all unbuilt. The same change after any of them ships would be considerably more expensive.

**Note.** Epic #62's user stories 3 and 9 ("mark a license field required or optional", "a template rejected if it omits a required field") are superseded by this ADR. Story 9's intent survives in stronger form: a template is rejected if it omits *any* field.
