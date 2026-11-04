-- Product users created without a status, and every Clerk-synced user, were
-- stored as 'ACTIVE', outside the contract enum (active, invited, suspended).
-- Responses returned 'ACTIVE' and a statuses: ["active"] search missed them.
-- The service now writes the contract spelling; this folds existing rows.
UPDATE product_users SET status = lower(status) WHERE status <> lower(status);
