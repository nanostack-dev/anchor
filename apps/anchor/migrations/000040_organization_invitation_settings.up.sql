-- =============================================
-- Migration 000040: Organization Invitation Settings
-- =============================================
-- One row per Product that changed its invitation settings. A Product with no
-- row reads the defaults, which live in the service: Product delivery, no
-- template, no accept URL template, seven days.
--
-- Deleting the email template clears the reference. The service then refuses
-- Anchor delivery until a Platform User chooses another template.
--
-- No CHECK constraints and no business triggers.

CREATE TABLE organization_invitation_settings (
    product_id VARCHAR(255) PRIMARY KEY REFERENCES products(id) ON DELETE CASCADE,
    invitation_delivery VARCHAR(20) NOT NULL DEFAULT 'product',
    email_template_id VARCHAR(255) REFERENCES email_templates(id) ON DELETE SET NULL,
    accept_url_template TEXT,
    default_expiry_seconds BIGINT NOT NULL DEFAULT 604800,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_organization_invitation_settings_email_template_id ON organization_invitation_settings(email_template_id);

CREATE TRIGGER update_organization_invitation_settings_updated_at BEFORE UPDATE ON organization_invitation_settings FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
