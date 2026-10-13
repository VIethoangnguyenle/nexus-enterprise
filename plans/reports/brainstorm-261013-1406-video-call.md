---
type: brainstorm
date: 2026-10-13
topic: Cuộc gọi video/thoại trong Chat
status: mockup chờ duyệt
mockup: design/mockups/video-call.html
base: main @ 777e3c7
---

# Brainstorm — Cuộc gọi video và thoại

## Quyết định của người dùng (2026-10-13)
- Phạm vi: **1–1 (đổ chuông) và nhóm ≤ 20 người** (bắt đầu cuộc gọi trong nhóm, ai cũng tham gia, không đổ chuông cả nhóm).
- Stack: **LiveKit tự host** trên VPS hiện có.
- Đợt đầu: **chia sẻ màn hình, chat trong cuộc gọi, giơ tay + thả cảm xúc**. Không ghi hình.

## Contract
- **Outcome:** từ một tin nhắn trực tiếp gọi thoại/video (đổ chuông, nhỡ, gọi lại); từ một nhóm bắt đầu/tham gia cuộc
  gọi tới 20 người; trong cuộc gọi: micro, camera, trình bày màn hình, giơ tay, cảm xúc, chat = luồng của chính cuộc trò
  chuyện, danh sách người tham gia; thẻ cuộc gọi trong luồng (đang diễn ra / kết thúc / nhỡ); thông báo đẩy cuộc gọi đến.
- **Constraints:** quyền vào phòng quyết định ở backend bằng NGAC trên OA của kênh (vé LiveKit chỉ cấp sau khi kiểm,
  sống ngắn); bị gỡ khỏi nhóm → bị đá khỏi phòng; không ID trên UI (tên thiết bị là nhãn hệ điều hành); secret LiveKit
  chỉ ở env; design-first; mobile ưu tiên; tuân CLAUDE.md §5 (test deny, proto 2 phía, proxy dev, migration có backup).
- **Non-goals (đợt này):** ghi hình/Egress, phụ đề, làm mờ nền, phòng chờ có người duyệt, quyền chủ phòng (tắt mic
  người khác, mời ra), gọi số điện thoại (SIP), E2EE, gọi giữa các workspace, lịch họp.
- **Acceptance:** 1–1 gọi được giữa Chrome máy tính ↔ Safari iPhone (PWA) và Android; nhóm 20 người chạy ổn trên VPS
  (đo CPU/băng thông ghi report); người không thuộc nhóm không lấy được vé (test deny) và bị ngắt ≤ 5 giây khi bị gỡ;
  mạng công ty chỉ mở 443 vẫn gọi được (qua TURN/TLS); UI khớp mockup.

## Stack đã chọn

| Lớp | Chọn | Ghi chú |
|---|---|---|
| Media server (SFU) | `livekit/livekit-server` **v1.13.x** (pin bản chính xác khi làm; v1.13.3 phát hành 2026-07-03) | Go, Apache-2.0, simulcast + dynacast + adaptive stream, data channel cho giơ tay/cảm xúc |
| TURN | TURN tích hợp của LiveKit, `turn.enabled`, `external_tls: true` sau Traefik (TCP router HostSNI `turn.<domain>`, TLS do Traefik) | Mạng chỉ mở 443 vẫn qua được |
| Token | `github.com/livekit/protocol/auth` (`NewAccessToken` + `SetVideoGrant{RoomJoin, Room}` + `SetIdentity` + `SetValidFor`) — **không** dùng `AddGrant` (deprecated) | Cấp trong service `messaging` sau khi kiểm quyền |
| Sự kiện phòng | Webhook LiveKit (`room_started`, `participant_joined`, `participant_left`, `room_finished`) → `messaging`, qua mạng nội bộ `nexus`, xác thực chữ ký bằng `livekit/protocol/webhook` | Cập nhật thẻ cuộc gọi, thời lượng, người tham gia |
| Đá người bị gỡ | `RoomServiceClient.RemoveParticipant` từ `server-sdk-go/v2` khi membership đổi (sự kiện EPP/kênh) | |
| Frontend | `livekit-client` (≥ 2.20.1) + **chỉ hooks** của `@livekit/components-react` (2.9.x); **không** dùng `@livekit/components-styles` | UI tự vẽ theo DESIGN.md |
| Đổ chuông | WebSocket hiện có (`:8081`) frame mới `call_ring` / `call_cancel` / `call_answered` | Nhiều thiết bị: trả lời một nơi, nơi khác ngừng |
| Đẩy | OneSignal (đã có) loại mới "Cuộc gọi đến", hết 45s đổi thành "Cuộc gọi nhỡ" | Web không đổ chuông được như app native |

Phiên bản: kiểm `npm view livekit-client version`, `npm view @livekit/components-react version`, tag Docker
`livekit/livekit-server` khi bắt đầu, chọn bản ≥ 2 tuần tuổi, pin chính xác.

### Phương án đã cân nhắc
- **LiveKit Cloud:** cùng SDK, không mở cổng, trả theo phút, media qua bên thứ ba → giữ làm đường lui (đổi URL + key).
- **WebRTC P2P + coturn tự làm:** rẻ, nhưng nhóm > 4 người không chạy nổi (mesh), tự viết renegotiation/ICE restart.
- **Jitsi Meet:** UI riêng (iframe), nặng, khó khớp DESIGN.md và NGAC.

## Kiến trúc

```text
Trình duyệt ──REST──► nginx ──► messaging  POST /api/channels/:id/calls        (bắt đầu; NGAC write trên OA kênh)
                                           POST /api/calls/:id/token           (vé 10 phút; NGAC write; ≤ 20 người)
                                           POST /api/calls/:id/decline|cancel  (1–1)
            ──WSS /livekit──► nginx ──► livekit-server :7880 (tín hiệu)
            ──UDP 7882 / TCP 7881──────► livekit-server (media, cổng host)
            ──TLS 443 turn.<domain>──► Traefik (TCP, TLS) ──► livekit TURN (tls_port, external_tls)
livekit-server ──webhook (mạng nexus)──► messaging POST /internal/livekit/webhook ──► DB calls + WS + thẻ trong luồng
messaging ──WS frame──► realtime (:8081) ──► người được gọi (đổ chuông) / thành viên kênh (thẻ cập nhật)
messaging ──OneSignal REST──► người được gọi khi app đóng
```

- **Dữ liệu (migration mới):** `calls` (id, channel_id FK, room_name, kind `audio|video`, started_by, started_at,
  ended_at, status `ringing|active|ended|missed|declined`, message_id của thẻ), `call_participants` (call_id, user_id,
  joined_at, left_at). Mỗi kênh tối đa một cuộc gọi `ringing|active` (unique partial index).
- **Phòng LiveKit:** `room_name` = id cuộc gọi (không lộ ra UI); `max_participants` 20 đặt khi tạo phòng;
  `empty_timeout` 30s.
- **Quyền:** bắt đầu + lấy vé = `write` trên OA của kênh (không thêm operation mới — 8 operation cố định). Webhook chỉ
  nhận từ mạng nội bộ + chữ ký. Vé: identity = user id, name = tên hiển thị, metadata không chứa gì nhạy cảm.
- **Hạ tầng:** container `nexus-livekit` trong compose prod — **ngoại lệ có chủ đích** cho luật "không container nào
  publish cổng host": UDP 7882 + TCP 7881 (ufw allow), ghi vào `docs/deployment.md`; tín hiệu đi qua nginx (`/livekit`
  cùng origin, WebSocket upgrade); TURN qua Traefik TCP router; secret `LIVEKIT_API_KEY/SECRET` trong `/opt/nexus/.env`.
  Dev: thêm `livekit` vào `docker-compose.yml` (`--dev`), proxy Vite `/livekit` → `ws://localhost:7880`.

## UI (mockup `design/mockups/video-call.html`)
1. Nút gọi ở thanh đầu (DM: Gọi thoại, Gọi video; nhóm: Bắt đầu cuộc gọi → Tham gia + pill "Đang gọi"); thẻ cuộc gọi
   đang diễn ra / kết thúc / nhỡ.
2. Kiểm tra trước khi vào: xem trước camera, đo micro, chọn micro/camera/loa, trạng thái bị chặn quyền.
3. Cuộc gọi đến: thẻ nổi máy tính, màn đổ chuông điện thoại, thông báo đẩy khi app đóng.
4. Trong cuộc gọi (máy tính): lưới tự xếp, người nói viền accent, trạng thái micro/mạng/giơ tay, thanh điều khiển,
   panel Chat (luồng của nhóm) và Người tham gia (giơ tay, trong cuộc gọi, chưa vào + Mời), cảm xúc bay.
5. Trình bày màn hình: bản trình bày lớn + dải người; thanh "Bạn đang trình bày · Dừng".
6. Điện thoại: 1–1 toàn màn + ô của mình, nhóm lưới 2 cột theo trang, sheet Thêm, thu nhỏ thành ô nổi khi quay về chat.
7. Trạng thái: đang nối lại, không kết nối được (gợi ý mạng công ty), kết thúc + tổng kết, đang ở cuộc gọi khác, bị gỡ.
- Token màu mới cho màn gọi (luôn tối): `--color-stage`, `--color-stage-tile`, `--color-stage-raised`,
  `--color-stage-hover`, `--color-on-stage`, `--color-on-stage-muted`, `--color-stage-accent`, `--color-stage-danger`
  (đưa vào `tokens.css` + DESIGN.md sau khi duyệt).

## Rủi ro
- **Băng thông VPS:** 20 người × video. Simulcast + adaptive stream giữ mỗi người nhận lớp thấp cho ô nhỏ; ước tính
  thô cần ~50–100 Mbps upload phía server cho phòng 20 người có camera. Phải đo trên VPS thật (phase kiểm chứng) và
  ghi giới hạn; nếu thiếu → giới hạn số camera bật cùng lúc hoặc chuyển LiveKit Cloud.
- **Mở cổng host** trái luật mạng hiện tại → chỉ 2 cổng media, chỉ container LiveKit, ghi rõ trong deployment.md.
- **iOS PWA:** không đổ chuông khi app đóng (chỉ thông báo); cần một cú chạm để phát âm thanh; không chia sẻ màn hình
  trên trình duyệt di động.
- **Cổng 443 TURN:** cần bản ghi DNS `turn.<domain>` và router TCP Traefik — cần quyền chỉnh Traefik dùng chung trên VPS.

## Câu hỏi còn mở
- Đặt tên miền TURN: `turn.nexus.zaneng.xyz` được không (cần thêm bản ghi DNS)?
- Ai được bắt đầu cuộc gọi trong nhóm: mọi thành viên gửi được tin (mặc định trong mockup) hay chỉ quản trị nhóm?
- Lịch sử cuộc gọi có cần màn riêng (ví dụ trong tab Thêm) hay chỉ thẻ trong luồng là đủ?
