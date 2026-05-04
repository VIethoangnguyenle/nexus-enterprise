# Fix Workspace Select Flow

## What

Login thành công hiện đang redirect thẳng tới `/documents`, bỏ qua hoàn toàn trang `/workspace-select`. Trang workspace-select đã được build nhưng không được sử dụng trong flow thực tế.

## Why

1. **Flow bị đứt**: User login xong không được chọn workspace — bị ép vào workspace mặc định.
2. **Multi-workspace bị chặn**: User thuộc nhiều organization không có cách switch workspace tại điểm entry.
3. **Design đã có**: Stitch "Workspace Selection - Final" đã design, code đã build, chỉ thiếu routing.

## Scope

### Frontend Only — 3 files

1. **`login.tsx` (dòng 60)**: Redirect sang `/workspace-select` thay vì `/documents`
2. **`workspace-select.tsx`**: Thêm auto-skip logic khi user chỉ có 1 workspace
3. **`workspace-select.tsx`**: Hiển thị real member count từ API thay vì hardcode "Enterprise Tier" / "Free Tier · 1 Member"

### Không cần thay đổi Backend

- API `GET /workspaces` đã trả member_count
- Không cần thêm endpoint mới

## Out of Scope

- Workspace switcher trong sidebar (flow khác, sau khi user đã chọn workspace)
- "Join an Organization" button functionality (chưa có API)
- User avatar trong workspace cards (chưa có avatar upload)
