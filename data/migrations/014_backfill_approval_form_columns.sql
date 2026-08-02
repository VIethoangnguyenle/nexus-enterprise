-- 014_backfill_approval_form_columns.sql
--
-- Migration 010 declared that approval_templates.form_fields and
-- approval_requests.form_data_json were being added "via the tenant
-- provisioner", and then contained no executable SQL at all — it is a file of
-- comments. The provisioning function was updated, but it builds tables with
-- CREATE TABLE IF NOT EXISTS, so re-running it over a schema that already
-- exists is a no-op. Every tenant provisioned before that function changed is
-- therefore permanently missing both columns, and the approval queries that
-- select them fail for those tenants.
--
-- This walks the tenant registry and adds the columns wherever they are absent.
-- It is idempotent: ADD COLUMN IF NOT EXISTS over a schema that already has
-- them does nothing.

DO $$
DECLARE
    rec RECORD;
BEGIN
    FOR rec IN
        SELECT schema_name
        FROM tenant_schemas
        WHERE schema_name IN (SELECT nspname FROM pg_namespace)
    LOOP
        IF to_regclass(format('%I.approval_templates', rec.schema_name)) IS NOT NULL THEN
            EXECUTE format(
                'ALTER TABLE %I.approval_templates ADD COLUMN IF NOT EXISTS form_fields JSONB DEFAULT NULL',
                rec.schema_name);
        END IF;

        IF to_regclass(format('%I.approval_requests', rec.schema_name)) IS NOT NULL THEN
            EXECUTE format(
                'ALTER TABLE %I.approval_requests ADD COLUMN IF NOT EXISTS form_data_json JSONB DEFAULT NULL',
                rec.schema_name);
        END IF;
    END LOOP;
END $$;
