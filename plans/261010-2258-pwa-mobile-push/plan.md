---
title: "PWA, mobile UI and OneSignal push"
description: "Make the app installable to the home screen (PWA), optimise every screen for phones, and send push notifications through OneSignal."
status: in-progress
priority: P0
effort: 8-10d
issue:
branch: claude/inspiring-cray-32y4n1
tags: [feature, frontend, backend, mobile, pwa, notifications]
blockedBy: []
blocks: []
created: 2026-10-10
---

# PWA, tối ưu mobile và thông báo đẩy OneSignal

> **Ưu tiên (người dùng, 2026-10-10): tối ưu giao diện mobile.** Phase 01 (thiết kế mobile) và 03 (rà + sửa mọi
> màn hiện có ở 390px) chạy **ngay**, song song với Chat đợt 1; mọi phase UI của plan Chat làm mobile-first.

Nguồn: yêu cầu người dùng 2026-10-10 ("tối ưu giao diện cho mobile. Làm PWA để add vào homescreen"; "Có thông
báo đẩy về, dùng OneSignal").

## Mục tiêu (contract)
- **Outcome:** Người dùng thêm Nexus vào màn hình chính (Android Chrome, iOS Safari 16.4+), mở như app (không
  thanh địa chỉ), mọi màn dùng tốt ở 390px; nhận thông báo đẩy qua OneSignal khi app đóng (tin nhắn trực tiếp,
  được nhắc tên, trả lời trong thread đang theo dõi, việc cần duyệt…), bấm thông báo mở đúng chỗ.
- **Constraints:** thiết kế trước (DESIGN.md + mockup) rồi mới code; không đẩy nội dung cho người đã mất quyền
  đọc (kiểm lại quyền NGAC lúc gửi); khoá REST OneSignal chỉ ở biến môi trường/secret, không commit; không UUID
  trên UI; iOS chỉ nhận push khi app đã được thêm vào màn hình chính.
- **Non-goals:** app native (React Native/Capacitor), làm việc offline đầy đủ (đọc/soạn khi mất mạng), SMS/email
  qua OneSignal, in-app messaging của OneSignal.
- **Acceptance:** Lighthouse PWA installable xanh; cài được trên Android + iOS; mất mạng mở app thấy khung app và
  thông báo "Mất kết nối"; push tới trong ≤ 10s khi app đóng, không tới khi đang mở đúng cuộc trò chuyện; người
  bị gỡ khỏi nhóm không nhận push của nhóm (test deny); mọi màn không cuộn ngang ở 390px.

## Quyết định
- Push: **OneSignal** (người dùng chốt). Web SDK v16, định danh người dùng bằng `external_id` = `users.id`
  (`OneSignal.login`), gửi từ backend qua REST API `POST /notifications` nhắm `include_aliases.external_id`.
- Mất mạng: **chỉ khung app + báo mất kết nối** (người dùng đồng ý 2026-10-10).
- Icon/tên: **"Nexus"**, icon chữ N trên nền accent (người dùng đồng ý 2026-10-10).
- Push **có xem trước nội dung** (đoạn đầu văn bản thuần); quản trị workspace tắt được cho cả workspace (người dùng chốt 2026-10-10).
- Mockup `design/mockups/push.html` **đã duyệt** (2026-10-10).

## Phases

| # | Phase | Priority | Effort | Depends on | Status |
|---|---|---|---|---|---|
| 01 | [Thiết kế mobile, cài đặt, xin quyền thông báo](phase-01-design-mobile-and-push.md) | P0 | 1d | — | pending |
| 02 | [Nền tảng PWA](phase-02-pwa-foundation.md) | P1 | 1-1.5d | 01 | completed |
| 03 | [Tối ưu giao diện mobile](phase-03-mobile-ui.md) | P0 | 2-3d | 01 (màn không phải chat: làm ngay; màn chat: cùng chat phase 04) | completed (màn chat: cùng chat 04) |
| 04 | [OneSignal phía frontend](phase-04-onesignal-frontend.md) | P1 | 1d | 02 | pending |
| 05 | [Gửi push từ backend](phase-05-push-backend.md) | P1 | 2d | 04 | in-progress (backend xong) |
| 06 | [Kiểm chứng trên thiết bị thật](phase-06-device-verification.md) | P1 | 0.5-1d | 03, 05 | pending |

```text
01 design ──► 02 PWA ──► 04 OneSignal FE ──► 05 push BE ──┐
        └──► 03 mobile UI (màn chat sau chat đợt 1) ───────┴─► 06 verify
```

## Capabilities in `docs/specs/`
| Capability | Action | Phase |
|---|---|---|
| `pwa-install` | **new** — manifest, service worker, offline shell, update flow | 02 |
| `web-push` | **new** — OneSignal identity, push rules, suppression, authz recheck | 04, 05 |
| `notifications` | modify — push channel next to in-app notifications | 05 |

## Ground rules (CLAUDE.md §5)
- Mockup duyệt trước khi code UI; không ID trên màn hình.
- `ak:cook --tdd`; mỗi nhánh quyền mới có test **deny**.
- Route REST mới (tuỳ chọn push của người dùng) → `frontend/vite.config.js`.
- Migration (bảng tuỳ chọn push) → backup, `make db-migrate`, kiểm chứng trên DB thật.
- Secret `ONESIGNAL_REST_API_KEY` chỉ trong `.env*` gitignored / secret của deploy; ghi tên biến vào `docs/deployment.md`.

## Risks
- OneSignal và service worker PWA tranh nhau phạm vi `/` → dùng **một** SW: SW của app `importScripts` SW OneSignal,
  khai báo `serviceWorkerPath` cho SDK; kiểm bằng DevTools > Application.
- Nội dung push đi qua máy chủ OneSignal (bên thứ ba) → xem câu hỏi mở về xem trước nội dung.
- iOS: chỉ push khi đã thêm vào màn hình chính, phải xin quyền từ một cú chạm của người dùng.

## Unresolved questions
- Không còn.
