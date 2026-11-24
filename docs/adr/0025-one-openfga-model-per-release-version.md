# ADR-0025: One OpenFGA Model Per Release Version

- **Status:** Accepted
- **Date:** 2026-11-24
- **Context:** [anchor#401](https://github.com/nanostack-dev/anchor/issues/401), [ADR-0021](0021-access-decisions-come-from-openfga.md), [ADR-0022](0022-resource-types-scope-roles-and-permissions.md)

## Context

`authz_stores` recorded a single `(model_id, model_hash)` pointer per Product. When a release bumps `model.Version` (`apps/anchor/internal/authz/model/compile.go`), old and new replicas compute different hashes for the same catalog. Because `decision.Checker` runs `EnsureModel` every 30 s and `store.Open` reads the single pointer on `authz_stores`, old and new replicas repeatedly overwrote each other's model pointer during a rolling deploy. During those windows, new-release decisions could execute against the old model and deny newly introduced computed relations.

## Decision

1. **Key models by `(product_id, model_version)` in `authz_models`** (migration `000077_authz_models`):
   - Each release reads and writes only its own `model.Version` row (`RecordedModel(ctx, productID, version)` / `SetModel(ctx, productID, version, model)`).
   - Keying by `model_version` rather than `model_hash` lets `store.Open` resolve the release's model in a single SQL `LEFT JOIN` with no catalog read per decision, and lets catalog edits overwrite that release's row instead of growing unbounded history.
   - `model_hash` remains on the row so catalog edits still detect changes within a release version.
2. **Lazy adoption and rollback mirror:**
   - No SQL backfill is performed on `000077`. When a release has no `authz_models` row yet, `ensureModel` reads `authz_stores` via `LegacyModel`; if `authz_stores.model_hash` matches its compiled hash, it adopts `authz_stores.model_id` into `authz_models` without an OpenFGA `WriteModel` call.
   - `SetModel` mirrors `(model_id, model_hash)` back to `authz_stores` inside the same `authz-sync:<product>` advisory-lock transaction so pre-#401 replicas still running during a rolling deploy (or after a schema rollback with `migrate force 76`, see [rollback](../runbooks/rollback.md)) still find a valid pointer.
   - Once the oldest rollback target includes `#401`, a follow-up migration may drop `authz_stores.model_id` and `authz_stores.model_hash`.
3. **Inline model write on first decision for a new version:**
   - When `store.Open` sees an already-rebuilt store (`reconciled_at IS NOT NULL`) with no row for this release's `model.Version`, it falls back to the newest `authz_models` row of any version (or `authz_stores.model_id` when `authz_models` is empty) with `ModelCurrent: false`, preventing a full `Reconcile` on every Product after deploy.
   - `decision.Checker` bypasses the 30 s cache whenever `!ModelCurrent`, runs `EnsureModel` inline under singleflight, and serves the fallback model (retrying after `modelRetryDelay`) only if `EnsureModel` fails (e.g. `ErrLockBusy`). Note that during that brief failure fallback window, a relation defined only in the new model is absent from the fallback model and evaluates as denied rather than 503.

## Alternatives Considered

- **Key `authz_models` on `(product_id, model_hash)`:** Rejected because `store.Open` would have to read the Product's resource-type tree and permissions catalog to compute `Hash` on every decision, and every catalog edit would leave behind an extra row.
- **Newest-version-wins guard on `authz_stores`:** Rejected because rollbacks would be blocked from restoring the previous release's model.
- **Lengthen `modelCheckInterval` during deploys:** Rejected because it only reduces churn without eliminating cross-release model overwrites.

## Consequences

- Rolling deploys across `model.Version` bumps and rollbacks coexist cleanly: each release pins checks and syncs to its own model ID, and alternating decisions write zero extra OpenFGA models.
- **Known constraint (tuples):** Per-version models stop releases from overwriting each other's model, not from deleting tuples another version accepts. `fgasync` filters desired tuples by the syncing replica's `Accepts` and deletes stored tuples it does not claim, so a `model.Version` bump that changes direct tuple types must not be rolled across a live mixed deploy without a separate tuple-compatibility strategy.
