## ADDED Requirements

### Requirement: Asset dashboard
The frontend SHALL provide an asset management dashboard showing a summary of assets by type, state distribution, and recent activity.

#### Scenario: View asset dashboard
- **WHEN** a user navigates to the asset management section
- **THEN** the dashboard SHALL display: total assets by type, state distribution chart, recent transitions, and pending requests (if user has approve permission)

### Requirement: Asset type configuration UI
An admin SHALL be able to create and configure asset types through a visual form with field builder and lifecycle editor.

#### Scenario: Create asset type via UI
- **WHEN** an admin clicks "New Asset Type", fills in name, category, adds custom fields (drag-and-drop field builder), and configures lifecycle states
- **THEN** the UI SHALL submit the type definition to the API and display the new type in the types list

#### Scenario: Field builder
- **WHEN** an admin adds a custom field
- **THEN** the UI SHALL provide field type selection (text, number, date, select, boolean), required/optional toggle, and default value input

### Requirement: Asset list and detail views
The frontend SHALL provide list and detail views for assets with filtering, sorting, and lifecycle actions.

#### Scenario: Asset list view
- **WHEN** a user views the asset list
- **THEN** the UI SHALL display assets in a table/card view with type icon, name, state badge, assigned user, and last updated time. Filter controls for type, state, and assigned user SHALL be available

#### Scenario: Asset detail view
- **WHEN** a user clicks on an asset
- **THEN** the detail view SHALL show all custom fields, current state with available transitions as action buttons, assignment info, lifecycle timeline, and linked discussion threads

#### Scenario: Perform lifecycle transition
- **WHEN** a user clicks an available transition button (e.g., "Approve")
- **THEN** the UI SHALL show a confirmation dialog with optional comment field, then call the transition API

### Requirement: Asset request interface
The frontend SHALL provide a request form for employees to request assets and an approval queue for managers.

#### Scenario: Submit request
- **WHEN** an employee clicks "Request Asset", selects type, enters justification
- **THEN** the UI SHALL submit the request and show confirmation with request status

#### Scenario: Approval queue
- **WHEN** a manager views the approval queue
- **THEN** the UI SHALL list pending requests with requester info, type, justification, and approve/reject buttons

### Requirement: Notification center UI
The frontend SHALL display a notification bell icon with unread count badge and a dropdown showing recent notifications.

#### Scenario: Notification bell
- **WHEN** a user is logged in
- **THEN** the UI SHALL show a bell icon in the header with unread notification count as a badge

#### Scenario: Notification dropdown
- **WHEN** a user clicks the bell icon
- **THEN** a dropdown SHALL show recent notifications with type icon, message, timestamp, and read/unread status. Clicking a notification SHALL navigate to the relevant entity (asset, thread, etc.)

#### Scenario: Real-time notification update
- **WHEN** a new notification arrives via WebSocket
- **THEN** the badge count SHALL increment and the notification SHALL appear at the top of the dropdown without page refresh
