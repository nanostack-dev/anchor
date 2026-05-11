ALTER TABLE email_templates
    ADD COLUMN example_data JSONB NOT NULL DEFAULT '[]'::jsonb;
