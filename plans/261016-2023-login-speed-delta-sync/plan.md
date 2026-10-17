---
title: "Đăng nhập và mở app nhanh: gRPC không idle, tải trang nhẹ, cache trên máy, delta sync kiểu Telegram"
status: completed
priority: P1
branch: main
created: 2026-10-16
brainstorm: plans/reports/brainstorm-261016-1942-login-speed.md
relates: [261016-1942-lean-ui-drive-group-avatar]
blockedBy: []
blocks: []
---

# Đăng nhập và mở app nhanh

Hợp đồng (outcome, constraints, non-goals, acceptance 1–7), bằng chứng đo production và mọi quyết định của người
dùng: brainstorm report ở frontmatter. Plan này phân pha, không lặp lại.

Người dùng (2026-10-16): gom tất cả vào **một commit**, merge `main`, **deploy chung**; delta sync nằm trong plan này.

## Capability trong `docs/specs/`
- Sửa: `realtime-event-delivery` (nhật ký sự kiện, phát lại theo `since`, luồng theo người dùng — thay quy tắc
  "refresh mọi query family" khi kết nối lại), `tenant-auth-flow` (OTP gửi nền, khởi động ít vòng),
  `sign-in-screens` (trạng thái "Đang cập nhật" khi mở từ cache), `pwa-install` (cache trên máy và quy tắc xoá).
- Thêm: không. Không đổi mô hình NGAC.

## Phases

| # | Phase | Status | Phụ thuộc |
|---|-------|--------|-----------|
| 01 | [Kênh gRPC không idle, kết nối sẵn, tự nối lại](phase-01-grpc-channel-idle.md) | done, production `0bf4455` (#20/#21/#22) | — |
| 02 | [Tải trang: gzip, bỏ Manrope, bỏ preload gọi video](phase-02-load-quick-wins.md) | done, production `0bf4455` (#20/#21/#22) | — |
| 03 | [OTP gửi nền + khởi động ít vòng](phase-03-otp-and-startup.md) | done, production `0bf4455` (#20/#21/#22) | — |
| 04 | [Một request trạng thái chat + ETag/304](phase-04-chat-state-etag.md) | done, production `0bf4455` (#20/#21/#22) | 03 |
| 05 | [Chia bundle](phase-05-bundle-split.md) | done, production `0bf4455` (#20/#21/#22) | 02 |
| 06 | [Cache trên máy (IndexedDB)](phase-06-device-cache.md) | done, production `0bf4455` (#20/#21/#22) | 04, 05 |
| 07 | [Delta sync backend: nhật ký sự kiện + phát lại](phase-07-delta-sync-backend.md) | done, production `0bf4455` (#20/#21/#22) | 01 |
| 08 | [Delta sync frontend](phase-08-delta-sync-frontend.md) | done, production `0bf4455` (#20/#21/#22) | 06, 07 |
| 09 | [Cache Redis có phiên bản + danh bạ lô lớn](phase-09-redis-cache.md) | done, production `0bf4455` (#20/#21/#22) | 04 |
| 10 | [Đo trước/sau, kiểm chứng, review, một commit, deploy](phase-10-verify-ship.md) | done, production `0bf4455` (#20/#21/#22) | 01–09 |

Song song được: 03 ∥ 05 ∥ 07 (khác file); 04 → 06/09; 06 + 07 → 08.

## Acceptance (tóm tắt — chi tiết brainstorm §Acceptance 1–7)
- Đợt chậm 8 s sau idle về 0 (đếm lại 72 h log production sau deploy).
- JS nén; màn đăng nhập ≤ 250 KB nén, Trang chủ ≤ 450 KB; không tải livekit khi chưa gọi.
- Mở app đã đăng nhập có cache: Trang chủ hiện trước request mạng đầu tiên (< 1 s trên iPhone PWA).
- Kết nối lại / mở lại: chỉ tải phần thay đổi khi `since` còn trong cửa sổ; không còn "refresh mọi thứ".
- `POST /otp/request` p95 < 300 ms; đăng xuất / đổi workspace / 401 xoá cache trên máy.
- Mọi cổng `make` + `npm` xanh; test deny cho phát lại sự kiện và ETag.

## Rủi ro chính
- Phát lại sự kiện phải lọc quyền tại thời điểm phát lại (phase 07).
- Gom một commit lớn → review và rollback khó; mỗi phase vẫn có test riêng, `deploy.sh` có rollback.
- Thời gian: 01–02 xong ngay, 07–08 lớn nhất; deploy chung nghĩa là bản sửa 8 s chờ tới khi 03–09 xong.

## Câu hỏi còn mở
- Không còn (thời điểm merge hỏi khi tới phase 10).
