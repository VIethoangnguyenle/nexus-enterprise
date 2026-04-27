## Context

The NGAC platform currently has a Document Service that handles both file storage (MinIO) and access control (NGAC). This creates a monolithic file service that cannot support a Google Drive-like experience with folder hierarchies, granular sharing, and chat integration.

The platform uses:
- MinIO for object storage (bucket-per-workspace: `ws-{id}`)
- Presigned URLs for upload/download (3-step flow: get URL → PUT to MinIO → confirm)
- NGAC graph for access control (files are O nodes, folders are OA nodes)
- Messaging with `linked_entity_type/id` fields for entity-linked messages
- Kafka (Redpanda) for async event publishing
- Redis for WebSocket pub/sub and caching
- Policy Service split: PolicyReadService (replicas) + PolicyWriteService (single writer)

### Current NGAC Graph for Files

```
PC_Workspace
├── Owners_UA ──association──▶ Documents_OA [all ops]
├── Members_UA ──association──▶ Documents_OA [read]
└── Documents_OA
    ├── DraftDocs_OA
    │   └── file.pdf (O node)
    └── ApprovedDocs_OA
```

### Target Architecture

```
Gateway (:8080)
    │
    ├──▶ Drive Service (:50057)
    │        ├──▶ Policy Read/Write (NGAC)
    │        ├──▶ Document Storage Service (:50054)
    │        │         └──▶ MinIO
    │        └──▶ PostgreSQL (drive_items, drive_shares, drive_quotas)
    │
    ├──▶ Messaging Service (:50055)
    │        └──▶ Drive Service (channel drive creation, file attachment)
    │
    └──▶ Workspace Service (:50053)
             └──▶ Drive Service (root drive creation)
```

## Goals / Non-Goals

**Goals:**
- Dedicated Drive Service owning all file organization, hierarchy, and access control
- Document Service refactored to pure storage API (no NGAC dependency)
- Folder hierarchy with NGAC OA nodes — sharing a folder shares all contents
- Every channel and DM gets a drive for file attachments
- Public sharing ("anyone with the link") via NGAC PublicUsers UA
- Storage quota infrastructure (default unlimited)
- Move within same drive, copy across drives
- Migration path for existing documents

**Non-Goals:**
- Real-time collaborative editing (Phase 2+)
- Full-text file content search (Phase 2+)
- File versioning / history (Phase 2+)
- Star/favorite files (Phase 2)
- File comments (could use thread system later)
- Thumbnail generation / image processing
- Offline access
- Mobile-specific UI

## Decisions

### D1: Drive Service as Separate Microservice

**Choice: New `drive` service at `:50057`, independent from Document Service**

Drive owns: folder hierarchy, file metadata, NGAC access control, sharing, quotas.
Document Service becomes: presigned URL generation, MinIO object CRUD, no NGAC awareness.

**Why:**
- Clean separation of concerns: storage vs. organization
- Drive can evolve independently (versioning, search) without touching storage layer
- Document Storage Service becomes reusable by other features (e.g., asset attachments)
- Follows the "thin storage, smart orchestration" pattern used by Google Drive, Dropbox

**Trade-off:** Extra network hop (Drive → Document for storage ops). Acceptable for sub-millisecond Docker network latency.

### D2: Folder Hierarchy — DB + NGAC Dual Model

**Choice: `drive_items` table with `parent_id` for tree structure, NGAC OA nodes for access control**

```sql
drive_items.parent_id  → tree navigation (fast SQL recursive CTE)
NGAC OA assignments    → access control (CheckAccess traversal)
```

**Why:**
- SQL recursive CTEs are fast for tree queries (breadcrumb, subtree listing)
- NGAC graph is optimized for access decisions, not tree navigation
- Rich metadata (size, mime_type, updated_at, status) belongs in SQL, not NGAC properties
- Google Drive, Dropbox, SharePoint all use DB for hierarchy + separate authz system

**Sync guarantee:** When creating/moving/deleting drive_items, both DB and NGAC are updated in the same request. If NGAC fails, DB transaction rolls back.

### D3: Channel Drive — OA Under Channel Content

**Choice: Each channel/DM gets a `Ch_{name}_Drive` OA assigned under the channel's Content OA**

```
Ch_general_Content_OA
├── Ch_general_Drive_OA      ← channel drive
│   ├── meeting_notes.pdf (O)
│   └── design_spec.png (O)
└── (message access scope)

Ch_general_Members_UA ──association──▶ Ch_general_Content_OA [read, write]
```

**Why:**
- Channel members automatically get read/write on channel drive (inherited from Content OA association)
- No extra NGAC setup needed per channel drive
- Files uploaded in chat inherit channel access — only channel members can see them
- DMs work identically — only 2 participants have access

**Lifecycle:** Channel drive is created when channel is created. If channel is deleted, drive items are soft-deleted (trashed).

### D4: Move Within Drive, Copy Across Drives

**Choice: Move = change parent_id + NGAC reassignment. Copy = new drive_item + MinIO CopyObject**

**Move (within same drive context):**
- Update `drive_items.parent_id`
- NGAC: `RemoveAssignment(file, old_folder)` + `CreateAssignment(file, new_folder)`
- Explicit shares (direct associations) survive the move
- Inherited permissions change based on new parent

**Copy (cross-context: channel→workspace, DM→workspace, workspace→workspace):**
- New `drive_item` row with new ID, new NGAC O node
- MinIO `CopyObject` (server-side, no re-upload)
- Original untouched in source drive
- New copy inherits permissions from destination

**Why:**
- Matches user mental model: moving within your drive is reorganizing; taking something from a chat into your workspace is copying
- Prevents permission confusion (channel file doesn't lose channel access when someone copies it)
- MinIO CopyObject is O(1) — no bandwidth cost

### D5: Public Sharing via NGAC PublicUsers UA

**Choice: Three share levels, all implemented through NGAC associations**

```
Level 1 — User/Role share:
  CreateAssociation(target_UA → file_ShareOA, [read/write])

Level 2 — Workspace share ("anyone in workspace"):
  CreateAssociation(Workspace_Members_UA → file_ShareOA, [read])

Level 3 — Public ("anyone with link"):
  CreateAssociation(PublicUsers_UA → file_ShareOA, [read])
```

`PublicUsers_UA` is a UA under `PC_Global` that all registered users are assigned to.

**Why:**
- Single mechanism (NGAC associations) for all share levels
- Revoking is always `RemoveAssociation` — consistent
- Public share still requires authentication (registered user) — no anonymous access
- Can add "anyone on the internet" later with a special anonymous user node

### D6: Storage Quotas — Infrastructure First

**Choice: `drive_quotas` table with default `-1` (unlimited)**

```sql
CREATE TABLE drive_quotas (
    workspace_id TEXT PRIMARY KEY,
    max_bytes    BIGINT DEFAULT -1,   -- -1 = unlimited
    used_bytes   BIGINT DEFAULT 0,
    max_files    INT DEFAULT -1,
    used_files   INT DEFAULT 0
);
```

Quota checked before upload, updated atomically on confirm/delete.

**Why:**
- Infrastructure cost is near-zero (one table, one check)
- Avoids retrofitting quotas later (schema change, backfill)
- Default unlimited means no user-facing change until quotas are needed
- Admin can set limits per workspace when needed

### D7: Document Storage Service Proto Redesign

**Choice: Rename `DocumentService` to `DocumentStorageService`, remove all NGAC-related RPCs**

```protobuf
service DocumentStorageService {
  rpc GetUploadURL(GetUploadURLReq) returns (GetUploadURLResp);
  rpc ConfirmUpload(ConfirmUploadReq) returns (ConfirmUploadResp);
  rpc GetDownloadURL(GetDownloadURLReq) returns (GetDownloadURLResp);
  rpc DeleteObject(DeleteObjectReq) returns (Empty);
  rpc CopyObject(CopyObjectReq) returns (CopyObjectResp);
  rpc GetObjectInfo(GetObjectInfoReq) returns (ObjectInfo);
}
```

No `Share`, no `Approve`, no `Publish`, no `CheckAccess`, no `List`.

**Why:**
- Document Service no longer needs Policy Service connection
- Simpler, faster, easier to test
- Can be reused by Asset Service or any future service needing object storage

### D8: Database Schema

```sql
CREATE TABLE drive_items (
    id              TEXT PRIMARY KEY,
    workspace_id    TEXT NOT NULL REFERENCES workspaces(id),
    drive_context   TEXT NOT NULL DEFAULT 'workspace',  -- 'workspace', 'channel', 'dm'
    drive_context_id TEXT,                               -- channel_id or NULL for workspace
    parent_id       TEXT REFERENCES drive_items(id),     -- NULL = drive root
    item_type       TEXT NOT NULL CHECK (item_type IN ('file', 'folder')),
    name            TEXT NOT NULL,
    mime_type       TEXT,
    size_bytes      BIGINT,
    object_key      TEXT,                                -- MinIO path
    storage_doc_id  TEXT,                                -- Document Storage Service ID
    ngac_node_id    TEXT NOT NULL,
    owner_id        TEXT NOT NULL REFERENCES users(id),
    status          TEXT DEFAULT 'active' CHECK (status IN ('active', 'trashed', 'deleted')),
    trashed_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(parent_id, name) WHERE status = 'active'
);

CREATE TABLE drive_shares (
    id              TEXT PRIMARY KEY,
    drive_item_id   TEXT NOT NULL REFERENCES drive_items(id),
    share_type      TEXT NOT NULL CHECK (share_type IN ('user','role','workspace','public')),
    target_ngac_id  TEXT,          -- NULL for public shares
    target_label    TEXT,          -- display name for UI
    operations      TEXT[] NOT NULL,
    ngac_share_oa   TEXT NOT NULL, -- the share OA node
    created_by      TEXT NOT NULL,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE drive_quotas (
    workspace_id    TEXT PRIMARY KEY REFERENCES workspaces(id),
    max_bytes       BIGINT DEFAULT -1,
    used_bytes      BIGINT DEFAULT 0,
    max_files       INT DEFAULT -1,
    used_files      INT DEFAULT 0,
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);
```

## Risks / Trade-offs

- **[Service dependency chain]** Drive → Document Storage → MinIO creates a 3-hop chain for uploads. Mitigated by presigned URLs (client uploads directly to MinIO, only metadata goes through services).
- **[NGAC graph size]** Every file and folder adds nodes to the NGAC graph. Mitigated by materialized views and CQRS read replicas already in place.
- **[DB + NGAC sync]** Hierarchy in two places (drive_items.parent_id and NGAC assignments). Mitigated by updating both in same request handler with NGAC-first approach (if NGAC fails, don't update DB).
- **[Migration complexity]** Existing documents need to be migrated to drive_items. Mitigated by coexistence period where both systems work.
- **[Messaging coupling]** Messaging Service must call Drive Service for channel drive creation. Mitigated by making it non-fatal (channel works without drive if Drive Service is temporarily unavailable).
- **[Cross-service file copy]** Copying files between drives requires MinIO CopyObject + NGAC node creation. Mitigated by server-side copy (no data transfer).
