## ADDED Requirements

### Requirement: Asset Management Policy Class
The system SHALL create a PC_AssetManagement Policy Class when the first asset type is created in a workspace. All asset-related OA and UA nodes SHALL be scoped under this PC.

#### Scenario: Auto-create PC on first asset type
- **WHEN** the first asset type is created in a workspace
- **THEN** the system SHALL create `PC_AssetManagement` (if not exists), create `{workspace}_Assets` OA under it, and create the type's OA under the workspace Assets OA

#### Scenario: Subsequent types reuse existing PC
- **WHEN** a second asset type is created in a workspace that already has asset types
- **THEN** the system SHALL create the new type's OA under the existing `{workspace}_Assets` OA without creating a new PC

### Requirement: Category OA hierarchy
Asset categories SHALL be represented as intermediate OA nodes between the workspace Assets OA and type-specific OAs, enabling category-level permission grants.

#### Scenario: Category auto-creation
- **WHEN** an asset type is created with category "IT Equipment" and no IT Equipment OA exists
- **THEN** the system SHALL create `{workspace}_IT_Equipment` OA under `{workspace}_Assets`, then create the type OA under it

#### Scenario: Category reuse
- **WHEN** a second type (Monitors) is created in existing category "IT Equipment"
- **THEN** the system SHALL create `{workspace}_Monitors` OA under the existing `{workspace}_IT_Equipment` OA

#### Scenario: Category-level permission inheritance
- **WHEN** an association grants `IT_Admin` UA `[read, write, approve]` on `{workspace}_IT_Equipment` OA
- **THEN** the IT Admin SHALL have those permissions on ALL types and assets under IT Equipment (Laptops, Monitors, etc.) via NGAC inheritance

### Requirement: Per-asset NGAC object
Each asset instance SHALL be represented as an Object (O) in the NGAC graph, assigned to its type's OA.

#### Scenario: Asset object creation
- **WHEN** a new asset "MacBook Pro #042" of type Laptop is created
- **THEN** the system SHALL create an NGAC Object node for the asset and assign it to `{workspace}_Laptops` OA

#### Scenario: Per-asset prohibition
- **WHEN** an admin creates a prohibition denying user X access to a specific asset
- **THEN** `CheckAccess(userX, asset_oa, "read")` SHALL return DENY even if user X has read access to the type via UA association

### Requirement: Asset access associations
The system SHALL support creating associations between workspace UAs and asset OAs for department-level and role-level access control.

#### Scenario: Grant department access to asset category
- **WHEN** an admin calls the API to create association `Engineering_UA → IT_Equipment_OA with [read, request]`
- **THEN** all users in Engineering UA SHALL be able to read and request IT equipment assets

#### Scenario: Revoke department access
- **WHEN** an admin removes the association between a UA and asset OA
- **THEN** users in that UA SHALL lose access to assets under that OA (unless they have access through another path)

### Requirement: Asset permission operations
The NGAC model for assets SHALL support the following operations: `read`, `write`, `request`, `approve`, `assign`, `manage`, `dispose`.

#### Scenario: Operation granularity
- **WHEN** a UA has only `[read, request]` on an asset type OA
- **THEN** users in that UA SHALL be able to view assets and submit requests, but NOT approve, assign, or dispose assets
