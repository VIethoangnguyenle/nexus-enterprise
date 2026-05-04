## Tasks

### Change 1: Drive page tab-aware data switching
- [x] Import `useSharedWithMe` in `drive.tsx`
- [x] Add `useSharedWithMe()` query alongside existing `useDriveFolder`
- [x] Switch `items` and `loading` based on `activeTab === 'shared'`
- [x] Disable folder creation/upload actions when in shared tab (user shouldn't create in shared view)
- [x] Verify tab switching works without page reload

### Change 2: DriveSidebar shared folder tree
- [x] Import `useSharedWithMe` and `useDriveFolder` in `DriveSidebar.tsx`
- [x] Remove underscore prefix from `section` and `workspaceId` params in `FolderSection`
- [x] Fetch and display actual folder list per section
- [x] Show "No shared folders" empty state for shared section
- [x] Verify sidebar folder click navigates correctly

### Change 3: Empty state for Shared tab
- [x] Add `Users` icon import from lucide-react
- [x] Conditional empty state message based on `isSharedTab`
- [x] Verify empty state renders correctly when no shared items

### Verification
- [x] Tab "My Folders" → shows only user's own folders
- [x] Tab "Shared Folders" → empty when nothing shared
- [x] Sidebar "Shared Folders" → shows "No shared folders" initially
- [x] `npm run build` passes
- [x] Visual check at 375px, 768px, 1280px viewports
