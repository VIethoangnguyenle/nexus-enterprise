---
phase: 4
title: "Một request trạng thái chat + ETag/304"
status: pending
priority: P1
effort: "1.5d"
dependencies: [3]
---

# Phase 04: Trạng thái chat một lần, kiểm tra lại rẻ

## Requirements
- `GET /api/workspaces/:id/chat-state` (messaging): kênh + DM + unread + activity + chat-sections + số mention chưa
  đọc + thread chưa đọc, một lần, kiểm quyền theo lô (`policyclient.BatchCheck`) như các endpoint hiện có. Endpoint
  cũ giữ nguyên. Thêm vào khối regex `/api/workspaces` trong `frontend/vite.config.js` (route dưới `/api/workspaces/:id/`).
- ETag (kiểu `hash` của Telegram) cho `chat-state`, `contacts`, `/me/workspaces`, `/workspaces`: phiên bản trong Redis
  theo (tenant, workspace, user khi dữ liệu theo user), tăng ở đúng chỗ phát sự kiện tương ứng; `If-None-Match` khớp →
  304 không truy vấn DB. Không có Redis → không ETag (luôn 200), không sai.
- Frontend: dùng `chat-state` thay 6 request lúc mở; fetch gửi `If-None-Match`, 304 giữ dữ liệu cũ.

## Implementation Steps
1. Test trước (domain + REST): allow trả đủ phần; **deny**: kênh không có quyền đọc không xuất hiện; ETag đổi sau khi
   có sự kiện; 304 không chạm store (store giả đếm lời gọi); tenant khác không dùng chung phiên bản.
2. Viết endpoint, phiên bản, middleware ETag dùng chung (`backend/pkg/httputil`).
3. Frontend hook `useChatState` + test; bỏ 6 query khởi động (hoặc seed từ `chat-state`).
4. Spec `realtime-event-delivery` (phiên bản theo sự kiện), `make check-layering`.

## Related Code Files
- Create: `backend/services/messaging/internal/{domain,rest}/chat_state*.go`, `backend/pkg/httputil/etag.go`
- Modify: `frontend/vite.config.js`, `frontend/src/hooks/useConversations.ts` (và các hook khởi động)

## Success Criteria
- [ ] Mở app: 1 request trạng thái chat thay 6; lần sau không đổi → 304.

## Risk Assessment
- Quên tăng phiên bản ở một đường ghi → dữ liệu cũ. Gắn tăng phiên bản vào cùng hàm phát sự kiện, test mỗi loại sự kiện.
