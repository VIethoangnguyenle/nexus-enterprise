---
phase: 1
title: "Thiết kế mobile, cài đặt, xin quyền thông báo"
status: pending
priority: P0
effort: "1d"
dependencies: []
---

# Phase 01: Thiết kế mobile, cài đặt, xin quyền thông báo

## Overview
Thiết kế trước khi code (CLAUDE.md §3): bổ sung DESIGN.md và mockup cho mọi phần mới của PWA và push.

## Requirements
- Mockup mobile 390px cho các màn chat mới (luồng hai bên, panel thread thành sheet, ô soạn với nút +, menu ⋯
  thành sheet đáy, mục trong tab Tin nhắn) — bổ sung `design/mockups/spaces.html` (khung 390).
- Gợi ý cài app: Android dùng `beforeinstallprompt` (banner nhỏ có "Cài đặt" / "Để sau", hiện sau lần dùng thứ 2);
  iOS hiện hướng dẫn "Chia sẻ → Thêm vào MH chính" một lần, đóng được.
- Xin quyền thông báo: **không bật hộp quyền của trình duyệt ngay khi mở**; trước tiên là thẻ trong app ("Nhận
  thông báo khi có tin mới?" + Bật / Để sau) sau một hành động có ý nghĩa (gửi tin đầu tiên, mở Thông báo).
- Cài đặt → Thông báo: bật/tắt push trên thiết bị này; loại sự kiện (tin nhắn trực tiếp, nhắc tên, thread đang
  theo dõi, phê duyệt); giờ yên lặng; trạng thái "Đã chặn trong trình duyệt" + hướng dẫn mở lại.
- Màn "Mất kết nối" (khung app + thanh báo, nút Thử lại) và thanh "Có bản mới — Tải lại".
- Splash/icon: dựng icon "Nexus" từ logo hiện có (192, 512, maskable, apple-touch-icon 180), màu nền theo token.

## Related Code Files
- Modify: `DESIGN.md` (§ mobile, mục PWA/push), `design/mockups/spaces.html` và/hoặc mockup mới `design/mockups/mobile.html`

## Success Criteria
- [ ] Mockup light/dark ở 390px và 768px; người dùng duyệt.
