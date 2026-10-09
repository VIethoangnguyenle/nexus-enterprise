---
phase: 6
title: "UI redesign and motion"
status: pending
priority: P1
effort: 6-8d
dependencies: [4]
---

# Phase 06 — UI redesign and motion  ★ trọng tâm của plan

## Overview
Ưu tiên số một của người dùng: **refactor UI cho đẹp, kể cả animation**. Đây là thiết kế lại có
chủ đích, không chỉ dọn code. Quy trình vẫn là Stitch-first (CLAUDE.md §3, `.stitch/WORKFLOW.md`):
thiết kế trên Stitch → duyệt → code render đúng thiết kế. Việc thiết kế (bước 1–3) **không phụ
thuộc backend** nên bắt đầu ngay, song song với phase 02–03; phần code (bước 4+) chờ phase 04.

> Session hiện tại không kết nối Stitch MCP. Bước thiết kế cần Stitch MCP (hoặc người dùng thiết
> kế trên Stitch rồi đưa screen ID).

## Key insights
**Độ phủ thiết kế.** Stitch project `14852434379132121789` có 24 screen (chat, contacts, drive,
approval, login/onboarding/welcome/verification, workspace-selection, tablet/mobile variants).
**Chưa có screen** cho: `assets/*` (dashboard, list, detail, requests, request/new, types),
`admin/*` (index, roles, users), `documents`, `settings`, `register`. Những màn này đang "thiết kế
trong code" — chính là chỗ xấu và lệch nhất.

**Lệch design system** (`.stitch/DESIGN.md`, `DESIGN.md`):
- Typography: 188 class `text-xs/sm/lg…` thô vs 195 token; 31 `<h1-6>` thô vs 10 `<Heading>`;
  51 `<p className>` vs 41 `<Text>`. Cỡ chữ lẻ `text-[10.5px]` ×8, `text-[12.5px]` ×4 lọt lint.
- Màu: `bg-amber-500/10` thô (`routes/_workspace/drive.tsx:281`); 45 hex màu loại file
  (`fileIcons.ts`) và màu ghép alpha `${fi.color}10` (`DriveFileRow.tsx:102`); `rgba` ở `AuthLayout.tsx:7`.
- Layer: `z-[9999]`, `z-[200]` thay vì thang z-index. `tracking-[0.08em]` ×8. 20 `style={{}}`.
- 77 `eslint-disable` né rule design-system; Rules of Hooks không bật cho TS.
- Trùng lặp làm UI không nhất quán: ~11 avatar initials tự vẽ cạnh `Avatar`; 4 tree renderer
  (`paddingLeft: 12+depth*20`); 2 dialog xoá; share UI 2 bản; header bảng drive copy; format ngày
  ~14 file, byte 5 file.
- `/assets` có shell riêng (`routes/assets.tsx`) → nav, header, logout khác phần còn lại.

**Motion** (tokens có sẵn ở `frontend/src/index.css:162-177`, DESIGN.md §7):
- Không có `prefers-reduced-motion` ở bất kỳ đâu.
- 42 `transition-all` (animate cả layout → giật, tốn), 22 `duration-150` + 1 `duration-700` thô
  thay vì `--duration-*`.
- Chỉ có animation vào, **không có animation ra**: DESIGN.md quy định đóng panel 150ms ease-in,
  slide-down 150ms — code unmount ngay.
- Modal dùng `scale-in` 0.95 + `--ease-spring`; DESIGN.md quy định 0.97→1, 200ms ease-out.
  Spring chỉ được phép cho `reaction-pop` (`.impeccable/config.json`).
- `animate-bounce` ×3, `animate-pulse` ×9 làm skeleton không thống nhất.
- Không có chuyển cảnh route, không có hiệu ứng khi danh sách đổi do realtime (hàng mới chen
  vào/biến mất đột ngột).

**Quy tắc cứng đang bị vi phạm** (CLAUDE.md §5 — phải sửa khi đụng màn):
- ID lên màn hình: `ApprovalDetailPanel.tsx:74-75`, `TemplateDetailPanel.tsx:27`,
  `DriveContextPanel.tsx:209`, `DriveFileRow.tsx:160`, `assets/$assetId.tsx:66,70`,
  `assets/list.tsx:80,149,153`; `admin/roles.tsx:98,108` tự cắt prefix "uuid_Owners".
- Nhập ID tay: `StepBuilder.tsx:97` ("UA / User ID"). ID bịa: `CreateRequestModal.tsx:57`.
- Audit không có actor: `ApprovalDetailPanel.tsx:40,115`, lịch sử asset `assets/$assetId.tsx:28`.

**a11y / state:** phần tử click không phải button (`ApprovalTable.tsx:63`, `NotificationBell.tsx:68`,
`ContactCard.tsx:15`, `DataTable.tsx:57`, `DriveFileRow.tsx:116`…); `Modal` không focus trap;
thiếu error state ở workspace-select, assets, admin/users, channels.index, contacts, AppSidebar;
thiếu empty state ở `channels.$channelId.tsx`, `assets/dashboard.tsx`; 2 họ component
loading/error/empty.

## Requirements

### A. Thiết kế (Stitch)
- Đánh giá hiện trạng từng màn: chụp screenshot app thật (Playwright) cạnh screen Stitch, chấm
  bằng `impeccable` (critique/audit) + `ak:ui-ux-pro-max`; ra danh sách vấn đề thị giác theo mức độ.
- Thiết kế mới trên Stitch cho 5 nhóm màn chưa có screen (assets, admin, documents, settings,
  register) và cập nhật screen hiện có ở chỗ critique chỉ ra.
- Bổ sung vào `.stitch/DESIGN.md`: thang z-index, thang icon/màu loại file trong token, quy tắc
  avatar, mật độ bảng, trạng thái loading/empty/error chuẩn, **motion spec đầy đủ** (dưới).
- Người dùng duyệt từng nhóm màn trước khi code.

### B. Motion system
- Một module `frontend/src/lib/motion.ts` + utilities CSS: enter/exit cho modal, panel, popover,
  toast, dropdown; stagger danh sách; chuyển cảnh route; hiệu ứng hàng mới/xoá do realtime
  (phase 05); skeleton shimmer duy nhất.
- Exit animation thật (giữ mount tới khi xong). Đề xuất: `motion` (Framer Motion) cho
  presence/layout animation, CSS cho phần còn lại — chốt ở Unresolved Q2 trong plan.md.
- Chỉ animate `transform`/`opacity`; bỏ `transition-all`; mọi duration/easing từ token.
- `prefers-reduced-motion`: tắt chuyển động, giữ fade ngắn.
- Lint: cấm `transition-all`, `duration-<số>` thô, `animate-bounce`.

### C. Code
- Mỗi màn render đúng Stitch; dùng primitive (`Heading`, `Text`, `Avatar`, `Modal`,
  `ConfirmDialog`, một `TreeView`, `lib/format`).
- Gộp `/assets` vào shell `_workspace` (hoặc dùng chung guard/WS/nav — Unresolved Q5).
- Picker user/role/department thay mọi ô nhập ID; backend trả display name/avatar cho
  owner/actor/approver/assignee (endpoint mới → `frontend/vite.config.js`, cả khối regex nếu dưới
  `/api/workspaces/:id/`).
- Audit entry = actor (tên + avatar) + nhãn action đọc được + thời gian.
- Modal focus trap + focus return; mọi phần tử tương tác dùng `button`/role + bàn phím.
- Một bộ Loading/Empty/Error, có ở mọi query.
- Siết lint: px thập phân, `z-[`, `tracking-[`, `rgba`, motion rules; bật `react-hooks`;
  giảm `eslint-disable` về chỉ còn exemption có lý do.
- Responsive theo screen tablet/mobile đã có trên Stitch.

## Order of screens
Theo mức dùng và mức lệch: (1) shell + sidebar + nav, (2) chat, (3) drive, (4) approval,
(5) assets, (6) admin, (7) contacts, documents, settings, (8) auth/onboarding.

## Related files
- modify `.stitch/DESIGN.md`, `DESIGN.md`, `.stitch/metadata.json` (screen mới), `.stitch/designs/`
- modify `frontend/src/index.css`, `frontend/eslint.config.js`, `frontend/src/components/**`,
  `frontend/src/routes/**`
- create `frontend/src/lib/{motion,format}.ts`, `frontend/src/components/composites/{UserPicker,RolePicker,DepartmentPicker}.tsx`
- backend display-name fields: approval, drive, asset REST
- modify `docs/specs/drive-context-panel/spec.md`, `docs/specs/lark-sidebar-layout/spec.md` (nếu shell đổi);
  create spec layout cho assets/admin nếu thêm màn

## Implementation steps
1. Chạy app (`make dev`), chụp toàn bộ màn (desktop/tablet/mobile) → báo cáo critique.
2. Thiết kế trên Stitch: màn thiếu + sửa theo critique + motion spec → người dùng duyệt.
3. Nền tảng: tokens bổ sung, `lib/motion`, `lib/format`, primitive còn thiếu, lint siết (ở mức warn
   trong lúc chuyển, khoá error khi xong).
4. Từng màn theo thứ tự trên, mỗi màn một PR: fetch Stitch HTML → code → test vitest (không UUID,
   có loading/empty/error) → screenshot so với Stitch.
5. Motion pass toàn app + reduced-motion; test thủ công 60fps trên danh sách dài (drive, chat).
6. Khoá lint ở `error`.

## Success criteria
- [ ] Mọi route có screen Stitch tương ứng và screenshot khớp.
- [ ] Không còn ID trên màn hình (test regex UUID), không còn ô nhập ID.
- [ ] Mọi overlay có enter + exit; tôn trọng `prefers-reduced-motion`.
- [ ] Không còn `transition-all`, duration/easing thô, z-index tuỳ ý.
- [ ] Lint design-system ở `error`, `eslint-disable` chỉ còn exemption có lý do.

## Risks
- Không có Stitch MCP → thiết kế bị chặn; fallback: người dùng thiết kế trên Stitch rồi đưa screen ID.
- Thêm thư viện animation tăng bundle (~30–40KB gz cho `motion`); đo trước/sau.
- Redesign lớn dễ trượt phạm vi → mỗi màn một PR, duyệt thiết kế trước khi code.
