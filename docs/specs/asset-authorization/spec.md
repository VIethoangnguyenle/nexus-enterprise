# asset-authorization

## Purpose
Decide who may see and act on an asset without putting the asset in the graph. The graph holds attributes, not objects: an asset lives in Postgres under its type, and every decision about it is a decision on the OA of that type.

## Requirements

### Requirement: An asset has no graph node
The system SHALL NOT create a node of type `O` for an asset, and SHALL NOT keep a node ID on the asset row. An asset is authorized on the OA of its type (`asset_types.ngac_oa_id`), read from the type each time it is needed, so there is a single source for which OA an asset answers to. The asset service holds a policy read client only.

#### Scenario: Create an asset
- **WHEN** a caller holding `write` on the asset type's OA creates an asset
- **THEN** the asset row is written and no node, assignment or association is created in the graph
- **AND** the asset reports the type's OA as the attribute it is authorized on (`Asset.ngac_node_id`)

#### Scenario: Delete an asset
- **WHEN** a caller holding `manage` on the asset type's OA deletes an asset
- **THEN** the row is soft-deleted and nothing in the graph changes

#### Scenario: No object nodes remain
- **WHEN** the migration `024_assets_authorized_on_type_oa.sql` has been applied
- **THEN** `ngac_nodes` holds no row with `node_type = 'O'`, and `assets` has no `ngac_node_id` column

#### Scenario: The policy service refuses object nodes
- **WHEN** a caller asks `CreateNode` for a node of type `O`
- **THEN** it fails with InvalidArgument

### Requirement: Legacy blanket grants on the Assets OA are removed
Before the asset tree was granted to the Owners UA alone, every UA under the workspace policy class (Members, tenant, department, channel Members UAs, roles) was associated with `{ws}_Assets` for all eight operations. They were inert only while a second policy class made the in-memory graph deny, so removing that class would have handed every member manage and approve over every asset. Migration 024 SHALL delete, before removing the class, each association on an Assets OA whose UA is not that workspace's Owners UA and whose operations include the full owner set; a partial grant made later through the permissions API is kept. The migration SHALL fail if any such association remains, and SHALL reset the decision caches.

#### Scenario: Member of an old workspace
- **WHEN** 024 runs on a database holding blanket associations from Members, TenantMember, a department and a channel UA to an Assets OA
- **THEN** those associations are gone, the Owners association and a role's `read` grant remain, and the member has no operation on the type OAs

#### Scenario: Create an asset under the wrong workspace
- **WHEN** a caller who may write on a type's OA creates an asset naming a different workspace
- **THEN** the request fails with InvalidArgument and no row is written; a caller without write on the type gets PermissionDenied and learns nothing about its workspace

### Requirement: Asset operations are checked on the type OA
Every operation on one asset SHALL be a check of the corresponding operation on the OA of the asset's type for the caller named by the request metadata.

| Operation | Op | Object |
|---|---|---|
| Create asset | `write` | the type's OA |
| Get asset, asset history | `read` | the type's OA |
| Update asset | `write` | the type's OA |
| Delete asset | `manage` | the type's OA |
| Lifecycle transition | the transition's `ngac_permission` | the type's OA |
| Available transitions | each transition's `ngac_permission`, in one batch | the type's OA |
| Assign / return an asset | `manage` | the type's OA |
| List assets | `read`, in one batch over the workspace's types | each type's OA; unreadable types are filtered out |

A grant on the workspace's Assets OA or on a category OA reaches the type OAs beneath it, so it reaches the assets of every type below.

#### Scenario: Reader of one type
- **WHEN** a caller holds `read` on the OA of type A and asks for an asset of type A
- **THEN** the check is `read` on type A's OA and the asset is returned

#### Scenario: Grant on another type
- **WHEN** a caller holds `read` on the OA of type B only and asks for an asset of type A
- **THEN** the request is denied with PermissionDenied

#### Scenario: One operation short
- **WHEN** a caller holds `write` on the OA of the asset's type and asks to read, delete or approve it
- **THEN** each is denied: a grant carries only the operations it names

#### Scenario: Type without an OA
- **WHEN** an asset's type has no OA recorded
- **THEN** every operation on its assets is denied, whatever else the caller holds

#### Scenario: No caller, or the policy service fails
- **WHEN** the request carries no caller, or the policy service cannot answer
- **THEN** the request is denied; nothing falls through to the protected operation

### Requirement: The asset tree hangs under the workspace's policy class only
Defining an asset type SHALL build, under the workspace's own policy class, `{ws}_Assets` → `{ws}_Category_{category}` → `{ws}_Type_{type id}` (`ngac.AssetsOAName`, `ngac.AssetCategoryOAName`, `ngac.AssetTypeOAName`), and associate the workspace's Owners UA with the Assets OA for every owner operation. The type OA SHALL be named by the type's ID: two types whose names are equal, or only differ in characters the category label drops, get two OAs. No second policy class SHALL sit over the tree: an access is allowed only when the user reaches every policy class the object reaches, and no workspace UA is assigned to a global asset class, so a tree under one would deny its own owners.

#### Scenario: Owner
- **WHEN** a user assigned to the workspace's Owners UA is checked for any owner operation on any type OA of that workspace
- **THEN** every check is ALLOW

#### Scenario: Member or department member
- **WHEN** a user holds only the workspace Members UA, or a department UA, and no association was made for them on the tree
- **THEN** every operation on the tree resolves to DENY

#### Scenario: Another workspace
- **WHEN** a user whose attributes reach only a different workspace's policy class is checked on this workspace's type OA, even with an association aimed straight at it
- **THEN** the check resolves to DENY

#### Scenario: Grants reach downward and carry only their operations
- **WHEN** `read` is associated with a category OA
- **THEN** `read` holds on the type OAs under that category, not on a sibling category's, not on the Assets OA above, and no other operation holds

#### Scenario: Prohibition
- **WHEN** a prohibition on `manage` over one type OA covers a user who is otherwise allowed
- **THEN** `manage` on that type OA is DENY and the same user keeps `manage` on the other types

#### Scenario: Asking about an asset ID
- **WHEN** a check names an asset's ID as the object
- **THEN** it resolves to DENY (the node is not in the graph); the system never answers it from the type

### Requirement: Defining a type is all-or-nothing
Creating an asset type SHALL create its graph nodes and its row as one unit. A failure at any graph write, or at the row insert, SHALL remove the nodes that run created, newest first, and leave the Assets and category OAs that already existed. Running it again SHALL reuse an Assets or category OA that exists rather than create a second, and SHALL treat a failed lookup as a failure, never as "absent".

#### Scenario: Failure part way
- **WHEN** any of the graph writes for a new type fails
- **THEN** the request fails, the nodes it created are deleted, and no type row exists

#### Scenario: Row refused
- **WHEN** every graph write succeeds and the type row is then refused
- **THEN** the whole tree that run created is deleted

#### Scenario: Workspace without a policy class
- **WHEN** the workspace has no policy class recorded
- **THEN** the request fails and nothing is written to the graph
