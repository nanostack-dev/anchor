-- Product user statuses take the contract's spelling (active, invited,
-- suspended). Until this release the API default and the Clerk sync stored
-- ACTIVE. The read ignores letter case during the deploy, because replicas of
-- the previous release keep writing ACTIVE until they stop. A later migration
-- runs this backfill again, and the read then accepts the contract spelling
-- only.
-- No CHECK constraint: docs/development/agent-rules.md keeps business rules in
-- the service layer.
SET LOCAL lock_timeout = '5s';

-- lint: allow-backfill product_users is small (one product in prod, under 500 user writes in 30 days); the WHERE skips rows already in the contract spelling
UPDATE product_users
SET status = lower(status)
WHERE status IN ('ACTIVE', 'INVITED', 'SUSPENDED');
