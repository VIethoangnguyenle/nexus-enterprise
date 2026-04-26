## ADDED Requirements

### Requirement: Request asset
An employee with `request` permission on an asset type's OA SHALL be able to submit an asset request specifying the type, quantity, justification, and optional preferred specifications.

#### Scenario: Submit asset request
- **WHEN** an engineer calls `POST /api/workspaces/{id}/asset-requests` with type_id (Laptop), quantity 1, and justification "New hire onboarding"
- **THEN** the system SHALL create a request record, emit `asset.request` Kafka event, and return the request with status "pending"

#### Scenario: No request permission denied
- **WHEN** a user without `request` permission on the asset type attempts to submit a request
- **THEN** the system SHALL return 403 Forbidden

### Requirement: Approve or reject request
A manager with `approve` permission on the asset type's OA SHALL be able to approve or reject pending requests with an optional comment.

#### Scenario: Approve request
- **WHEN** an IT admin calls `POST /api/asset-requests/{id}/approve` with comment "Approved, standard config"
- **THEN** the system SHALL update request status to "approved", emit `asset.request` Kafka event with status "approved", and notify the requester

#### Scenario: Reject request
- **WHEN** a manager calls `POST /api/asset-requests/{id}/reject` with reason "Budget exceeded for Q1"
- **THEN** the system SHALL update request status to "rejected", emit event, and notify the requester with the reason

#### Scenario: Cannot approve own request
- **WHEN** a user attempts to approve their own request
- **THEN** the system SHALL reject with 403 "Cannot approve own request"

### Requirement: Assign asset to user
After a request is approved, an admin with `assign` permission SHALL be able to assign a specific asset instance to the requesting user.

#### Scenario: Assign asset from approved request
- **WHEN** an admin calls `POST /api/asset-requests/{id}/assign` with asset_id
- **THEN** the system SHALL update the asset's assigned_to field, transition the asset to "assigned" state, create an NGAC assignment linking the user to the asset, emit `asset.assignment` event, and update the request status to "fulfilled"

#### Scenario: Assign already-assigned asset blocked
- **WHEN** an admin attempts to assign an asset that is already assigned to another user
- **THEN** the system SHALL reject with 409 Conflict "Asset is currently assigned to another user"

### Requirement: Return asset
The assigned user or an admin SHALL be able to initiate an asset return.

#### Scenario: User returns asset
- **WHEN** the assigned user calls `POST /api/assets/{id}/return`
- **THEN** the system SHALL transition the asset to "returned" state, remove the NGAC assignment, update assigned_to to null, and emit `asset.assignment` event

#### Scenario: Admin forces return
- **WHEN** an admin with `manage` permission calls `POST /api/assets/{id}/return` for an asset assigned to another user
- **THEN** the system SHALL process the return and notify the previously assigned user

### Requirement: List requests
A user SHALL be able to list their own requests. A manager SHALL be able to list all pending requests for asset types they have `approve` permission on.

#### Scenario: List my requests
- **WHEN** an employee calls `GET /api/workspaces/{id}/asset-requests?mine=true`
- **THEN** the system SHALL return all requests submitted by the user with status, type, dates

#### Scenario: List pending approvals
- **WHEN** a manager calls `GET /api/workspaces/{id}/asset-requests?status=pending`
- **THEN** the system SHALL return only pending requests for asset types where the manager has `approve` permission
