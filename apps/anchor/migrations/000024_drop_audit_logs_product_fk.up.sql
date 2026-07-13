-- 000023 originally shipped audit_logs.product_id with a FK to products
-- (ON DELETE CASCADE) and was later edited in place after some environments
-- had already applied it. golang-migrate tracks version numbers, not file
-- content, so those environments kept the constraint. This converges them:
-- audit rows must survive product deletion (see docs/audit-logs.md).
ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS audit_logs_product_id_fkey;
