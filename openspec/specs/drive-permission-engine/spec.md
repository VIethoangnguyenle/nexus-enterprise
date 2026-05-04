## ADDED Requirements

### Requirement: Permission Cache
The frontend SHALL maintain an in-memory permission cache keyed by `{tenantId}:{objectId}` with a configurable TTL (default 60 seconds). The cache SHALL store resolved permissions (read, write, delete, share) as booleans.

#### Scenario: Cache hit within TTL
- **WHEN** a component requests permissions for an object that was batch-checked less than 60 seconds ago
- **THEN** the cached permissions are returned synchronously without a network call

#### Scenario: Cache miss triggers batch fetch
- **WHEN** a component requests permissions for an object not in cache or with expired TTL
- **THEN** the permission engine queues the object for the next batch fetch cycle

#### Scenario: Tenant switch clears cache
- **WHEN** the user switches tenant context
- **THEN** the entire permission cache is cleared immediately

### Requirement: Batch Permission Hook
The frontend SHALL provide a `usePermissions(objectIds)` hook that batches permission checks for visible items. The hook SHALL coalesce multiple concurrent calls into a single batch request.

#### Scenario: File list renders with permissions
- **WHEN** a folder is loaded containing 50 items
- **THEN** `usePermissions` sends a single batch request for all 50 object IDs and returns a map of permissions

#### Scenario: Incremental items (scroll)
- **WHEN** user scrolls to reveal 20 new items in a virtualized list
- **THEN** `usePermissions` batch-checks only the newly visible items (not already-cached ones)

### Requirement: Permission-Aware Action Rendering
The UI SHALL render file/folder actions (edit, share, delete) only when the corresponding permission is `true`. The UI SHALL NOT render any action before permissions are resolved.

#### Scenario: User with write permission sees edit action
- **WHEN** user has write=true for a file
- **THEN** the edit/rename action is visible on hover

#### Scenario: User without delete permission
- **WHEN** user has delete=false for a file
- **THEN** the trash/delete action is not rendered (no disabled state, completely hidden)

#### Scenario: Permissions loading state
- **WHEN** permissions are being fetched for newly visible items
- **THEN** action buttons are not rendered until permissions resolve (skeleton or hidden)
