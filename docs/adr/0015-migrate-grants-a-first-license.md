# ADR-0015: Migrating onto a template grants a first license too

**Status:** Accepted

Supersedes one clause of [ADR-0014](0014-organization-licenses-are-migrated-in-bulk.md): the paragraph that has an organization holding no license `SKIPPED` with reason `NOT_LICENSED`, and calls instantiation "a different verb with a different scope." Everything else ADR-0014 decided — restamping provenance, `CARRY_FORWARD`/`DISCARD`, the 500 cap, one transaction per organization, the search route — stands unchanged.

## Context

An operator selects a cohort of organizations in the admin console and moves them onto a tier. Some of the cohort already hold a license on another tier. Some hold none — a prospect that was created but never licensed, an organization whose license was never instantiated for whatever reason. ADR-0014 skips the second group and reports why. The operator, looking at one list and one destination tier, does not experience that as two operations; they experience it as the route refusing part of an otherwise ordinary request.

ADR-0014's stated reason for the split was that instantiation and migration are different verbs. That line was a conclusion, not an argument — the ADR spends real weight arguing why a *consumer-side loop of adjustments* cannot substitute for a real migrate route, and no comparable weight on why granting a first license through this same route specifically must stay out of scope. Asked directly, the answer an operator gives is the one this ADR takes: an organization arriving at a tier for the first time through this route is not having its history rewritten or its provenance faked — it is being stamped, exactly like every other organization in the same run. The route already computes the right values, validates against the right template, and writes the right kind of history entry for every other member of the cohort; withholding that from an organization only because its previous value happened to be "none" was the part worth reversing.

### Why this is safe to change now

The route shipped in PR #103, already merged to `origin/main`. Two facts keep this a low-risk change rather than a breaking one:

- The route's `security` block lists `platformBearerAuth: []` (no scope check) ahead of `productApiKeyAuth: [organization_license:migrate]`. The admin console — the only caller today — authenticates with a platform bearer token, so no caller is scope-gated on this route in practice.
- No product API key holds `organization_license:migrate`. Grepping every consumer repo (`echopoint`, `echopoint-runner`, `echopoint-cli`, `anchor` itself) turns up no reference to that scope outside `anchor`'s own OpenAPI document and its tests.

There is no live integration to keep compatible. This ADR does not propose a deprecation window for that reason: there is nothing yet depending on the shape being replaced.

## Decision

**The migrate route grants a first license to any selected organization that has none, using the same target template and the same difference policy as every other organization in the run.**

### One outcome, not two verbs

`LicenseMigrationOutcome` drops `MIGRATED` in favor of `CHANGED`, and drops `SKIPPED` entirely — nothing this route does skips an organization anymore. `LicenseMigrationSkipReason` is deleted; it held exactly one value, and that value no longer occurs.

```
Before: MIGRATED | UNCHANGED | SKIPPED | FAILED
After:  CHANGED   | UNCHANGED |           FAILED
```

`CHANGED` was chosen over keeping `MIGRATED` because "migrated" presumes a prior tier, and over introducing new vocabulary (`STAMPED` was considered for this field too) because the enum already has half the pair: `UNCHANGED` exists, so `CHANGED` completes it rather than adding a third word for the same axis. `previous_template_id` continues to say whether an organization had a prior tier; the outcome no longer duplicates that distinction under a different name.

### The history entry says something narrower than the outcome does

The persisted `change_type` on `organization_license_changes` is not the same field as the API outcome, and does not take the same word. `ADJUSTED` already occupies "a change happened" at that layer, for the unrelated case of a field-level edit that never touches `template_id`. Writing `CHANGED` there too would make a tier-stamp and a field-adjustment indistinguishable in the one place — the audit history — whose entire job is to keep those two apart. That is the specific failure ADR-0014 was written to prevent, applied to itself.

`change_type` instead takes `STAMPED`, replacing the old `MIGRATED` value there, for both an organization moving between tiers and an organization licensed for the first time through this route. ADR-0014's own language already used "restamped" for this — `template_id` and `instantiated_at` being overwritten with the target's — for both cases; this makes the recorded value match the word the design already reached for. `INSTANTIATED` is unchanged, and still means exactly what it meant before: the single-organization `POST .../license` route wrote it, and still does. Two different doors to a first license now exist — one organization at a time, or as part of a batch moving to a shared tier — and the history entry says which door was used.

### The scope collapses into the one instantiation already uses

`organization_license:migrate` is removed from the permission catalog. The migrate route's `productApiKeyAuth` requirement becomes `organization_license:update` — the scope `PATCH .../license` (adjust) already requires. This is a deliberate choice to keep the license permission surface at three verbs (`create`, `read`, `update`) rather than four, on the reasoning that granting and moving a license are both, from a permissions standpoint, "changing what this organization's license says" — the same shape `update` already covers for a field-level edit. `create` stays scoped to the single-organization instantiate route, which keeps its own refusal (an organization with a license already cannot be instantiated again) that the batch route does not share.

### An organization with no license is no longer a distinguishable case

Selecting organizations by `from_template_id` still only ever matches organizations that hold a license naming that template, unchanged from ADR-0014 — an unlicensed organization has no `template_id` to match. It is reachable only through explicit `organization_ids`. Once selected, it is licensed onto the target exactly as a licensed organization would be moved: `on_difference` does not apply to it, because there is no prior value to carry forward or discard, and the target's values are taken whole — which is what `CARRY_FORWARD` already does for a field with nothing to carry.

## Consequences

**Good.** The operator's actual question — "put this cohort on this tier" — is now one call regardless of what the cohort currently holds, matching what the admin console's selection UI already presented as one action.

**Good.** The license permission surface stays three verbs instead of drifting to four for an operation that is semantically an update to what an organization's license says, not a new kind of write.

**Good.** History keeps telling apart "this organization's tier changed" from "this organization's fields were hand-adjusted," which is the one distinction ADR-0014 actually needed to protect. `STAMPED` vs `ADJUSTED` protects it exactly as `MIGRATED` vs `ADJUSTED` did before.

**Cost.** `SKIPPED` and `NOT_LICENSED` existed for exactly this case and are deleted rather than left in place unused, because a value with zero remaining producers is worse than no value: a client written against it would wait forever for an outcome that can no longer occur. Any external reader coded against the old enum breaks; per the safety argument above, nothing production-facing is coded against it today.

**Cost.** `organization_license:migrate` is removed rather than kept as an unused alias. The same argument applies: no caller holds it, so keeping it costs a permanently-dead catalog entry for no compatibility it actually buys.

**Cost.** A data migration is required: existing `organization_license_changes` rows with `change_type = 'MIGRATED'` are rewritten to `STAMPED` so the column's history reads consistently with the new vocabulary rather than splitting into "old runs say MIGRATED, new runs say STAMPED" forever. `change_type` carries no `CHECK` constraint (ADR-0014), so this is a data-only migration, not a schema one.
