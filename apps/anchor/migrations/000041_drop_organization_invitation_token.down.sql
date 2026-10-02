DROP INDEX IF EXISTS idx_organization_invitations_product_email;

ALTER TABLE organization_invitations ADD COLUMN token_hash VARCHAR(255);
UPDATE organization_invitations SET token_hash = encode(sha256(gen_random_uuid()::text::bytea), 'hex');
ALTER TABLE organization_invitations ALTER COLUMN token_hash SET NOT NULL;
ALTER TABLE organization_invitations ADD CONSTRAINT organization_invitations_token_hash_key UNIQUE (token_hash);
