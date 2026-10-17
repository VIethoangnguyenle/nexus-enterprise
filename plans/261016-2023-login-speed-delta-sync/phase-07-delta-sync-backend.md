---
phase: 7
title: "Delta sync backend: nhật ký sự kiện + phát lại"
status: pending
priority: P1
effort: "2.5d"
dependencies: [1]
---

# Phase 07: Delta sync backend (kiểu `getDifference` của Telegram)

## Hiện trạng (đã kiểm)
- Hub messaging cấp số thứ tự theo workspace (`hub_domain.go:37` `wsseq:` Redis `INCR`, bộ đếm trong process khi không
  có Redis); sự kiện kênh/người dùng mang số 0 (ngoài luồng). Lỗ hổng hoặc kết nối lại → frontend refresh mọi query
  family (`docs/specs/realtime-event-delivery/spec.md:79-90`). Sự kiện chỉ là gợi ý + id, không dữ liệu (dòng 101).

## Requirements (người dùng chốt: workspace + theo người dùng)
- **Nhật ký:** mỗi sự kiện đã cấp số ghi vào Redis Stream: `rt:ws:<workspace>` (sự kiện workspace) và
  `rt:user:<tenant>:<userNode>` (sự kiện địa chỉ người dùng: unread, mention, DM, lời mời…), `XADD … MAXLEN ~ 10000`,
  giữ tối đa 24 h (trim theo thời gian định kỳ). Luồng người dùng có số thứ tự riêng `useq:<tenant>:<userNode>` (`INCR`),
  giống `pts` theo người dùng của Telegram. Sự kiện kênh giữ như hiện tại (gợi ý cho người đang mở kênh); khi kết nối
  lại, phần kênh được đồng bộ qua `chat-state` + ETag (phase 04).
- **Phát lại:** đăng ký workspace kèm `since=<seq>`; đăng ký luồng người dùng kèm `since=<useq>`. Còn trong cửa sổ →
  gửi các frame thiếu theo thứ tự rồi tin "đã đồng bộ tới N"; ngoài cửa sổ / không có Redis / số nhỏ hơn đã cấp (bộ đếm
  mất) → "quá xa" để client tải lại như hiện tại.
- **Quyền tại thời điểm phát lại:** chỉ phát lại sau khi đăng ký được duyệt như hiện tại (tenant + `read` trên Documents
  OA qua policy); luồng người dùng chỉ cho chính người đó (cùng tenant). Frame vẫn không chứa tên/nội dung.
- Không có Redis: hành vi cũ (không phát lại), không lỗi.
- Giới hạn: phát lại tối đa N frame (vd 2 000) mỗi lần; quá → "quá xa".

## Implementation Steps
1. Test trước (hub): phát lại đúng thứ tự từ `since`; `since` ngoài cửa sổ → "quá xa"; **deny**: đăng ký workspace
   bị từ chối → không phát lại gì; người dùng khác / tenant khác không nhận luồng người dùng; frame phát lại giống frame
   trực tiếp; không Redis → không phát lại; đếm sự kiện người dùng tăng đúng một mỗi sự kiện.
2. Ghi stream trong cùng chỗ cấp số (khoá `seqMu` hiện có, hoặc `MULTI` INCR + XADD để số và frame không lệch).
3. Giao thức WebSocket: thêm trường `since` vào tin đăng ký + tin "đã đồng bộ tới" / "quá xa"; tương thích ngược
   (client cũ không gửi `since` → như hiện tại). Nếu frame/tin định nghĩa bằng proto dùng chung → `make proto` **và**
   `npm run proto:gen`.
4. Trim định kỳ theo thời gian; metric độ dài stream.
5. Spec `realtime-event-delivery`: thay yêu cầu "refresh mọi query family khi kết nối lại" bằng phát lại có điều kiện;
   thêm luồng người dùng.

## Related Code Files
- Modify: `backend/services/messaging/internal/grpc/hub.go`, `hub_domain.go`, `hub_broadcast.go`, handler WebSocket,
  `backend/pkg/realtime/event.go` (nếu đổi địa chỉ người dùng), `docs/specs/realtime-event-delivery/spec.md`

## Success Criteria
- [ ] Test trên xanh (race); kết nối lại trong 24 h chỉ nhận frame thiếu.

## Risk Assessment
- Số và frame lệch nhau khi lỗi giữa INCR và XADD → gom một lệnh nguyên tử; test.
- Bộ nhớ Redis: 10 000 frame × số workspace/người dùng → ước tính và giới hạn trong spec.
