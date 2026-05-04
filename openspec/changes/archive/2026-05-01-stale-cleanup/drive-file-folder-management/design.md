# Design — Drive File/Folder Management + Sharing

## Screen Inventory

### S1: Drive File List (Modified)
- **Existing**: `DriveFileList` context menu has: Download, Rename, Move, Share, Delete
- **Change**: Add "Delete permanently" option to context menu with destructive styling

### S2: Delete Confirm Dialog (Existing)
- **Existing**: `DeleteConfirmDialog` in `components/drive/`
- **Change**: Minor — pass `permanent` flag to show stronger warning text

### S3: Share Dialog (New Composite)
- **Composition**: Modal + useContacts + useDriveShares + Button + Spinner
- **Layout**:
  - Modal.Header: "Share {itemName}"
  - Current shares section: List of shared users with revoke (X) button
  - Divider
  - Search input: "Search workspace members..."
  - Member picker: List of workspace contacts with "Share" / "Shared" buttons
  - Permission selector: Dropdown (Read / Read & Write)

### S4: Drive Context Panel — Permissions Tab (Modified)
- **Existing**: `DriveContextPanel` has permissions tab
- **Change**: Use same ShareDialog composition pattern

## Component Mapping

| UI Element | Existing Component | New? |
|---|---|---|
| Delete confirmation | `DeleteConfirmDialog` | No |
| Delete permanently | `ConfirmDialog` | No (reuse) |
| Share dialog shell | `Modal` | No |
| Member search | `<input>` pattern from AddMemberModal | No |
| Member list | Contact row pattern from ChannelInfoPanel | No |
| Share button | `Button variant="ghost"` | No |
| Revoke button | `IconButton` | No |
| Permission dropdown | `<select>` | No |
| Shared badge | `<span>` with surface-container bg | No |

## New Component: `ShareDialog`
**Location**: `frontend/src/components/drive/ShareDialog.tsx`
**Composition**: Modal(Header + search input + contacts list + current shares + permission select)
**Pattern**: Same as `AddMemberModal` from ChannelInfoPanel — proven pattern

## Design Tokens
- All from existing Stitch design system
- No new tokens needed
- 4/8/12/16px spacing scale
- `text-sm` body, `text-xs` labels, Inter font
