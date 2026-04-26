## ADDED Requirements

### Requirement: Login and registration pages
The frontend SHALL provide login and registration forms. Registration SHALL collect username, password, company selection, and department selection.

#### Scenario: User registers and logs in
- **WHEN** a user fills the registration form and submits
- **THEN** the account is created and the user is redirected to the dashboard

### Requirement: Document dashboard
The frontend SHALL display a dashboard showing all documents accessible to the current user, organized with tabs or filters for: My Documents, Shared With Me, Public Documents.

#### Scenario: Dashboard loads accessible documents
- **WHEN** a logged-in user views the dashboard
- **THEN** they see documents filtered by their NGAC access with document title, type, status (draft/approved), and owner

### Requirement: Document upload interface
The frontend SHALL provide a file upload form with title input and file picker. Uploaded documents appear as drafts in the dashboard.

#### Scenario: Upload a document
- **WHEN** a user selects a file, enters a title, and clicks Upload
- **THEN** the document is uploaded and appears in the dashboard as a draft

### Requirement: Sharing interface
The frontend SHALL provide a sharing dialog for approved documents. The dialog SHALL show available companies and departments to share with, and operation checkboxes (read, write).

#### Scenario: Share a document
- **WHEN** a user clicks Share on an approved document, selects Beta_Engineering, checks [read], and confirms
- **THEN** the sharing API is called and the document becomes accessible to Beta_Engineering

### Requirement: Approval workflow interface
The frontend SHALL show a review/approve button on draft documents for users with approve access. After approval, the document status changes to "Approved".

#### Scenario: Approve a document
- **WHEN** a reviewer clicks Approve on a draft document
- **THEN** the document moves from DraftDocs to ApprovedDocs and its status updates in the UI

### Requirement: Public toggle interface
The frontend SHALL provide a "Make Public" / "Make Private" toggle on approved documents. This toggles the PublicDocs OA assignment through the API.

#### Scenario: Make document public
- **WHEN** a user clicks "Make Public" on an approved document
- **THEN** the publish API is called and the document becomes visible to all users

### Requirement: Permission explanation view
The frontend SHALL display the access explanation for any document when requested. Users can click "Why can I access this?" or "Why is access denied?" to see the NGAC graph path explanation.

#### Scenario: View access explanation
- **WHEN** a user clicks the explanation button on a document
- **THEN** a modal shows the NGAC traversal path explaining the access decision
