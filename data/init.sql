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
    status          TEXT NOT NULL DEFAULT 'draft',
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
-- Messaging Thread Extensions
-- ============================================

ALTER TABLE messages ADD COLUMN IF NOT EXISTS message_type  TEXT NOT NULL DEFAULT 'user' CHECK (message_type IN ('user', 'system'));
ALTER TABLE messages ADD COLUMN IF NOT EXISTS parent_message_id TEXT REFERENCES messages(id);
ALTER TABLE messages ADD COLUMN IF NOT EXISTS linked_entity_type TEXT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS linked_entity_id TEXT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS reply_count INT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_messages_thread ON messages(parent_message_id) WHERE parent_message_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_messages_entity ON messages(linked_entity_type, linked_entity_id) WHERE linked_entity_type IS NOT NULL;

-- Thread participants — tracks who's subscribed to a thread
CREATE TABLE IF NOT EXISTS thread_participants (
    message_id  TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at   TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (message_id, user_id)
);

-- Channel members — denormalized for efficient DM lookup by member pair.
-- NGAC graph remains the source of truth for access control.
CREATE TABLE IF NOT EXISTS channel_members (
    channel_id    TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    ngac_node_id  TEXT NOT NULL,
    joined_at     TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (channel_id, ngac_node_id)
);

CREATE INDEX IF NOT EXISTS idx_channel_members_node ON channel_members(ngac_node_id);



CREATE TABLE IF NOT EXISTS notifications (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type            TEXT NOT NULL,   -- "asset_requested", "asset_approved", "asset_assigned", "thread_reply", "mention"
    title           TEXT NOT NULL,
    body            TEXT NOT NULL DEFAULT '',
    entity_type     TEXT,            -- "asset", "asset_request", "message", etc.
    entity_id       TEXT,
    read            BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notifications_user ON notifications(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_unread ON notifications(user_id) WHERE read = FALSE;

-- ============================================
-- Asset Tables (owned by Asset Service)
-- ============================================

CREATE TABLE IF NOT EXISTS asset_types (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    category        TEXT NOT NULL,
    workspace_id    TEXT NOT NULL REFERENCES workspaces(id),
    fields_schema   JSONB NOT NULL DEFAULT '{}',   -- JSON Schema for custom fields
    lifecycle       JSONB NOT NULL DEFAULT '{}',   -- State machine definition
    ngac_oa_id      TEXT REFERENCES ngac_nodes(id),
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(workspace_id, name)
);

CREATE INDEX IF NOT EXISTS idx_asset_types_workspace ON asset_types(workspace_id);
CREATE INDEX IF NOT EXISTS idx_asset_types_category ON asset_types(workspace_id, category);

CREATE TABLE IF NOT EXISTS assets (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    type_id         TEXT NOT NULL REFERENCES asset_types(id),
    workspace_id    TEXT NOT NULL REFERENCES workspaces(id),
    state           TEXT NOT NULL DEFAULT 'requested',
    custom_fields   JSONB NOT NULL DEFAULT '{}',
    assigned_to     TEXT REFERENCES users(id),
    ngac_node_id    TEXT REFERENCES ngac_nodes(id),
    created_by      TEXT NOT NULL REFERENCES users(id),
    deleted         BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_assets_workspace ON assets(workspace_id) WHERE deleted = FALSE;
CREATE INDEX IF NOT EXISTS idx_assets_type ON assets(type_id) WHERE deleted = FALSE;
CREATE INDEX IF NOT EXISTS idx_assets_state ON assets(workspace_id, state) WHERE deleted = FALSE;
CREATE INDEX IF NOT EXISTS idx_assets_assigned ON assets(assigned_to) WHERE assigned_to IS NOT NULL AND deleted = FALSE;

CREATE TABLE IF NOT EXISTS asset_transitions (
    id              TEXT PRIMARY KEY,
    asset_id        TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    from_state      TEXT NOT NULL,
    to_state        TEXT NOT NULL,
    action          TEXT NOT NULL,
    actor_id        TEXT NOT NULL REFERENCES users(id),
    comment         TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_transitions_asset ON asset_transitions(asset_id, created_at);

CREATE TABLE IF NOT EXISTS asset_requests (
    id              TEXT PRIMARY KEY,
    type_id         TEXT NOT NULL REFERENCES asset_types(id),
    workspace_id    TEXT NOT NULL REFERENCES workspaces(id),
    requester_id    TEXT NOT NULL REFERENCES users(id),
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'fulfilled')),
    justification   TEXT NOT NULL DEFAULT '',
    quantity        INT NOT NULL DEFAULT 1,
    assigned_asset_id TEXT REFERENCES assets(id),
    approver_id     TEXT REFERENCES users(id),
    approver_comment TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_requests_workspace ON asset_requests(workspace_id, status);
CREATE INDEX IF NOT EXISTS idx_requests_requester ON asset_requests(requester_id);
CREATE INDEX IF NOT EXISTS idx_requests_type ON asset_requests(type_id, status);

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

-- ============================================
-- Drive Tables (owned by Drive Service)
-- ============================================

CREATE TABLE IF NOT EXISTS drive_items (
    id              TEXT PRIMARY KEY,
    workspace_id    TEXT NOT NULL REFERENCES workspaces(id),
    drive_context   TEXT NOT NULL DEFAULT 'workspace' CHECK (drive_context IN ('workspace', 'channel', 'dm')),
    drive_context_id TEXT,
    parent_id       TEXT REFERENCES drive_items(id) ON DELETE CASCADE,
    item_type       TEXT NOT NULL CHECK (item_type IN ('file', 'folder')),
    name            TEXT NOT NULL,
    mime_type       TEXT,
    size_bytes      BIGINT,
    object_key      TEXT,
    storage_doc_id  TEXT,
    ngac_node_id    TEXT NOT NULL,
    owner_id        TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'pending', 'trashed', 'deleted')),
    trashed_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_drive_items_unique_name
    ON drive_items(parent_id, name) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_drive_items_parent ON drive_items(parent_id) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_drive_items_workspace ON drive_items(workspace_id, status);
CREATE INDEX IF NOT EXISTS idx_drive_items_context ON drive_items(drive_context, drive_context_id) WHERE status = 'active';

CREATE TABLE IF NOT EXISTS drive_shares (
    id              TEXT PRIMARY KEY,
    drive_item_id   TEXT NOT NULL REFERENCES drive_items(id) ON DELETE CASCADE,
    share_type      TEXT NOT NULL CHECK (share_type IN ('user', 'role', 'workspace', 'public')),
    target_ngac_id  TEXT,
    target_label    TEXT,
    operations      TEXT[] NOT NULL,
    ngac_share_oa   TEXT NOT NULL,
    created_by      TEXT NOT NULL,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_drive_shares_item ON drive_shares(drive_item_id);
CREATE INDEX IF NOT EXISTS idx_drive_shares_target ON drive_shares(target_ngac_id) WHERE target_ngac_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS drive_quotas (
    workspace_id    TEXT PRIMARY KEY REFERENCES workspaces(id),
    max_bytes       BIGINT DEFAULT -1,
    used_bytes      BIGINT DEFAULT 0,
    max_files       INT DEFAULT -1,
    used_files      INT DEFAULT 0,
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);
