-- NGAC Platform Database Schema
-- Shared PostgreSQL instance for all microservices

-- ============================================
-- NGAC Core Tables (owned by Policy Service)
-- ============================================

CREATE TABLE IF NOT EXISTS ngac_nodes (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    node_type  TEXT NOT NULL CHECK (node_type IN ('U','UA','O','OA','PC')),
    properties JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ngac_assignments (
    id        TEXT PRIMARY KEY,
    child_id  TEXT NOT NULL REFERENCES ngac_nodes(id) ON DELETE CASCADE,
    parent_id TEXT NOT NULL REFERENCES ngac_nodes(id) ON DELETE CASCADE,
    UNIQUE(child_id, parent_id)
);

CREATE TABLE IF NOT EXISTS ngac_associations (
    id         TEXT PRIMARY KEY,
    ua_id      TEXT NOT NULL REFERENCES ngac_nodes(id) ON DELETE CASCADE,
    oa_id      TEXT NOT NULL REFERENCES ngac_nodes(id) ON DELETE CASCADE,
    operations TEXT[] NOT NULL,
    UNIQUE(ua_id, oa_id)
);

-- NGAC indexes
CREATE INDEX IF NOT EXISTS idx_assignments_child ON ngac_assignments(child_id);
CREATE INDEX IF NOT EXISTS idx_assignments_parent ON ngac_assignments(parent_id);
CREATE INDEX IF NOT EXISTS idx_associations_ua ON ngac_associations(ua_id);
CREATE INDEX IF NOT EXISTS idx_associations_oa ON ngac_associations(oa_id);
CREATE INDEX IF NOT EXISTS idx_nodes_type ON ngac_nodes(node_type);
CREATE INDEX IF NOT EXISTS idx_nodes_name ON ngac_nodes(name);

-- ============================================
-- Auth Tables (owned by Auth Service)
-- ============================================

CREATE TABLE IF NOT EXISTS users (
    id          TEXT PRIMARY KEY,
    username    TEXT UNIQUE NOT NULL,
    password    TEXT NOT NULL,
    ngac_node   TEXT REFERENCES ngac_nodes(id),
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- ============================================
-- Workspace Tables (owned by Workspace Service)
-- ============================================

CREATE TABLE IF NOT EXISTS workspaces (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    description     TEXT DEFAULT '',
    owner_id        TEXT NOT NULL REFERENCES users(id),
    ngac_pc_id      TEXT REFERENCES ngac_nodes(id),
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_workspaces_owner ON workspaces(owner_id);

-- ============================================
-- Document Tables (owned by Document Service)
-- ============================================

CREATE TABLE IF NOT EXISTS documents (
    id              TEXT PRIMARY KEY,
    title           TEXT NOT NULL,
    filename        TEXT NOT NULL,
    mime_type       TEXT,
    owner_id        TEXT REFERENCES users(id),
    ngac_node       TEXT REFERENCES ngac_nodes(id),
    workspace_id    TEXT REFERENCES workspaces(id),
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_documents_workspace ON documents(workspace_id);
CREATE INDEX IF NOT EXISTS idx_documents_owner ON documents(owner_id);

-- ============================================
-- Messaging Tables (owned by Messaging Service)
-- ============================================

CREATE TABLE IF NOT EXISTS channels (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    channel_type    TEXT NOT NULL CHECK (channel_type IN ('workspace', 'private', 'dm')),
    workspace_id    TEXT REFERENCES workspaces(id),
    ngac_oa_id      TEXT REFERENCES ngac_nodes(id),
    ngac_ua_id      TEXT REFERENCES ngac_nodes(id),
    created_by      TEXT REFERENCES users(id),
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS messages (
    id              TEXT PRIMARY KEY,
    channel_id      TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    sender_id       TEXT NOT NULL REFERENCES users(id),
    content         TEXT NOT NULL,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_channels_workspace ON channels(workspace_id);
CREATE INDEX IF NOT EXISTS idx_channels_type ON channels(channel_type);
CREATE INDEX IF NOT EXISTS idx_messages_channel ON messages(channel_id);
CREATE INDEX IF NOT EXISTS idx_messages_created ON messages(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_channel_time ON messages(channel_id, created_at DESC);

-- ============================================
-- Seed Data: Global NGAC Infrastructure
-- ============================================

INSERT INTO ngac_nodes (id, name, node_type, properties) VALUES
  ('pc-global', 'PC_Global', 'PC', '{"scope": "global"}'),
  ('ua-public-users', 'PublicUsers', 'UA', '{}'),
  ('oa-public-docs', 'PublicDocs', 'OA', '{}')
ON CONFLICT (id) DO NOTHING;

INSERT INTO ngac_assignments (id, child_id, parent_id) VALUES
  ('asg-pub-users-global', 'ua-public-users', 'pc-global'),
  ('asg-pub-docs-global', 'oa-public-docs', 'pc-global')
ON CONFLICT (child_id, parent_id) DO NOTHING;

INSERT INTO ngac_associations (id, ua_id, oa_id, operations) VALUES
  ('assoc-pub-read', 'ua-public-users', 'oa-public-docs', ARRAY['read'])
ON CONFLICT (ua_id, oa_id) DO UPDATE SET operations = ARRAY['read'];

