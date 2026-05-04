-- Migration 007: Schema-per-Tenant for Approval Workflow Engine
-- Creates tenant_schemas registry, approval tables template, and provisioning function.
-- Each tenant gets an isolated PostgreSQL schema containing their business data.

-- ============================================
-- 1. Tenant schema registry (public schema)
-- ============================================

CREATE TABLE IF NOT EXISTS tenant_schemas (
    tenant_id     TEXT PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
    schema_name   TEXT NOT NULL UNIQUE,
    status        TEXT NOT NULL DEFAULT 'provisioning'
                  CHECK (status IN ('provisioning', 'active', 'migrating', 'disabled')),
    version       INT NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    updated_at    TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tenant_schemas_status ON tenant_schemas(status);

-- ============================================
-- 2. Function: provision tenant schema
-- Creates the schema and all approval tables.
-- Called by the approval service during tenant onboarding.
-- ============================================

CREATE OR REPLACE FUNCTION provision_tenant_schema(p_tenant_id TEXT)
RETURNS TEXT AS $$
DECLARE
    v_schema TEXT;
BEGIN
    -- Derive schema name from tenant ID (first 8 chars of UUID)
    v_schema := 'tenant_' || REPLACE(LEFT(p_tenant_id, 8), '-', '');

    -- Create schema
    EXECUTE format('CREATE SCHEMA IF NOT EXISTS %I', v_schema);

    -- Approval templates
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.approval_templates (
            id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            name            TEXT NOT NULL,
            entity_type     TEXT NOT NULL,
            is_active       BOOLEAN DEFAULT true,
            priority        INT DEFAULT 0,
            form_fields     JSONB DEFAULT NULL,
            created_by      TEXT NOT NULL,
            created_at      TIMESTAMPTZ DEFAULT NOW(),
            updated_at      TIMESTAMPTZ DEFAULT NOW()
        )', v_schema);

    -- Approval conditions
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.approval_conditions (
            id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            template_id     UUID NOT NULL REFERENCES %I.approval_templates(id) ON DELETE CASCADE,
            field           TEXT NOT NULL,
            operator        TEXT NOT NULL,
            value           JSONB NOT NULL
        )', v_schema, v_schema);

    -- Approval steps
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.approval_steps (
            id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            template_id     UUID NOT NULL REFERENCES %I.approval_templates(id) ON DELETE CASCADE,
            step_order      INT NOT NULL,
            name            TEXT NOT NULL,
            approver_type   TEXT NOT NULL,
            approver_value  TEXT,
            required_count  INT DEFAULT 1,
            timeout_hours   INT,
            UNIQUE(template_id, step_order)
        )', v_schema, v_schema);

    -- Approval requests (with scope_oa_id for department visibility)
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.approval_requests (
            id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            entity_type     TEXT NOT NULL,
            entity_id       UUID NOT NULL,
            template_id     UUID REFERENCES %I.approval_templates(id),
            template_name   TEXT NOT NULL DEFAULT '''',
            template_snapshot JSONB NOT NULL,
            form_data_json  JSONB DEFAULT NULL,
            current_step    INT DEFAULT 1,
            status          TEXT DEFAULT ''pending''
                            CHECK (status IN (''pending'',''approved'',''rejected'',''cancelled'')),
            scope_oa_id     TEXT NOT NULL,
            department_id   TEXT NOT NULL,
            created_by      TEXT NOT NULL,
            created_at      TIMESTAMPTZ DEFAULT NOW(),
            completed_at    TIMESTAMPTZ
        )', v_schema, v_schema);

    -- Approval assignments (denormalized for fast queries)
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.approval_assignments (
            id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            request_id      UUID NOT NULL REFERENCES %I.approval_requests(id) ON DELETE CASCADE,
            step_order      INT NOT NULL,
            user_node_id    TEXT NOT NULL,
            grant_source    TEXT NOT NULL,
            status          TEXT DEFAULT ''pending''
                            CHECK (status IN (''pending'',''approved'',''rejected'',''skipped'',''revoked'')),
            acted_at        TIMESTAMPTZ,
            comment         TEXT
        )', v_schema, v_schema);

    -- Audit log (append-only)
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.approval_audit_log (
            id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            request_id      UUID NOT NULL,
            action          TEXT NOT NULL,
            actor_node_id   TEXT NOT NULL,
            step_order      INT,
            detail          JSONB,
            ip_address      INET,
            created_at      TIMESTAMPTZ DEFAULT NOW()
        )', v_schema);

    -- Indexes for query patterns
    -- Tab 1: Pending (load all, no paging)
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_aa_pending ON %I.approval_assignments(user_node_id, status) WHERE status = ''pending''', v_schema);
    -- Tab 2: History (cursor paging on acted_at)
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_aa_history ON %I.approval_assignments(user_node_id, status, acted_at DESC) WHERE status IN (''approved'', ''rejected'')', v_schema);
    -- Tab 3: My requests (cursor paging on created_at)
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_ar_creator ON %I.approval_requests(created_by, created_at DESC)', v_schema);
    -- Tab 4: Department requests (scope-based)
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_ar_scope ON %I.approval_requests(scope_oa_id, status, created_at DESC)', v_schema);
    -- Reconciliation: find pending assignments by grant_source
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_aa_grant_source ON %I.approval_assignments(grant_source, status) WHERE status = ''pending''', v_schema);
    -- Audit trail
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_audit_request ON %I.approval_audit_log(request_id, created_at)', v_schema);

    -- Append-only trigger: prevent UPDATE and DELETE on audit_log
    EXECUTE format('
        CREATE OR REPLACE FUNCTION %I.audit_log_immutable() RETURNS TRIGGER AS $t$
        BEGIN
            RAISE EXCEPTION ''approval_audit_log is append-only: %% not allowed'', TG_OP;
        END;
        $t$ LANGUAGE plpgsql;
    ', v_schema);
    EXECUTE format('
        DROP TRIGGER IF EXISTS trg_audit_immutable_update ON %I.approval_audit_log;
        CREATE TRIGGER trg_audit_immutable_update
            BEFORE UPDATE ON %I.approval_audit_log
            FOR EACH ROW EXECUTE FUNCTION %I.audit_log_immutable();
    ', v_schema, v_schema, v_schema);
    EXECUTE format('
        DROP TRIGGER IF EXISTS trg_audit_immutable_delete ON %I.approval_audit_log;
        CREATE TRIGGER trg_audit_immutable_delete
            BEFORE DELETE ON %I.approval_audit_log
            FOR EACH ROW EXECUTE FUNCTION %I.audit_log_immutable();
    ', v_schema, v_schema, v_schema);

    -- Register in public registry
    INSERT INTO tenant_schemas (tenant_id, schema_name, status, version)
    VALUES (p_tenant_id, v_schema, 'active', 1)
    ON CONFLICT (tenant_id) DO UPDATE SET status = 'active', updated_at = NOW();

    RETURN v_schema;
END;
$$ LANGUAGE plpgsql;
