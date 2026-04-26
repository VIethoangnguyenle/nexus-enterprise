## MODIFIED Requirements

### Requirement: Workspace selector
The frontend SHALL display a workspace selector allowing the user to switch between workspaces they are a member of. A "Create Workspace" option SHALL be available.

#### Scenario: Switch workspace
- **WHEN** a user clicks on a different workspace in the selector
- **THEN** the sidebar and main content SHALL update to show that workspace's channels, documents, and settings

#### Scenario: Create workspace
- **WHEN** a user clicks "Create Workspace" and enters a name
- **THEN** a new workspace SHALL be created via the API and the user SHALL be redirected to it

### Requirement: Sidebar navigation
Within a workspace, the sidebar SHALL display sections for: Documents, Channels, Direct Messages, and Settings (if user has manage permission).

#### Scenario: Sidebar sections
- **WHEN** a user selects a workspace
- **THEN** the sidebar SHALL show categorized navigation: Documents section, Channels section (listing channels the user is a member of), DMs section, and Settings (conditionally)

#### Scenario: Settings visibility
- **WHEN** a user does NOT have `manage` permission on the workspace
- **THEN** the Settings section SHALL NOT be visible in the sidebar

### Requirement: Chat interface
The frontend SHALL provide a chat interface for channels and DMs with message list, input field, and member list.

#### Scenario: View channel messages
- **WHEN** a user clicks on a channel in the sidebar
- **THEN** the main area SHALL display the channel's messages in chronological order with sender name, timestamp, and content

#### Scenario: Send message
- **WHEN** a user types a message and presses Enter or clicks Send
- **THEN** the message SHALL be sent via the API and appear in the chat immediately (optimistic update)

#### Scenario: Real-time message reception
- **WHEN** another user sends a message to the active channel
- **THEN** the message SHALL appear in the chat in real-time via WebSocket without page refresh

### Requirement: Channel management UI
The frontend SHALL provide UI for creating channels and managing channel members.

#### Scenario: Create channel
- **WHEN** a user with `create_channel` permission clicks "Create Channel"
- **THEN** a dialog SHALL appear for entering the channel name and the channel SHALL be created via the API

#### Scenario: Add member to channel
- **WHEN** a channel admin clicks "Add Member" and selects a workspace member
- **THEN** the selected user SHALL be added to the channel's Members UA via the API

### Requirement: Direct message UI
The frontend SHALL provide UI for starting and viewing direct message conversations.

#### Scenario: Start DM
- **WHEN** a user clicks "New Message" and selects a user
- **THEN** a DM channel SHALL be created (or existing one returned) and the chat view SHALL open

### Requirement: Workspace admin panel
The frontend SHALL provide an admin panel (visible only to users with `manage` permission) for managing workspace structure.

#### Scenario: Roles management
- **WHEN** an admin navigates to Settings > Roles
- **THEN** they SHALL see a list of custom UAs (roles/departments) with options to create, edit, and delete

#### Scenario: Permission management
- **WHEN** an admin navigates to Settings > Permissions
- **THEN** they SHALL see a matrix or list of associations (UA → OA + operations) with options to create and delete

#### Scenario: Member management
- **WHEN** an admin navigates to Settings > Members
- **THEN** they SHALL see a list of workspace members with their role assignments and options to change roles, invite new members, or remove members

### Requirement: Preserved document UI
All existing document UI features SHALL be preserved within the workspace context: document list, upload modal, share modal, access check modal, approve/publish actions.

#### Scenario: Document list in workspace
- **WHEN** a user clicks "Documents" in the sidebar
- **THEN** the main area SHALL display workspace documents the user has read access to, with the same card layout and actions as the current system
