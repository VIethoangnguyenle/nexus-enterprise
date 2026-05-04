-- Migration 010: Add dynamic form fields to approval module
-- Adds form_fields JSONB column to templates (field definitions)
-- Adds form_data_json JSONB column to requests (submitted values)

-- This migration runs on ALL tenant schemas via the tenant provisioner.
-- For existing schemas, run manually:
-- ALTER TABLE {schema}.approval_templates ADD COLUMN IF NOT EXISTS form_fields JSONB DEFAULT NULL;
-- ALTER TABLE {schema}.approval_requests ADD COLUMN IF NOT EXISTS form_data_json JSONB DEFAULT NULL;

-- The tenant schema provisioner (007_tenant_schema_approval.sql function) should be updated
-- to include these columns in CREATE TABLE statements.
-- However, for safety, this migration adds them via ALTER TABLE to existing schemas.

-- Note: This file documents the change. The actual ALTER TABLE must run per-tenant schema.
-- The approval service handles NULL gracefully (no form fields = legacy template).
