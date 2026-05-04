## ADDED Requirements

### Requirement: BatchCheckAccess RPC
The PolicyReadService SHALL expose a `BatchCheckAccess` RPC that accepts a user node ID, a list of object IDs, and a list of operations, and SHALL return a map of object ID to permission results for all requested operations.

#### Scenario: Batch check for file list
- **WHEN** client sends BatchCheckAccessRequest with user_node_id, 10 object_ids, and operations ["write","delete","share"]
- **THEN** system returns BatchAccessResult with 10 entries, each containing a map of operation → boolean

#### Scenario: Empty object list
- **WHEN** client sends BatchCheckAccessRequest with empty object_ids
- **THEN** system returns empty BatchAccessResult (no error)

#### Scenario: Unknown object ID in batch
- **WHEN** client sends BatchCheckAccessRequest containing an object_id that does not exist in the NGAC graph
- **THEN** system returns all operations as `false` for that object_id (no error thrown)

### Requirement: Drive Batch Access REST Endpoint
The Drive service SHALL expose `POST /api/drive/batch-access` that accepts a JSON body with `object_ids` and `operations`, validates that all objects belong to the caller's current tenant workspace, and returns the batch permission result.

#### Scenario: Valid batch access request
- **WHEN** authenticated user sends POST to /api/drive/batch-access with object_ids belonging to their workspace
- **THEN** system returns permission map for each object and operation

#### Scenario: Cross-tenant object ID rejected
- **WHEN** user sends batch-access request containing object_ids from a different tenant's workspace
- **THEN** system excludes those objects from the result (returns permissions only for valid objects)

#### Scenario: Unauthenticated request
- **WHEN** request lacks a valid JWT
- **THEN** system returns 401 Unauthorized
