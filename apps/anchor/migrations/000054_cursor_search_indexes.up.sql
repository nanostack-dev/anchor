-- =============================================
-- Migration 000043: Indexes behind cursor-paginated searches
-- =============================================
-- Every search orders by its default sort and ends with the primary key, so the
-- cursor seek is one row comparison, (created_at, id) > ($1, $2). A composite
-- index on the scope column followed by the sort columns answers it with one
-- range scan at any depth, in either direction. Without one, each page sorts
-- the whole scope again.
--
-- Plain CREATE INDEX, not CONCURRENTLY: golang-migrate sends a migration file as
-- one transaction, and no earlier migration builds an index concurrently.
--
-- No CHECK constraints and no business triggers.

CREATE INDEX IF NOT EXISTS idx_organizations_product_created_id
    ON organizations (product_id, created_at, id);

CREATE INDEX IF NOT EXISTS idx_workspaces_organization_created_id
    ON workspaces (organization_id, created_at, id);

CREATE INDEX IF NOT EXISTS idx_product_users_product_created_id
    ON product_users (product_id, created_at, id);

-- Members of one organization: the default order is joined_at, which is
-- organization_memberships.created_at, then the member id.
CREATE INDEX IF NOT EXISTS idx_organization_memberships_organization_created_user
    ON organization_memberships (organization_id, created_at, product_user_id);

CREATE INDEX IF NOT EXISTS idx_organization_invitations_organization_created_id
    ON organization_invitations (organization_id, created_at, id);

-- The product-wide invitation search has no organization to lead with.
CREATE INDEX IF NOT EXISTS idx_organization_invitations_product_created_id
    ON organization_invitations (product_id, created_at, id);

CREATE INDEX IF NOT EXISTS idx_organization_api_keys_organization_created_id
    ON organization_api_keys (organization_id, created_at, id);

CREATE INDEX IF NOT EXISTS idx_product_api_keys_product_created_id
    ON product_api_keys (product_id, created_at, id);

CREATE INDEX IF NOT EXISTS idx_email_templates_product_created_id
    ON email_templates (product_id, created_at, id);

-- idx_email_send_records_rate_limit already leads with (product_id, created_at
-- DESC) for the rate limiter, but it ends before the id the seek compares.
CREATE INDEX IF NOT EXISTS idx_email_send_records_product_created_id
    ON email_send_records (product_id, created_at, id);

CREATE INDEX IF NOT EXISTS idx_integration_audit_logs_instance_created_id
    ON integration_audit_logs (integration_instance_id, created_at, id);
