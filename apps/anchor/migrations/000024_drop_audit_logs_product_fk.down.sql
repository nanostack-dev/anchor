-- Intentionally a no-op: re-adding the FK would fail on any audit rows that
-- reference deleted products, and the constraint is not wanted (audit rows
-- must survive product deletion).
SELECT 1;
