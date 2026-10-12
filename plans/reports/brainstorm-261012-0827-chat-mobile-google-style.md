---
type: brainstorm
date: 2026-10-12
topic: Chat điện thoại theo hướng Google Chat
status: mockup chờ duyệt
mockup: design/mockups/chat-mobile-home.html
---

# Brainstorm — vỏ Chat điện thoại theo hướng Google Chat

## Tóm tắt
Người dùng gửi 2 ảnh Google Chat iOS (Home, Sections) và yêu cầu thiết kế lại chat mobile theo hướng đó. Chốt:
thanh đáy **chuyên Chat** (Trang chủ · Trực tiếp · Mục · Thêm), **chỉ icon**, nút + tách riêng; **☰ phải đưa được
sang các màn khác** (người dùng nhắc lại). Mockup: `design/mockups/chat-mobile-home.html`.

## Contract
- **Outcome:** ở 390px, Chat có ô tìm viên thuốc (☰ · Tìm · avatar); Trang chủ = dòng hoạt động mọi cuộc trò chuyện
  có "Người gửi: đoạn đầu" + dòng trả lời thread mới nhất; tab Trực tiếp; tab Mục gập/mở, lọc chưa đọc theo mục;
  tab Thêm (lối tắt chat); thanh đáy nổi chỉ icon + nút +; ngăn kéo ☰ sang mọi module + đổi workspace.
- **Constraints:** design-first (mockup + DESIGN.md trước code); không ID trên UI; xem trước/tìm tin chỉ trả cho
  cuộc trò chuyện còn quyền đọc (kiểm trên OA kênh, test deny); giữ nguyên màn trong cuộc trò chuyện
  (chat-mobile.html: luồng, thread, nhấn giữ, ô soạn); workspace cá nhân không Tài sản/Phê duyệt/Quản trị.
- **Non-goals:** bố cục máy tính; màn trong cuộc trò chuyện; app native.
- **Acceptance:** mockup duyệt; Trang chủ có xem trước ngay khi mở app (không phụ thuộc WS trong phiên); không cuộn
  ngang 390px; thanh đáy không che hàng cuối; ☰ có trên mọi màn danh sách của mọi module; test deny xem trước khi bị
  gỡ khỏi nhóm.

## Hiện trạng (scout)
- `MobileNav.tsx`: thanh đáy theo module (Tin nhắn · Tài liệu · Phê duyệt · Thêm), có nhãn, dính đáy.
- `MobileTopBar.tsx`: "Nexus · workspace" + nút tìm (đi tới ô tìm của màn qua `[data-module-search]`).
- `PhoneChatList.tsx`: một danh sách dài (lối tắt, mục, DM, Nhóm); chưa có Trang chủ dạng dòng hoạt động.
- `MobileMoreSheet.tsx`: Tài sản · Danh bạ · Quản trị · Cài đặt + đổi workspace.
- **Khoảng trống dữ liệu:** xem trước chỉ từ `websocket.store.ts` `lastMessages`/`lastReplies` (tin thấy trong phiên).
  Không có API trả tin mới nhất / trả lời thread mới nhất theo kênh. Tìm tin chỉ có `GET /api/channels/:id/search`.

## Phương án
- **A (chọn):** thanh đáy chuyên Chat; module khác qua ☰. Giống ảnh nhất; Phê duyệt xa hơn 1 chạm → bù bằng chấm
  trên ☰ + số "2 chờ bạn" trong ngăn kéo.
- **B:** giữ thanh module, chip Trang chủ/Trực tiếp/Mục trong tab Tin nhắn. Ít xáo trộn, không giống hướng yêu cầu.

## Quyết định thiết kế (trong mockup)
- Ô tìm viên thuốc là thanh trên chung của **mọi màn danh sách mọi module**, chữ gợi ý theo module; chạm → ô tìm của
  màn đó (Chat: màn tìm toàn màn). Thay `MobileTopBar`.
- Thanh đáy Chat chỉ hiện trong Chat; module khác không có thanh đáy; về Chat qua ☰ → Chat.
- Ngăn kéo: Nexus · khối workspace (mở danh sách workspace tại chỗ, Tạo workspace) · Chat · Tài liệu · Phê duyệt
  (N chờ bạn) · Tài sản · Danh bạ | Thông báo · Quản trị · Cài đặt. Cá nhân: bỏ Tài sản/Phê duyệt/Quản trị.
- Thêm = lối tắt Chat (Được nhắc tới, Tin đã gắn sao, Thread đang theo dõi, Duyệt nhóm, Thông báo của Chat); module
  khác **không** lặp ở đây (đổi so với câu hỏi ban đầu — ☰ đã gánh việc đó).
- Avatar → sheet tài khoản (trạng thái, Tạm dừng thông báo, Cài đặt tài khoản, Cài đặt, Đăng xuất).
- Badge thanh đáy: Trang chủ chấm; Trực tiếp số DM chưa đọc; Mục số cuộc chưa đọc (theo mức thông báo); Thêm chấm khi
  có lượt nhắc mới.

## Việc backend kéo theo (cho plan)
1. API tóm tắt hoạt động theo kênh: tin mới nhất luồng chính + trả lời thread mới nhất (người gửi, đoạn văn bản thuần,
   giờ, id thread nội bộ để mở — không hiện ra UI). Chỉ kênh người gọi còn quyền `read` (OA kênh); test deny.
2. Tìm tin trên mọi cuộc trò chuyện của người gọi (có thể làm sau; người/nhóm tìm phía client trước).
Cả hai: route mới → `vite.config.js`; spec `docs/specs/` messaging cập nhật.

## Câu hỏi còn mở
- Tìm tin nhắn toàn cục: làm cùng đợt hay sau? (mockup đã vẽ; cần API mới)
- Máy tính bảng 768–1023px: dùng vỏ điện thoại này hay cột như máy tính? (hiện `useBottomBarLayout` < 1024px)
