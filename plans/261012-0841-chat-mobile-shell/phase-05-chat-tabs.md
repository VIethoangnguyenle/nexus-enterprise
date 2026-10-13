---
phase: 5
title: "Tab Trang chủ, Trực tiếp, Mục, Thêm"
status: completed
priority: P0
effort: "2d"
dependencies: [2, 4]
---

# Phase 05: Bốn tab Chat trên điện thoại

## Overview
`HomeView` trên điện thoại hiện gắn `PhoneChatList` (một danh sách dài). Thay bằng 4 tab theo `?tab=` (DESIGN.md §6
"Chat trên điện thoại", mockup §1–§4). Máy tính không đổi.

## Requirements
- **Dữ liệu:** `useConversations` lấy thêm activity (phase 02); `buildConversations` gộp: xem trước/giờ lấy cái mới hơn
  giữa server và WS (`lastMessages`, `lastReplies`), thêm `reply?: { author, text, at, parentId, mentionsMe }` khi trả
  lời mới hơn luồng chính. Thứ tự Trang chủ theo `lastActivity` (cả trả lời). WS reconnect → invalidate activity.
  `kind` → chữ: tệp "📎 tên tệp", "Bình chọn: …", "Công việc: …".
- **FeedRow** (`components/spaces/FeedRow.tsx`): đúng spec DESIGN (avatar 44 nhóm/người + chấm hoạt động, huy hiệu
  thread, tên + giờ tương đối, dòng chính 1–2 dòng, dòng thread có nét cong, chip @tên — khớp tên hiển thị của người
  trong `mentions` qua `usePeople`, "Bạn:", chưa đọc/tắt thông báo, pill số hoặc "@"). Chạm hàng → cuộc trò chuyện;
  chạm dòng thread → cuộc trò chuyện mở thread đó (search param thread hiện có của `channels.$channelId`). Nhấn giữ →
  sheet hàng hiện có + "Đánh dấu đã đọc" (`markRead`).
- **Trang chủ:** tiêu đề + nút Thread (số `UnreadThreads`; mở màn Thread toàn màn với chip Có tin mới / Đang theo dõi,
  dùng `/threads/unread` và `/threads/followed`) + nút chỉ chưa đọc (localStorage theo user, try/catch; trạng thái rỗng
  "Bạn đã đọc hết"). Kéo xuống tải lại (invalidate conversations + activity) — tự làm ngắn gọn vì PWA standalone không
  có pull-to-refresh của trình duyệt.
- **Trực tiếp:** chỉ DM, cùng FeedRow, nút chỉ chưa đọc riêng.
- **Mục:** tách phần mục của `PhoneChatList` thành `PhoneSectionsList`: thứ tự Đã gắn sao · mục của bạn · Tin nhắn trực
  tiếp · Nhóm; tiêu đề gập/mở (dùng `nav-collapse.store` chung với máy tính) + nút lọc chưa đọc của riêng mục (state
  cục bộ); hàng 56 avatar 36; nhấn giữ tiêu đề mục của bạn → Đổi tên / Lên / Xuống / Xoá (dùng overlay mục hiện có);
  "Mục mới" cuối. Bỏ hai ô lối tắt đầu danh sách và `UnreadThreadsSection` khỏi điện thoại (đã chuyển sang Thêm / Thread).
- **Thêm:** Được nhắc tới (pill số) → `channels.mentions`; Tin đã gắn sao → `channels.starred`; Thread đang theo dõi
  ("N có tin mới") → màn Thread chip Đang theo dõi; Duyệt nhóm; Thông báo của Chat → Cài đặt › Thông báo.
- Không ID trên màn; mọi chữ tiếng Việt theo DESIGN.md.

## Related Code Files
- Modify: `frontend/src/lib/conversations.ts` (+ test), `hooks/useConversations.ts`, `components/spaces/HomeView.tsx`
  (+ `HomeView.phone.test.tsx`), `PhoneChatList.tsx` → tách/đổi tên (+ test)
- Create: `components/spaces/FeedRow.tsx`, `PhoneHomeFeed.tsx`, `PhoneSectionsList.tsx`, `PhoneMoreList.tsx`,
  `PhoneThreadsScreen.tsx`, `hooks/usePullToRefresh.ts` (+ test mỗi file)
- Modify: `docs/specs/chat-shortcuts/spec.md`, `docs/specs/chat-sections/spec.md`; spec `chat-activity-feed` thêm scenario UI

## Implementation Steps
1. Test `buildConversations` gộp server + WS (server mới hơn, WS mới hơn, chỉ có trả lời, tin xoá/không có).
2. Test FeedRow (chưa đọc, tắt thông báo, chip @, dòng thread, chạm dòng thread mở thread, nhấn giữ).
3. Test từng tab (lọc chưa đọc + nhớ, gập mục, lọc theo mục, Thêm điều hướng đúng).
4. Làm, rồi gate frontend; ảnh Playwright 390 sáng/tối so với mockup.

## Success Criteria
- [x] Mở app lạnh (không WS event nào) vẫn thấy xem trước và dòng thread.
- [x] Bốn tab khớp mockup (theo CSS/markup mockup; ảnh Playwright 390 sáng/tối để phase kiểm chứng chụp); máy tính không đổi (test HomeView máy tính xanh, không sửa kỳ vọng; chỉ xoá test menu "Trò chuyện mới" ở đầu Trang chủ vì menu đó đã bỏ).
- [x] Gate frontend xanh.

## Risk Assessment
- Chip @ dựa trên khớp tên hiển thị trong văn bản thuần — tên trùng một phần có thể tô sai; chỉ tô khi khớp đúng
  "@<tên hiển thị>" của người có trong `mentions`.
- `PhoneChatList` đang có nhiều test; tách theo hướng giữ test cũ cho phần mục, xoá test của phần bị bỏ (lối tắt) có
  ghi lý do trong commit.
