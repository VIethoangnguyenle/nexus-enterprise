---
phase: 6
title: "UI redesign and motion"
status: in-progress
priority: P1
effort: 6-8d
dependencies: [4]
---

# Phase 06 — UI redesign and motion  ★ trọng tâm của plan

## Overview
Ưu tiên số một của người dùng: **refactor UI cho đẹp, kể cả animation**. Người dùng đã đồng ý
làm **một bộ UI hoàn toàn mới qua `ak:brainstorm`** thay vì bám thiết kế Stitch hiện có.

- Brainstorm: [`reports/brainstorm-261009-0457-new-ui-direction.html`](reports/brainstorm-261009-0457-new-ui-direction.html)
  — 3 hướng (A Sổ cái, B Tín hiệu, C Ca trực), mockup có chú thích, demo motion. Đề xuất: **B**,
  mượn bảng kẻ mảnh của A cho Tài liệu và Quản trị.
- Khi người dùng chọn hướng: viết lại `DESIGN.md` theo hướng đó, đổi dòng "Design source of truth
  is Stitch" trong CLAUDE.md §3 sang `DESIGN.md` mới + mockup trong repo, rồi mới code.
- Thiết kế (token, font, primitive, motion) **không phụ thuộc backend**, bắt đầu ngay song song
  phase 02–03; phần code màn hình chờ phase 04.

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

### A. Thiết kế (ak:brainstorm → DESIGN.md)
- Người dùng chọn hướng trong file brainstorm.
- Viết lại `DESIGN.md` theo hướng đã chọn (token OKLCH, font Fontsource có subset tiếng Việt,
  thang cỡ chữ, spacing 4pt, radius, chiều sâu, motion); `.stitch/` giữ làm lịch sử.
- Mockup có chú thích cho mọi nhóm màn (shell, chat, drive, approval, assets, admin, contacts,
  documents, settings, auth) theo `ak:frontend-design`, duyệt từng nhóm trước khi code.
- Nội dung `DESIGN.md` phải có: thang z-index, thang icon/màu loại file trong token, quy tắc
  avatar, mật độ bảng, trạng thái loading/empty/error chuẩn, **motion spec đầy đủ** (dưới).

### B. Motion system
- Một module `frontend/src/lib/motion.ts` + utilities CSS: enter/exit cho modal, panel, popover,
  toast, dropdown; stagger danh sách; chuyển cảnh route; hiệu ứng hàng mới/xoá do realtime
  (phase 05); skeleton shimmer duy nhất.
- Exit animation thật (giữ mount tới khi xong), chạy ở ~75% thời gian mở. Đề xuất: `motion`
  cho presence/layout animation, CSS cho phần còn lại (Unresolved Q2 trong plan.md).
- Hướng B: realtime mang danh tính, nghĩa là avatar loé và hàng phủ màu của tác giả rồi tan trong
  2,4s; gộp ≥ 3 thay đổi trong 2s thành "X và N người khác"; giảm chuyển động thì chỉ còn lớp
  phủ màu.
- Chỉ animate `transform`/`opacity`; bỏ `transition-all`; mọi duration/easing từ token.
- `prefers-reduced-motion`: tắt chuyển động, giữ fade ngắn.
- Lint: cấm `transition-all`, `duration-<số>` thô, `animate-bounce`.

### C. Code
- Mỗi màn render đúng mockup đã duyệt; dùng primitive (`Heading`, `Text`, `Avatar`, `Modal`,
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
- Responsive: mockup tablet/mobile cho từng nhóm màn.

## Order of screens
Theo mức dùng và mức lệch: (1) shell + sidebar + nav, (2) chat, (3) drive, (4) approval,
(5) assets, (6) admin, (7) contacts, documents, settings, (8) auth/onboarding.

## Related files
- modify `DESIGN.md` (viết lại), `CLAUDE.md` §3 (nguồn thiết kế); `.stitch/` giữ làm lịch sử
- create `frontend/src/assets/fonts` qua `@fontsource/*` (subset `vietnamese`)
- modify `frontend/src/index.css`, `frontend/eslint.config.js`, `frontend/src/components/**`,
  `frontend/src/routes/**`
- create `frontend/src/lib/{motion,format}.ts`, `frontend/src/components/composites/{UserPicker,RolePicker,DepartmentPicker}.tsx`
- backend display-name fields: approval, drive, asset REST
- modify `docs/specs/drive-context-panel/spec.md`, `docs/specs/lark-sidebar-layout/spec.md` (nếu shell đổi);
  create spec layout cho assets/admin nếu thêm màn

## Implementation steps
1. [x] Người dùng chọn hướng B Tín hiệu (2026-10-09). CLAUDE.md §3 trỏ sang `DESIGN.md`; `.stitch/` lưu trữ.
2. [~] `DESIGN.md` mới (xong) + mockup: màn cốt lõi (Tin nhắn, Tài liệu, Phê duyệt, trạng thái,
   di động) ở `design/mockups/core-screens.html` chờ duyệt; còn Tài sản, Quản trị, Danh bạ,
   Tài liệu văn bản, Cài đặt, Auth.
3. [~] Nền tảng: tokens + type + `lib/motion` (`motion/react`, preset theo DESIGN.md §7, reduced-motion)
   + `lib/format` đã có (PR #2, `d0e8bac`); Dialog/Popover/Toast đã có exit. Còn: primitive thiếu, lint
   siết (warn trong lúc chuyển, khoá error khi xong), 32 `transition-all` ở các màn chưa làm.
   Nhóm (1) shell + sidebar và (2) chat/spaces **đã code** trong PR #2.
4. [~] Từng nhóm màn theo thứ tự trên, mỗi nhóm một PR **gộp với 04b của domain đó** (xem plan.md). Xong: (1) shell, (2) chat (PR #2), (3) drive (2026-10-10, `reports/phase-06-drive-report.md`). Tiếp: approval.
   code theo mockup → test vitest (không UUID,
   có loading/empty/error) → screenshot 3 khổ × 2 theme so với mockup.
5. Motion pass toàn app + reduced-motion; test thủ công 60fps trên danh sách dài (drive, chat).
6. Khoá lint ở `error`.

## Success criteria
- [ ] Mọi route có mockup đã duyệt và screenshot khớp, ở 3 khổ × 2 theme.
- [ ] Không còn ID trên màn hình (test regex UUID), không còn ô nhập ID.
- [ ] Mọi overlay có enter + exit; tôn trọng `prefers-reduced-motion`.
- [ ] Không còn `transition-all`, duration/easing thô, z-index tuỳ ý.
- [ ] Lint design-system ở `error`, `eslint-disable` chỉ còn exemption có lý do.

## Risks
- Hướng B nhiễu khi nhiều người sửa cùng lúc → gộp theo cửa sổ 2s, tối đa một lớp phủ mỗi hàng.
- Thêm thư viện animation tăng bundle (~30–40KB gz cho `motion`); đo trước/sau.
- Redesign lớn dễ trượt phạm vi → mỗi màn một PR, duyệt thiết kế trước khi code.
