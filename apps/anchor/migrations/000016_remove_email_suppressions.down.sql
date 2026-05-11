-- =============================================
-- Migration 000016 Down: Restore Email Suppressions
-- =============================================

CREATE TABLE email_suppressions (
    id VARCHAR(255) PRIMARY KEY,
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
