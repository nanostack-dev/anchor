-- Restores the column 000024 created. The per-field values are not recoverable,
-- so it defaults to TRUE rather than to 000024's FALSE: while the column was
-- absent every declared field was mandatory, and defaulting to FALSE would
-- silently make every field optional on the way back.
ALTER TABLE license_schema_fields ADD COLUMN is_required BOOLEAN NOT NULL DEFAULT TRUE;
