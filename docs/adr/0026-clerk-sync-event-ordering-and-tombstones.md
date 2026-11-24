# ADR-0026: Clerk Sync Event Ordering, Tombstones, and 404-Confirmed Reconcile Deletion

- **Status:** Accepted
- **Date:** 2026-11-24
- **Context:** [anchor#400](https://github.com/nanostack-dev/anchor/issues/400), [Clerk reconcile scheduler](../technical/clerk-reconcile-scheduler.md)

## Context

Clerk delivers `user.created`, `user.updated`, and `user.deleted` webhooks asynchronously over Svix and Anchor also runs periodic Clerk reconciliation. Previously:

1. Neither webhooks nor reconciliation tracked provider event timestamps (`updated_at` / `timestamp`), so a delayed `user.updated` or `user.created` arriving after `user.deleted` resurrected the deleted Product User, and an older update overwrote newer fields.
2. Hard-deleting a Product User left no trace, so a `user.deleted` delivered before `user.created` became a no-op and the late create inserted the user.
3. `UpsertByExternalID` merged only on `(product_id, external_id)`. A clash on `UNIQUE (product_id, email)` raised an unhandled unique violation (`23505`), aborting the webhook transaction or an entire 200-command reconcile batch.
4. Periodic reconciliation never removed users deleted in Clerk whose webhook was missed, and `fetchAllUsers` silently stopped on an early empty page.

## Decision

1. **Provider event timestamps (`product_users.last_synced_event_at`) and tombstones (`product_user_tombstones`)** (migration `000078_clerk_sync_ordering`):
   - `user.created` and `user.updated` use Clerk's `data.updated_at` (ms), falling back to envelope `timestamp` (ms). `user.deleted` uses envelope `timestamp` (ms), falling back to execution `time.Now()` (a later delete timestamp is always the safe direction because Clerk user IDs are never reused). Reconcile upserts use Clerk SDK `u.UpdatedAt` (ms); reconcile deletes use the per-ID 404 confirmation time. `Svix-Timestamp` is never used because retries carry fresh delivery timestamps.
   - `productUsersUpdatableColumns()` excludes `last_synced_event_at` so Product API `Create`/`Update` never resets it to `NULL`.
   - Both upsert and delete handlers take a transaction-scoped advisory lock (`pg_advisory_xact_lock(hashtextextended('clerk-user:'||product_id||':'||external_id, 0))`) so concurrent deletes and upserts serialize even when no `product_users` row exists yet.
   - Deletes always upsert `product_user_tombstones (product_id, external_id, deleted_at)` with `GREATEST(existing, excluded)` even when no row is present. Upserts skip when a tombstone exists with `event_at IS NULL OR event_at <= deleted_at`, or when `event_at < last_synced_event_at`.
   - Unchanged upserts still advance `last_synced_event_at` monotonically when `last_synced_event_at IS NULL OR last_synced_event_at < event_at` without emitting events or rewriting unchanged rows on steady-state reconciles.
2. **Explicit email conflict and verified adoption handling:**
   - Before inserting or changing an email, the upsert handler checks `(product_id, email) FOR UPDATE`.
   - A row with `external_id IS NULL` is adopted (linking `external_id`, preserving ID and `suspended` status) **only when the Clerk primary email has `verification.status == "verified"`**; an unverified Clerk email is skipped with a warning audit entry to prevent account takeover of pre-existing/invited memberships.
   - A row held by a different `external_id` raises a typed `ProductUserEmailConflictError` (`ErrProductUserEmailConflict`) on the webhook path (retryable, because the previous user's delete may still be in flight) and is skipped with a warning audit row on the reconcile path so steady-state conflicts do not abort a 200-command reconcile batch.
   - New inserts use `ON CONFLICT DO NOTHING` (no conflict target) and skip on reconcile races rather than failing the batch; a rare concurrent insert racing an email-change `UPDATE` can still abort that batch and recover on the next reconcile tick.
3. **404-confirmed reconcile deletion with safety caps:**
   - Reconcile records `snapshotStart` before page 1, pages with `order_by=+created_at`, and fails the run on any page error or early empty page (`len(users) == 0 && offset < total_count`) with zero deletes.
   - Only rows with `external_id IS NOT NULL`, `last_synced_event_at IS NOT NULL`, `last_synced_event_at < snapshotStart`, `created_at < snapshotStart`, and not in `SeenIDs` are deletion candidates.
   - The entire reconcile run (both deletes and upserts) aborts with a warning audit row if `SeenIDs` is empty while stamped rows exist or if none of the previously synced users appear in `SeenIDs` (guarding against an API key swapped to a different Clerk app even on small products below the 25-user floor, and preventing wrong-app upserts from creating false overlap on a second run).
   - Deletions alone abort (with a warning audit row, while upserts still run) if candidates exceed `max(25, 10% of stamped rows)`.
   - Remaining candidates (capped at 50 per run for Clerk rate limits) are each verified via `GET /v1/users/{id}`; **only an explicit HTTP 404** confirms deletion. Confirmed deletes run before upserts so a user who deleted and re-signed up with the same email heals in a single reconcile pass.

## Alternatives Considered

- **Use `Svix-Timestamp` for event ordering:** Rejected because Svix retries carry fresh delivery timestamps, making a retried stale event appear newer than a later delete.
- **Blindly delete every user missing from paginated `List`:** Rejected because offset pagination is not a snapshot (a delete mid-walk shifts offsets and skips live users) and a misconfigured API key returning an empty list would wipe every membership.
- **Tombstone API-initiated deletions:** Rejected because Clerk is the source of truth for Clerk-synced users; deleting a user via Anchor's API while they remain active in Clerk intentionally allows the next Clerk event to recreate them.

## Consequences

- Out-of-order Clerk webhooks and races between delete and upsert/create no longer resurrect deleted users or regress newer profile updates.
- Email conflicts no longer wedge reconciliation batches; verified Clerk accounts cleanly adopt `external_id IS NULL` rows without risking unverified email takeover.
- **Pre-migration rows:** Rows created before `000078` keep `last_synced_event_at = NULL` until a reconcile or webhook upserts them. Rows whose Clerk account was already deleted before `000078` never receive a stamp and are therefore never auto-deleted by reconcile; cleaning them up requires an explicit operator task.
- **Pre-existing key/webhook mismatch on first stamping run:** While `stampedCount == 0` (the very first reconcile on a fresh instance or immediately after migration `000078`), the zero-overlap guard cannot fire yet. Ensure an instance's Clerk API key and webhook signing secret point at the same Clerk application before its first reconcile after rollout.
- **Intentional Clerk application migration:** Because the zero-overlap guard aborts both deletes and upserts, intentionally moving an existing product to a brand-new Clerk application keeps aborting with `no_overlap_with_existing_users` until at least one new-app user is synced via webhook or the old `last_synced_event_at` stamps are cleared by an operator.
- **Retention:** `product_user_tombstones` rows are kept indefinitely in v1; Clerk user IDs are never reused, so tombstones never block a new user.
