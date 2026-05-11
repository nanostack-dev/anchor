-- =============================================
-- Migration 000016: Remove Email Suppressions
-- =============================================
-- Drops the email_suppressions table and its indexes.
-- Suppression list feature removed temporarily; will be reintroduced
-- when provider-side bounce/complaint webhooks are wired up.
-- =============================================

DROP TABLE IF EXISTS email_suppressions;
