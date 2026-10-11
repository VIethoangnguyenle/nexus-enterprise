---
title: "Personal workspaces: no assets, no approvals"
description: "Give workspaces a trustworthy personal/organization type, hide and refuse Assets and Approvals in personal workspaces, and let owners convert a personal workspace to an organization."
status: pending
priority: P2
effort: 5-6d
issue:
branch: claude/inspiring-cray-32y4n1
tags: [feature, frontend, backend, authorization, workspace]
blockedBy: []
blocks: []
created: 2026-10-10
---

# Workspace cá nhân: không có Tài sản và Phê duyệt

Nguồn: [brainstorm](../reports/brainstorm-261010-1559-personal-workspace-features.md) (đã chấp nhận). Làm **sau
Chat** (người dùng: Chat quan trọng nhất).

## Mục tiêu (contract)
- **Outcome:** Ở workspace cá nhân, Tài sản và Phê duyệt (gồm duyệt tài liệu) không xuất hiện ở đâu và server từ
  chối mọi thao tác của chúng; workspace tổ chức không đổi. Chủ workspace chuyển cá nhân → tổ chức trong Cài đặt.
- **Constraints:** loại đúng cho workspace mới và cũ; TDD + test deny; migration có backup, áp và kiểm chứng;
  proto đổi thì `make proto` + `npm run proto:gen`; không UUID trên UI.
- **Non-goals:** đổi tính năng bên trong Tài sản/Phê duyệt; xoá dữ liệu có sẵn (chỉ ẩn); chuyển tổ chức → cá nhân.
- **Acceptance:** workspace cá nhân — không có mục ở sidebar/mobile/màn khác; `/assets`, `/approval` chuyển về
  trang chính; API asset/approval trả 403; chuyển sang tổ chức thì các mục xuất hiện và dùng được ngay.

## Quyết định (người dùng chốt)
1. Mỗi tài khoản mới có một workspace cá nhân riêng, kể cả email công ty (Google `hd`, vd vnpay.vn). Đăng
   nhập không tự tạo tổ chức; nếu đã có tổ chức giữ tên miền đó thì tự vào thêm làm thành viên. Muốn có
   workspace khác thì dùng "Tạo workspace", **chọn loại Cá nhân hoặc Tổ chức** (một người có thể có nhiều
   workspace cá nhân). (Chốt lại 2026-10-11, thay cho "Google theo tên miền = tổ chức".)
   Migration workspace cũ: tên dạng "<tên>'s Workspace" + 1 thành viên → cá nhân, còn lại → tổ chức.
2. Chủ workspace chuyển cá nhân → tổ chức trong Cài đặt (một chiều).
3. Mobile: ở workspace cá nhân, tab Phê duyệt thay bằng **Danh bạ**.
4. Duyệt tài liệu: ẩn ở workspace cá nhân.
5. (2026-10-11, câu hỏi thiết kế) Chỉ Chủ sở hữu được chuyển sang tổ chức; hộp xác nhận chỉ đổi loại (không ô
   đổi tên) và nói rõ tài sản/yêu cầu phê duyệt đã ẩn sẽ hiện lại; nhãn dưới tên là "Workspace cá nhân".
   Màn "Tạo workspace" cần thêm lựa chọn loại — cần bổ sung vào mockup trước phase 04.
6. (2026-10-11) Mockup `personal-workspace.html` §6/§8/§9 **đã duyệt**; "Tạo workspace" mặc định chọn **Cá nhân**.
   Workspace tổ chức tạo mới luôn **độc lập** (không gắn tên miền, vào bằng lời mời). Workspace cá nhân tự tạo lúc
   đăng ký **mang tên người dùng** (tên hiển thị), không còn "<tên>'s Workspace" / "Không gian của bạn".

## Phases

| # | Phase | Priority | Effort | Depends on | Status |
|---|---|---|---|---|---|
| 01 | [Thiết kế](phase-01-design.md) | P2 | 0.5d | — | completed (mockup duyệt 2026-10-11) |
| 02 | [Loại workspace đáng tin](phase-02-workspace-type.md) | P1 | 1d | 01 | completed |
| 03 | [Chặn ở server](phase-03-server-enforcement.md) | P1 | 1.5d | 02 | completed |
| 04 | [Ẩn ở frontend](phase-04-frontend.md) | P2 | 1d | 02 | pending |
| 05 | [Chuyển sang tổ chức](phase-05-convert-to-organization.md) | P2 | 1-1.5d | 03, 04 | in-progress (backend xong) |
| 06 | [Kiểm chứng](phase-06-verification.md) | P2 | 0.5d | 05 | pending |

## Capabilities in `docs/specs/`
| Capability | Action | Phase |
|---|---|---|
| `tenant-identity` | modify — workspace type, assignment at creation, conversion | 02, 05 |
| `asset-authorization` | modify — personal workspaces refuse asset operations | 03 |
| `approval-screens` | modify — hidden in personal workspaces | 04 |
| `workspace-type` | **new** (hoặc gộp vào `tenant-identity`) — rule list across services | 02–05 |

## Risks
- Chuyển sang tổ chức có thể cần khởi tạo thứ workspace tổ chức có mà cá nhân thiếu (schema phê duyệt theo
  tenant, OA tài sản) → scout `tenant-ngac-init` ở phase 05; mọi ghi đồ thị qua EPP.
- Heuristic migration sai với workspace đã đổi tên → báo số workspace mỗi loại trước/sau, cho quản trị sửa tay.

## Unresolved questions
- Không còn. Mời thêm người vào workspace cá nhân: giữ hành vi hiện tại (người dùng đồng ý mặc định 2026-10-10).
