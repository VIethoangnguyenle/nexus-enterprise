# Design: Fix Shared Folders Display

## Overview

Wire frontend Drive page để phân biệt "My Folders" vs "Shared Folders" data. Backend đã có API `GET /api/drive/shared-with-me` — chỉ cần frontend gọi đúng API theo tab active.

## Change 1: Drive page — tab-aware data switching

### Problem
Tab "Shared Folders" set `activeTab` state nhưng content area vẫn render data từ `useDriveFolder()`.

### Solution
Thêm conditional data source dựa trên `activeTab`:
- Tab `my-folders` → `useDriveFolder(wsId, currentFolderId)` (giữ nguyên)
- Tab `shared` → `useSharedWithMe()` (fetch `/drive/shared-with-me`)

#### File: `frontend/src/routes/_drive/drive.tsx`

```diff
 function DriveIndex() {
   // ... existing state ...
+  const { data: sharedData, isLoading: sharedLoading } = useSharedWithMe()
   const { data, isLoading, error, refetch } = useDriveFolder(wsId, currentFolderId ?? undefined)

-  const items = data?.items || []
+  // Switch data source based on active tab
+  const isSharedTab = activeTab === 'shared'
+  const items = isSharedTab
+    ? (sharedData?.items || [])
+    : (data?.items || [])
+  const loading = isSharedTab ? sharedLoading : isLoading
```

**Why this approach**: Minimal change. Reuse existing `useSharedWithMe()` hook mà không cần thêm API mới. Tab switching chỉ swap data source, UI render logic giữ nguyên.

**Impact trên UX**:
- Tab "My Folders": Hiển thị folder tree như hiện tại (user's own folders)
- Tab "Shared Folders": Chỉ hiển thị items có trong `drive_shares` table với target là user hoặc user's UAs
- Breadcrumb, folder navigation, context panel vẫn hoạt động — `useSharedWithMe` trả về `DriveItem[]` cùng format

---

## Change 2: DriveSidebar — shared folder tree

### Problem
`FolderSection` component bỏ qua prop `section` (prefix `_section`), hiển thị cùng "All files" cho cả My và Shared.

### Solution
Fetch shared folder data khi `section === 'shared'` và render folder list thực tế.

#### File: `frontend/src/components/drive/DriveSidebar.tsx`

```diff
 function FolderSection({
   label,
-  workspaceId: _wsId,
+  workspaceId,
   onSelect,
-  section: _section,
+  section,
 }: { ... }) {
   const [expanded, setExpanded] = useState(true)
+  const { data: sharedData } = useSharedWithMe()
+  const { data: myData } = useDriveFolder(workspaceId)
+
+  const folders = section === 'shared'
+    ? (sharedData?.items || []).filter(i => i.item_type === 'folder')
+    : (myData?.items || []).filter(i => i.item_type === 'folder')

   return (
     <div className="mb-1">
       {/* ... expand button ... */}
       {expanded && (
         <div className="pl-3 text-caption text-gray-10">
-          <button onClick={() => onSelect?.(null)} ...>
-            <Folder size={12} /> All files
-          </button>
+          {folders.length === 0 ? (
+            <span className="px-1 py-1 text-micro text-gray-9">
+              {section === 'shared' ? 'No shared folders' : 'No folders'}
+            </span>
+          ) : (
+            <>
+              <button onClick={() => onSelect?.(null)} ...>
+                <Folder size={12} /> All files
+              </button>
+              {folders.map(f => (
+                <button key={f.id} onClick={() => onSelect?.(f.id)} ...>
+                  <Folder size={12} /> {f.name}
+                </button>
+              ))}
+            </>
+          )}
         </div>
       )}
     </div>
   )
 }
```

**Why this approach**: FolderSection becomes data-aware. "My Folders" shows workspace root folders, "Shared Folders" shows only items from `shared-with-me` API. Both filter by `item_type === 'folder'` to only show folders in the tree.

---

## Change 3: Empty state cho Shared tab

### Problem
Khi user ở tab "Shared Folders" nhưng chưa có ai share folder cho họ, cần empty state rõ ràng.

### Solution

#### File: `frontend/src/routes/_drive/drive.tsx`

Thêm specific empty state cho shared tab:

```diff
 ) : items.length > 0 ? (
   <DriveFileList ... />
 ) : (
-  <EmptyState icon={...} title="Drive is empty" ... />
+  <EmptyState
+    icon={isSharedTab
+      ? <Users size={40} color="#6366f1" strokeWidth={1.5} />
+      : <FolderOpen size={40} color="#f59e0b" strokeWidth={1.5} />
+    }
+    title={isSharedTab ? 'No shared items' : currentFolderId ? 'Folder is empty' : 'Drive is empty'}
+    description={isSharedTab
+      ? 'Items shared with you will appear here.'
+      : 'Upload files or create folders to get started.'
+    }
+  />
 )
```

---

## Verification

1. **Tab My Folders**: Hiển thị folders mới tạo — unchanged behavior
2. **Tab Shared Folders**: Trống khi chưa share → chỉ hiển thị items khi có `drive_shares` record
3. **Sidebar My Folders**: Hiển thị workspace root folders
4. **Sidebar Shared Folders**: Hiển thị "No shared folders" khi trống, hoặc shared folder list
5. **Breadcrumb**: Vẫn hoạt động khi navigate vào shared folder
6. **Build**: `npm run build` passes
