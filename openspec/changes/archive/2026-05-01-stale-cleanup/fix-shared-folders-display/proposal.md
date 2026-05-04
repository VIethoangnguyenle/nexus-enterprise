# Fix Shared Folders Display

## Problem

"Shared Folders" trong Drive sidebar và tabs hiển thị **tất cả folders của user**, bao gồm cả folders mới tạo chưa hề được share. Đây là bug UX nghiêm trọng — user thấy folder riêng tư của mình xuất hiện trong "Shared Folders", gây nhầm lẫn về quyền truy cập.

## Root Cause

Frontend có 2 sections "My Folders" và "Shared Folders" nhưng **cả hai đều fetch cùng một API** (`useDriveFolder`) mà không phân biệt data:

1. **DriveSidebar**: `FolderSection` nhận prop `section: 'my' | 'shared'` nhưng bỏ qua (prefix `_section`). Cả hai render cùng "All files" → navigate về root.

2. **Drive page tabs**: Tab "Shared Folders" chỉ set `activeTab` state nhưng không thay đổi data query — vẫn gọi `useDriveFolder(wsId)`.

3. **Backend**: API `GET /api/drive/shared-with-me` đã tồn tại và hoạt động đúng, nhưng frontend **không gọi** khi user ở tab "Shared Folders".

## Scope

### In scope
- Wire tab "Shared Folders" trong Drive page để gọi `useSharedWithMe()` thay vì `useDriveFolder()`
- Wire sidebar "Shared Folders" section để hiển thị shared folder tree thực tế
- Hiển thị empty state đúng khi chưa có folder nào được share

### Out of scope
- Sharing UI flow (đã có `CreateShare`, `RevokeShare` API)
- NGAC policy changes (backend share logic đã đúng)
- "Shared" tab ở Home view (separate feature)

## Impact
- **Services affected**: Frontend only (Drive page, DriveSidebar)
- **Risk**: Very low — chỉ rewire frontend, backend không thay đổi
- **Users affected**: Tất cả users — cải thiện accuracy của Shared Folders view
