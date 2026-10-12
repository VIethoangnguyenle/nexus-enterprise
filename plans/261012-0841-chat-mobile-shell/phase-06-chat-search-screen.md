---
phase: 6
title: "Màn Tìm trong chat"
status: pending
priority: P1
effort: "1d"
dependencies: [3, 4]
---

# Phase 06: Màn Tìm trong chat

## Overview
Ô tìm ở Chat mở màn tìm toàn màn (DESIGN.md §6 "Tìm trong chat", mockup §6).

## Requirements
- Ô nhập focus ngay (bàn phím mở), Quay lại, Xoá; chip Tất cả · Người · Nhóm · Tin nhắn · Có tệp.
- Người và nhóm: lọc phía client trên `useConversations` + `usePeople`, không phân biệt hoa thường và dấu
  (`normalize('NFD')` bỏ dấu, đ→d). Chạm: người → mở/tạo DM như `StartChatDialog`; nhóm → cuộc trò chuyện.
- Tin nhắn: `searchAll` (phase 03), debounce 250ms, từ 2 ký tự, 25 mỗi trang + "Tải thêm"; hàng: avatar, người gửi ·
  tên cuộc trò chuyện, đoạn có từ khớp tô `--color-warning-wash`, giờ. Chạm → cuộc trò chuyện, tin vào giữa và loé
  (đường `openTarget` của `lib/shortcut-feed.ts`, thread nếu là trả lời).
- Khi chưa gõ: "Tìm gần đây" (tối đa 8, localStorage theo user, try/catch; xoá từng mục).
- Trạng thái: đang tìm (Skeleton), không có kết quả ("Không tìm thấy … — thử từ khác"), lỗi + Thử lại.
- Modal: focus trap, Esc/Quay lại đóng và trả focus về ô tìm; giảm chuyển động = không trượt.

## Related Code Files
- Create: `frontend/src/components/spaces/ChatSearchScreen.tsx`, `hooks/useMessageSearch.ts`, `lib/search-text.ts`
  (bỏ dấu + tô khớp) — + test
- Modify: `components/patterns/PhoneSearchBar.tsx` (Chat mở màn này thay cho fallback)

## Success Criteria
- [ ] "doi soat" tìm ra nhóm "Đối soát giao dịch" (client); "đối soát" ra tin (server).
- [ ] Kết quả không có tin của nhóm đã bị gỡ (dựa API phase 03; kiểm trên stack thật ở phase 07).
- [ ] Gate frontend xanh.

## Risk Assessment
- Tìm tin không dấu chưa hỗ trợ ở server (phase 03) → chữ gợi ý trong trạng thái rỗng: "Thử gõ có dấu".
