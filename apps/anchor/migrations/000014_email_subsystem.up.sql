-- =============================================
-- Migration 000014: Email Subsystem
-- =============================================
-- Per-product email templates (with draft/published versioning), send
-- history, and suppression list. The actual transport (SMTP / SES / etc.)
-- is owned by the integration subsystem; v1 enforces a 1-1 product <->
-- email-integration relation via the existing UNIQUE
-- (product_id, provider_type) on integration_instances.

-- 1. Email Templates
-- The template envelope: identity (slug, name) plus pointers to the current
-- DRAFT and PUBLISHED versions. Authoring fields (subject, bodies,
-- variables) live on email_template_versions so PUBLISHED rows are
-- immutable: publishing is an atomic pointer swap, never a row mutation.
CREATE TABLE email_templates (
    id VARCHAR(255) PRIMARY KEY, -- KSUID prefix: etpl_
    platform_tenant_id VARCHAR(255) NOT NULL,
    product_id VARCHAR(255) NOT NULL,
    slug VARCHAR(120) NOT NULL, -- stable identifier used by product code
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    draft_version_id VARCHAR(255),     -- FK added after versions table below
    published_version_id VARCHAR(255), -- FK added after versions table below
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (product_id, slug),
    CONSTRAINT fk_email_templates_product_tenant
        FOREIGN KEY (product_id, platform_tenant_id)
        REFERENCES products(id, platform_tenant_id)
        ON DELETE CASCADE
);

CREATE INDEX idx_email_templates_product_id ON email_templates(product_id);
CREATE INDEX idx_email_templates_platform_tenant_id ON email_templates(platform_tenant_id);

CREATE TRIGGER update_email_templates_updated_at BEFORE UPDATE ON email_templates FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 2. Email Template Versions
-- Snapshot of authoring content at a point in time. Lifecycle:
--   DRAFT: editable in place; visible only to authoring + TestSend.
--   PUBLISHED: immutable; the version returned to API senders by default.
--   ARCHIVED: a previously-published version superseded by a newer publish.
-- Subject and bodies support Go text/template syntax (variables, range
-- loops, conditionals).
CREATE TABLE email_template_versions (
    id VARCHAR(255) PRIMARY KEY, -- KSUID prefix: etplv_
    template_id VARCHAR(255) NOT NULL REFERENCES email_templates(id) ON DELETE CASCADE,
    version_number INTEGER NOT NULL,
    subject TEXT NOT NULL,
    body_html TEXT NOT NULL,
    body_text TEXT NOT NULL DEFAULT '',
    variables_json JSONB NOT NULL DEFAULT '[]', -- declared VariableSchema[]
    status VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    UNIQUE (template_id, version_number),
    CONSTRAINT chk_email_template_versions_status CHECK (
        status IN ('DRAFT', 'PUBLISHED', 'ARCHIVED')
    )
);

CREATE INDEX idx_email_template_versions_template_id ON email_template_versions(template_id);
CREATE INDEX idx_email_template_versions_status ON email_template_versions(template_id, status);

-- At most one DRAFT and one PUBLISHED row per template (ARCHIVED is
-- unbounded). Enforced via partial unique indexes so the version-pointer
-- columns on email_templates are guaranteed to resolve to a single row.
CREATE UNIQUE INDEX idx_email_template_versions_one_draft
    ON email_template_versions(template_id) WHERE status = 'DRAFT';
CREATE UNIQUE INDEX idx_email_template_versions_one_published
    ON email_template_versions(template_id) WHERE status = 'PUBLISHED';

CREATE TRIGGER update_email_template_versions_updated_at BEFORE UPDATE ON email_template_versions FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Wire the version pointers on email_templates now that the target table
-- exists. Both are nullable (a brand-new template may have only a draft;
-- a hard-deleted version would null the pointer).
ALTER TABLE email_templates
    ADD CONSTRAINT fk_email_templates_draft_version
        FOREIGN KEY (draft_version_id) REFERENCES email_template_versions(id) ON DELETE SET NULL,
    ADD CONSTRAINT fk_email_templates_published_version
        FOREIGN KEY (published_version_id) REFERENCES email_template_versions(id) ON DELETE SET NULL;

-- 3. Email Send Records
-- Immutable audit row for every send attempt (including suppressed sends).
-- We persist the rendered subject + bodies so support can answer "what did
-- the user actually receive" even if the template later changed.
CREATE TABLE email_send_records (
    id VARCHAR(255) PRIMARY KEY, -- KSUID prefix: esnd_
    platform_tenant_id VARCHAR(255) NOT NULL,
    product_id VARCHAR(255) NOT NULL,
    integration_instance_id VARCHAR(255), -- nullable: SUPPRESSED sends never reached an integration
    template_id VARCHAR(255) REFERENCES email_templates(id) ON DELETE SET NULL,
    -- The exact version snapshot rendered at send time. Decoupled from the
    -- template's current published_version_id so support can replay history
    -- after a re-publish.
    template_version_id VARCHAR(255) REFERENCES email_template_versions(id) ON DELETE SET NULL,
    dedupe_key VARCHAR(255), -- per-product idempotency
    to_address VARCHAR(320) NOT NULL,
    to_name VARCHAR(255) NOT NULL DEFAULT '',
    from_address VARCHAR(320) NOT NULL,
    from_name VARCHAR(255) NOT NULL DEFAULT '',
    subject TEXT NOT NULL,
    body_html TEXT NOT NULL,
    body_text TEXT NOT NULL DEFAULT '',
    variables_json JSONB NOT NULL DEFAULT '{}',
    message_id VARCHAR(255) NOT NULL DEFAULT '',
    status VARCHAR(50) NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_email_send_records_status CHECK (
        status IN ('QUEUED', 'SENDING', 'SENT', 'FAILED', 'SUPPRESSED')
    ),
    CONSTRAINT fk_email_send_records_product_tenant
        FOREIGN KEY (product_id, platform_tenant_id)
        REFERENCES products(id, platform_tenant_id)
        ON DELETE CASCADE,
    CONSTRAINT fk_email_send_records_integration
        FOREIGN KEY (integration_instance_id)
        REFERENCES integration_instances(id)
        ON DELETE SET NULL
);

CREATE INDEX idx_email_send_records_product_id ON email_send_records(product_id);
CREATE INDEX idx_email_send_records_platform_tenant_id ON email_send_records(platform_tenant_id);
CREATE INDEX idx_email_send_records_template_id ON email_send_records(template_id);
CREATE INDEX idx_email_send_records_template_version_id ON email_send_records(template_version_id);
CREATE INDEX idx_email_send_records_status ON email_send_records(status);
CREATE INDEX idx_email_send_records_created_at ON email_send_records(created_at DESC);
-- Dedupe: nullable unique on (product_id, dedupe_key) when dedupe_key is set.
CREATE UNIQUE INDEX idx_email_send_records_dedupe ON email_send_records(product_id, dedupe_key)
    WHERE dedupe_key IS NOT NULL;
-- Rate-limit query: sends per product since timestamp.
CREATE INDEX idx_email_send_records_rate_limit ON email_send_records(product_id, created_at DESC);

CREATE TRIGGER update_email_send_records_updated_at BEFORE UPDATE ON email_send_records FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 4. Email Suppressions
-- Per-product blocklist consulted on every send. v1 entries are populated
-- manually; provider-side bounce/complaint webhooks (SES, SendGrid) can
-- populate AUTO_* reasons in a follow-up.
CREATE TABLE email_suppressions (
    id VARCHAR(255) PRIMARY KEY, -- KSUID prefix: esup_
    platform_tenant_id VARCHAR(255) NOT NULL,
    product_id VARCHAR(255) NOT NULL,
    address VARCHAR(320) NOT NULL,
    reason VARCHAR(50) NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (product_id, address),
    CONSTRAINT chk_email_suppressions_reason CHECK (
        reason IN ('MANUAL', 'AUTO_BOUNCE', 'AUTO_COMPLAINT', 'AUTO_UNSUBSCRIBE')
    ),
    CONSTRAINT fk_email_suppressions_product_tenant
        FOREIGN KEY (product_id, platform_tenant_id)
        REFERENCES products(id, platform_tenant_id)
        ON DELETE CASCADE
);

CREATE INDEX idx_email_suppressions_product_id ON email_suppressions(product_id);
CREATE INDEX idx_email_suppressions_platform_tenant_id ON email_suppressions(platform_tenant_id);
