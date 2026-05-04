# Nuclear DB Clean + Re-seed

## Problem

The database has accumulated 18 users, 15 workspaces, 20 channels, 265 NGAC nodes, and 67 drive items from various testing sessions. Many NGAC nodes are orphaned, workspace-channel relationships are inconsistent, and duplicate files pollute the drive. The system cannot be reliably tested in this state.

## Solution

TRUNCATE all application tables and NGAC graph, then re-seed via API calls to guarantee 100% consistency between application data and NGAC policy graph. No manual SQL inserts for NGAC nodes.

## Scope

### In scope
- TRUNCATE all tables (users, workspaces, channels, messages, drive, assets, NGAC)
- Clean MinIO buckets
- Re-seed 1 user + 1 workspace + 1 channel via API
- Seed sample messages
- Verify NGAC graph integrity after seed

### Out of scope
- Schema changes
- Code changes (all fixes already applied in fix-channel-access-denied)
- Multi-tenant seed data (can be added later)

## Impact
- **Destructive**: All existing data will be permanently deleted
- **Services affected**: All (auth, workspace, messaging, drive, asset, document)
- **Downtime**: Services must be restarted after TRUNCATE for cache consistency
