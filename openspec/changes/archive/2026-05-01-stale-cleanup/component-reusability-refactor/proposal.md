# Component Reusability Refactor

## What
Refactor tất cả UI components mới tạo gần đây (DeleteConfirmDialog, MoveItemDialog, FolderTreeSelect) để sử dụng đúng component primitives và composites đã có, đồng thời nâng cấp `Modal` composite lên M3 tokens. Thêm skill bắt buộc "component-reuse-checklist" để ngăn code rác tái diễn.

## Why

### Vấn đề phát hiện
- **Modal compound** (`composites/Modal.tsx`) đã tồn tại nhưng bị bỏ qua — 2 dialog mới tự viết lại toàn bộ modal shell (backdrop, click-outside, ESC key, container)
- **Spinner primitive** đã tồn tại nhưng inline SVG spinner copy-paste 4 lần
- **IconButton primitive** đã tồn tại nhưng close button viết inline 2 lần
- **Button variant="danger"** đã tồn tại nhưng danger button viết inline 18 dòng
- **Modal.tsx vẫn dùng legacy tokens** (`bg-gray-5`, `text-gray-13`) — chưa migrate M3

### Hậu quả nếu không fix
- Mỗi feature mới → thêm 1 modal tự viết → diverge styling theo thời gian
- Bug fix trong Modal.tsx không tự động apply cho DeleteConfirmDialog / MoveItemDialog
- Developer mới copy pattern sai → snowball technical debt
- Inconsistency giữa các dialog (z-index, animation, ESC behavior)

## Scope

### Code refactor
- Upgrade `Modal.tsx` → M3 tokens + thêm `Modal.Header` sub-component
- Tạo `ConfirmDialog` composite — generic confirm dialog (uses Modal)
- Tạo `AlertBanner` composite — reusable error/warning/info banner
- Thêm `variant="error"` vào `Button` primitive
- Refactor `DeleteConfirmDialog` → compose từ ConfirmDialog + AlertBanner
- Refactor `MoveItemDialog` → compose từ Modal
- Dọn inline SVG spinner trong FolderTreeSelect → dùng `<Spinner>`

### Skill/Rule enforcement
- Tạo skill `component-reuse-checklist` — bắt buộc check trước khi tạo component mới
- Cập nhật `component-design.md` — thêm section "Reuse Before Create"

## Out of Scope
- Backend logic changes
- New features / APIs
- Database changes
- Redesign of existing features
