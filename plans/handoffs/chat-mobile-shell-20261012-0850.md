---
handoff-version: 1
created: 2026-10-12 08:50 UTC
branch: claude/inspiring-cray-32y4n1
base: main @ 64b75bf
plan: plans/261012-0841-chat-mobile-shell/plan.md
---

# Handoff — Chat điện thoại theo hướng Google Chat + menu ☰

## Mission and current status

**Mục tiêu:** dưới 1024px, thay vỏ điện thoại của Nexus theo hai ảnh Google Chat iOS người dùng gửi (Home, Sections):
ô tìm viên thuốc (☰ · gợi ý theo module · avatar) trên mọi màn danh sách; menu ☰ (ngăn kéo) sang Chat, Tài liệu,
Phê duyệt, Tài sản, Danh bạ, Thông báo, Quản trị, Cài đặt và đổi workspace; Chat có thanh đáy nổi **chỉ icon**
(Trang chủ · Trực tiếp · Mục · Thêm) + nút + riêng; Trang chủ là dòng hoạt động có "Người gửi: đoạn đầu" và dòng
trả lời thread mới nhất **ngay khi mở app**; Mục gập/mở có lọc chưa đọc theo mục; tìm người, nhóm và tin nhắn.

**Đã xong (thiết kế + kế hoạch, chưa có code):**
- Mockup **đã duyệt** 2026-10-12: `design/mockups/chat-mobile-home.html` (6 phần). Bản xem trực tuyến (riêng tư):
  https://claude.ai/artifact/WFk2Cbt4Piis9TKzcKSvy6
- `DESIGN.md` cập nhật: §5 "Điều hướng di động" (viết lại toàn bộ), §6 "Chat trên điện thoại" (Trang chủ, Trực tiếp,
  Mục, Thêm, Tìm), NotificationPanel trên điện thoại mở từ ☰, lối tắt điện thoại ở tab Thêm.
- Brainstorm: `plans/reports/brainstorm-261012-0827-chat-mobile-google-style.md`.
- Plan: `plans/261012-0841-chat-mobile-shell/` — phase 01 completed, 02–07 pending.
- Liên kết plan cũ: `261010-2258-pwa-mobile-push` `blockedBy` plan này (phase 06 kiểm máy thật chạy sau);
  `261010-1559-chat-inline-threads` phase 09 chỉ còn phần máy tính.

**Commits trên nhánh (chưa merge `main`):** `3d6f034` mockup + brainstorm, `a78eb54` DESIGN.md + plan, và commit
handoff này.

## Where to start

1. `git fetch origin claude/inspiring-cray-32y4n1 && git checkout claude/inspiring-cray-32y4n1` (hoặc merge vào nhánh
   local của bạn). Đọc `plan.md` rồi phase cần làm; đọc DESIGN.md §5 "Điều hướng di động" và §6 "Chat trên điện thoại";
   mở mockup trong trình duyệt (`design/mockups/chat-mobile-home.html`, cần `tokens.css` cùng thư mục).
2. Thứ tự: **02, 03, 04 song song** (02/03 backend `messaging`, 04 frontend vỏ) → **05** (cần 02 + 04) → **06** (cần
   03 + 04) → **07** kiểm chứng. 05 và 06 làm tuần tự (cùng vùng `HomeView`).
3. Mỗi phase: `ak:cook --tdd plans/261012-0841-chat-mobile-shell/phase-NN-*.md`; cập nhật `status` trong frontmatter
   phase + bảng trong `plan.md` (files-first; `ak` CLI có thể không có).

## Decisions (đừng đảo lại nếu người dùng chưa đổi)

- Thanh đáy **chuyên Chat**, **chỉ icon** (người dùng chọn). Module khác **chỉ** qua ☰ (người dùng yêu cầu: "vẫn phải
  thiết kế Hamburger để qua các màn khác"). Tab Thêm chỉ chứa lối tắt Chat, không lặp module.
- Thanh đáy chỉ hiện ở danh sách Chat; ẩn trong cuộc trò chuyện và ở mọi module khác.
- Tab Chat = search param `tab` của `/channels` (`home` | `direct` | `sections` | `more`); máy tính bỏ qua.
- Màn trong cuộc trò chuyện (luồng, thread, nhấn giữ, ô soạn) giữ nguyên `design/mockups/chat-mobile.html`.
- **Mặc định tạm** (người dùng "chốt" mà chưa trả lời; hỏi lại nếu cần): tìm tin toàn cục **làm trong plan này**;
  768–1023 **dùng vỏ điện thoại** như hiện nay.

## Evidence from scout (đã kiểm, dùng luôn)

- Xem trước hiện chỉ từ WS trong phiên: `frontend/src/stores/websocket.store.ts` (`lastMessages`, `lastReplies`) →
  `lib/conversations.ts` `buildConversations`. Vì thế cần API phase 02.
- Mẫu quyền cho truy vấn xuyên kênh: `readableChannelIDs` trong
  `backend/services/messaging/internal/domain/reactions_pins_receipts.go` (dùng bởi unread, mentions, unread threads).
  Tenant lấy từ `claims.TenantID`.
- Route hiện có cùng họ: `GET /api/channels/unread`, `/api/threads/unread`, `/api/messages/mentions` — proxy dev
  `/api/channels`, `/api/threads`, `/api/messages` đều đã trỏ 8183 (`frontend/vite.config.js`).
- `plainText(format, content)` ở `domain/richtext.go` cho văn bản thuần.
- Index: `idx_messages_channel_time (channel_id, created_at DESC)`, `idx_messages_thread_time` (migration 037).
- Tìm hiện tại: `search_vector = to_tsvector('english', content)` (migration 004) — sai cho tiếng Việt, lẫn thẻ HTML;
  phase 03 đổi sang `'simple'` + bỏ thẻ, và **sửa luôn** tìm trong một nhóm. Migration cuối hiện là `049_*`.
- Không có API thread đang theo dõi xuyên kênh → phase 02 thêm `GET /api/threads/followed`.
- Vỏ hiện tại: `components/patterns/MobileNav.tsx` (thanh đáy module), `MobileTopBar.tsx`, `MobileMoreSheet.tsx`;
  danh sách điện thoại `components/spaces/PhoneChatList.tsx`, gắn trong `HomeView.tsx`; breakpoint
  `hooks/usePhone.ts` `useBottomBarLayout` (< 1024). Điểm đến sidebar: `AppSidebar.tsx` `mainNavItems`/`footNavItems`
  (cá nhân ẩn `approval`, `assets`) → phase 04 tách ra `lib/nav-destinations.ts`.

## Scope and guardrails

- Tuân `CLAUDE.md` §4–§5: TDD; **test deny** (người không thành viên / bị gỡ / DM người khác không thấy xem trước,
  thread theo dõi, kết quả tìm); đổi `.proto` → `make proto` **và** `cd frontend && npm run proto:gen`; route mới phải
  có trong proxy dev; **không ID/UUID lên UI**; migration: **backup trước**, `make db-migrate`, kiểm trên DB thật;
  cập nhật spec (`chat-activity-feed` mới, `chat-search` mới, `chat-threads`, `lark-sidebar-layout`, `chat-shortcuts`,
  `chat-sections`) và `docs/specs/README.md`.
- UI phải khớp DESIGN.md + mockup; không tự thiết kế thêm trong code.
- Không đặt mã plan/phase vào code, tên test, commit message.
- Không commit `.agentkit/` (trả phí, repo public), không commit secret (`.env.dev` gitignore; khoá OneSignal chỉ ở env).
- OTP cố định `999999` là quyết định của người dùng — giữ.
- Go không có sẵn trên PATH: `export PATH="/usr/local/go/bin:$HOME/go/bin:$PATH"`; chạy Go trong thư mục service
  (`make test s=messaging`), không `go test ./...` ở `backend/`.

## Verification (phase 07 tóm tắt)

`make build-check`, `make check-layering`, `make check-ngac`, `make test s=messaging`; `cd frontend && npm test`,
`npm run lint`, typecheck, `npm run build`. Trên `make dev`: 2 người, nhóm + thread, gỡ một người → 3 endpoint mới không
còn nhóm đó. Playwright 390×844 sáng/tối mọi màn của mockup → `plans/reports/chat-mobile-shell-screens/`; màn ≥ 1024
không đổi. Rồi `ak:code-review`, sync trạng thái plan.

## Risks

- Đổi generated column `search_vector` ghi lại cả bảng `messages` (khoá ngắn) — ghi vào `docs/deployment.md`.
- Tìm không dấu ("doi soat") chưa có ở server; client có cho tên người/nhóm.
- Xoá `MobileMoreSheet`/`MobileTopBar` có thể làm rơi luồng đổi workspace, đăng xuất, thông báo → test từng luồng
  trước khi xoá.

## Open questions (người dùng)

- Tìm tin toàn cục làm ngay hay để sau? (mặc định: làm — bỏ phase 03 + 06 nếu để sau)
- Máy tính bảng 768–1023 dùng vỏ điện thoại hay chờ rail? (mặc định: vỏ điện thoại)
- Việc của người dùng còn treo từ trước: tạo app OneSignal (App ID + REST key vào config server), xoay vòng Google
  client secret.

## Prompt để chạy ở máy local

```text
Đọc plans/handoffs/chat-mobile-shell-20261012-0850.md rồi plans/261012-0841-chat-mobile-shell/plan.md.
Làm theo plan bằng /ak:cook --tdd, bắt đầu phase 02, 03, 04 (song song được), rồi 05, 06, 07.
Mọi UI khớp DESIGN.md §5 "Điều hướng di động", §6 "Chat trên điện thoại" và design/mockups/chat-mobile-home.html.
Tuân CLAUDE.md §5 (test deny, proto hai phía, proxy dev, không ID trên UI, migration có backup + make db-migrate).
Cập nhật status từng phase trong file plan khi xong; commit theo phase.
```
