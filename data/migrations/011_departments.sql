-- 011_departments.sql: Department hierarchy for workspace organization management.

CREATE TABLE IF NOT EXISTS departments (
    id              TEXT PRIMARY KEY,
    workspace_id    TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    parent_id       TEXT REFERENCES departments(id),
    ngac_ua_id      TEXT NOT NULL,
    sort_order      INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(workspace_id, name, parent_id)
);

CREATE INDEX IF NOT EXISTS idx_departments_workspace ON departments(workspace_id);
CREATE INDEX IF NOT EXISTS idx_departments_parent ON departments(parent_id);

-- Link users to departments
ALTER TABLE tenant_users ADD COLUMN IF NOT EXISTS department_id TEXT REFERENCES departments(id);
CREATE INDEX IF NOT EXISTS idx_tenant_users_department ON tenant_users(department_id) WHERE department_id IS NOT NULL;
