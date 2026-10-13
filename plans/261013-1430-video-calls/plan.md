---
title: "Cuộc gọi video và thoại trong Chat (LiveKit tự host)"
description: "Gọi 1–1 có đổ chuông và cuộc gọi nhóm tới 20 người trong cuộc trò chuyện: LiveKit tự host, vé vào phòng cấp sau kiểm quyền NGAC, thẻ cuộc gọi trong luồng, chia sẻ màn hình, chat trong cuộc gọi, giơ tay, cảm xúc, đổ chuông qua WebSocket và đẩy OneSignal."
status: in-progress
priority: P1
effort: 9-11d
issue:
branch: claude/inspiring-cray-32y4n1
tags: [feature, frontend, backend, infra, realtime, chat]
blockedBy: []
blocks: []
created: 2026-10-13
---

# Cuộc gọi video và thoại trong Chat

Nguồn: [brainstorm + chọn stack](../reports/brainstorm-261013-1406-video-call.md); mockup **đã duyệt** 2026-10-13
(`design/mockups/video-call.html`, "Duyệt mockup, lên plan đi"); DESIGN.md §2 "Màn gọi" (token stage) và §6 "Cuộc gọi
video và thoại" đã cập nhật; `design/mockups/tokens.css` có token stage. Nền: `main` @ 777e3c7.

## Mục tiêu (contract)
- **Outcome:** gọi thoại/video trong tin nhắn trực tiếp (đổ chuông 45s, nhỡ, gọi lại, trả lời một thiết bị thì thiết bị
  khác ngừng); trong nhóm bắt đầu/tham gia cuộc gọi tới 20 người; trong cuộc gọi: micro, camera, chọn thiết bị, trình bày
  màn hình (máy tính), giơ tay, cảm xúc, chat = luồng chính của cuộc trò chuyện, người tham gia; thẻ cuộc gọi trong luồng
  (đang diễn ra / kết thúc / nhỡ) và xem trước "📹 …"; thông báo đẩy cuộc gọi đến; điện thoại toàn màn + thu nhỏ.
- **Constraints:** vé LiveKit chỉ cấp bởi `messaging` sau `authorizeChannel(..., ngac.OpWrite)` trên OA của kênh, sống
  10 phút; không operation NGAC mới; bị gỡ khỏi nhóm/workspace → bị đá khỏi phòng ≤ 5s; webhook chỉ nhận trên mạng nội
  bộ + chữ ký; secret `LIVEKIT_API_KEY/SECRET` chỉ ở env; không ID trên UI; CLAUDE.md §5 (TDD, test deny, proto hai phía,
  proxy dev + nginx, migration có backup + `make db-migrate`); UI khớp DESIGN.md + mockup; mobile ưu tiên.
- **Non-goals:** ghi hình/Egress, phụ đề, làm mờ nền, phòng chờ duyệt, quyền chủ phòng, SIP, E2EE, gọi liên workspace,
  lịch họp, màn lịch sử cuộc gọi riêng.
- **Acceptance:** 1–1 Chrome máy tính ↔ Safari iPhone (PWA) ↔ Chrome Android; nhóm 20 người (`lk load-test`) trên VPS
  với số đo CPU/băng thông ghi report; người ngoài nhóm không lấy được vé (test deny + curl trên stack thật); bị gỡ khi
  đang gọi → rời phòng ≤ 5s; chỉ mở 443 (chặn UDP) vẫn gọi được qua TURN/TLS; ảnh Playwright khớp mockup; gate xanh.

## Quyết định
- Phạm vi 1–1 + nhóm ≤ 20; stack LiveKit tự host; đợt đầu có chia sẻ màn hình, chat trong cuộc gọi, giơ tay + cảm xúc
  (người dùng, 2026-10-13).
- **Mặc định khi người dùng chưa trả lời** (đổi được trước phase liên quan): TURN tại `turn.nexus.zaneng.xyz` (người dùng
  thêm bản ghi DNS A → IP VPS); ai gửi được tin trong nhóm thì bắt đầu được cuộc gọi; không có màn lịch sử cuộc gọi.
- Container `nexus-livekit` là **ngoại lệ có chủ đích** của luật "không publish cổng host": chỉ UDP 7882 + TCP 7881.
- Cửa sổ cuộc gọi là route trong SPA (`/calls/$callId`), phòng LiveKit sống trong provider ở layout `_workspace` để thu
  nhỏ/đi màn khác không rớt cuộc gọi.

## Phases

| # | Phase | Priority | Effort | Depends on | Status |
|---|---|---|---|---|---|
| 01 | [Thiết kế: mockup + DESIGN.md + token stage](phase-01-design.md) | P1 | — | — | completed |
| 02 | [Hạ tầng LiveKit: dev, prod, TURN, proxy](phase-02-livekit-infra.md) | P1 | 1-1.5d | 01 | completed |
| 03 | [Backend cuộc gọi: dữ liệu, quyền, vé, webhook, thẻ](phase-03-calls-backend.md) | P1 | 2-2.5d | 01 | completed |
| 04 | [Đổ chuông, sự kiện realtime, thông báo đẩy](phase-04-ringing-push.md) | P1 | 1d | 03 | completed |
| 05 | [Frontend lõi cuộc gọi: provider, kiểm tra trước, stage, điện thoại](phase-05-call-ui.md) | P1 | 3d | 02, 03 | pending |
| 06 | [Frontend điểm bắt đầu, thẻ, cuộc gọi đến, cài đặt](phase-06-entry-cards-incoming.md) | P1 | 1.5d | 04, 05 | pending |
| 07 | [Kiểm chứng: thiết bị thật, mạng chặn UDP, tải 20 người, deny](phase-07-verification.md) | P1 | 1d | 06 | pending |

```text
01 ─┬─► 02 hạ tầng ───────────────┐
    └─► 03 backend ─┬─► 04 chuông/đẩy ─┐
                    └───────────► 05 UI lõi ─┴─► 06 điểm vào/thẻ/chuông ─► 07 kiểm chứng
```
02 và 03 song song (khác vùng file). 05 cần 02 (LiveKit chạy ở dev) và 03 (API vé).

## Capabilities in `docs/specs/`
| Capability | Action | Phase |
|---|---|---|
| `chat-calls` | **new** — vòng đời cuộc gọi, quyền vé, đá người mất quyền, thẻ, giới hạn 20 | 03, 04 |
| `web-push` | modify — loại "Cuộc gọi đến" / "Cuộc gọi nhỡ" | 04 |
| `realtime-event-delivery` | modify — khung WS `call_event` | 04 |
| `chat-cards` | modify — thẻ cuộc gọi | 03 |
| `chat-activity-feed` | modify — xem trước kind `call` | 03 |
| `docs/deployment.md` | modify — container LiveKit, cổng, TURN, env, route `/livekit`, `/api/calls` | 02 |

## Ground rules
- `ak:cook --tdd`; test **deny** cho: không thành viên bắt đầu/lấy vé, bị gỡ rồi lấy vé, webhook sai chữ ký, người vào
  phòng không còn quyền (kiểm lại ở `participant_joined`).
- Chuỗi NGAC chỉ từ `backend/ngac` (`ngac.OpWrite`); không thêm operation.
- Không đặt mã plan/phase vào code, test, commit; commit theo phase lên `claude/inspiring-cray-32y4n1`.
- Pin phiên bản chính xác ≥ 2 tuần tuổi: `livekit/livekit-server` (1.13.x), `github.com/livekit/protocol`,
  `github.com/livekit/server-sdk-go/v2`, `livekit-client` (≥ 2.20.1), `@livekit/components-react` (2.9.x, chỉ hooks).

## Risks
- Băng thông/CPU VPS cho 20 camera — đo ở 07; thiếu → giới hạn camera bật cùng lúc hoặc chuyển LiveKit Cloud (đổi
  `LIVEKIT_URL` + key, không đổi code).
- Traefik dùng chung trên VPS cần router TCP cho TURN — cần quyền sửa cấu hình/labels; DNS `turn.` do người dùng thêm.
- iOS PWA: không đổ chuông khi đóng app; cần chạm để phát âm thanh; không trình bày màn hình.

## Unresolved questions
- (Mặc định đã đặt ở trên) tên miền TURN; ai được bắt đầu cuộc gọi nhóm; có cần màn lịch sử cuộc gọi.
