---
phase: 8
title: "Delta sync frontend"
status: pending
priority: P1
effort: "1.5d"
dependencies: [6, 7]
---

# Phase 08: Delta sync frontend

## Requirements
- `websocket.store.ts` / `lib/realtime.ts`: nhớ số cuối theo workspace và theo luồng người dùng; lưu cùng cache trên máy
  (phase 06, cùng điều kiện xoá).
- Kết nối lại / mở app: đăng ký kèm `since`; nhận frame phát lại → làm cũ đúng query bị chạm (dùng ánh xạ frame → key
  hiện có); "đã đồng bộ tới N" → cập nhật số; "quá xa" → refresh mọi query family như hiện tại.
- Lỗ hổng giữa chừng: đăng ký lại với `since` thay vì refresh hết (một lần; lỗi tiếp → refresh hết).
- Không còn refresh toàn bộ sau mỗi lần kết nối lại khi còn trong cửa sổ.

## Implementation Steps
1. Test trước: kết nối lại có `since` → chỉ key bị chạm bị invalidate; "quá xa" → refresh hết; lỗ hổng → đăng ký lại
   có `since`; đăng xuất xoá số đã lưu; workspace khác không dùng số của nhau.
2. Sửa store + realtime; giữ cách vá cache trực tiếp cho tin nhắn, cảm xúc, ghim.
3. `npm test`, `npm run lint`.

## Related Code Files
- Modify: `frontend/src/stores/websocket.store.ts`, `frontend/src/lib/realtime.ts`, `frontend/src/lib/query-persist.ts`

## Success Criteria
- [ ] Mở lại app sau 1 giờ: số request REST giảm (đo trong DevTools), dữ liệu đúng.

## Risk Assessment
- Bỏ sót ánh xạ frame → key → màn cũ. Test mỗi loại frame đang có.
