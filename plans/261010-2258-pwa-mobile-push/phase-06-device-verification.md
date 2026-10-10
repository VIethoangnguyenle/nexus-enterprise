---
phase: 6
title: "Kiểm chứng trên thiết bị thật"
status: pending
priority: P1
effort: "0.5-1d"
dependencies: [3, 5]
---

# Phase 06: Kiểm chứng trên thiết bị thật

## Overview
Cài và dùng thử trên Android (Chrome) và iPhone (Safari 16.4+, đã thêm vào MH chính) trên bản deploy thật.

## Requirements
- Kịch bản: cài app, đăng nhập, bật thông báo, người thứ hai nhắn DM / nhắc tên / trả lời thread → push tới, bấm mở
  đúng chỗ; đang mở kênh thì không push; gỡ khỏi nhóm thì không push; mất mạng → khung app; có bản mới → thanh tải lại.
- Ghi kết quả + ảnh màn hình vào `plans/261010-2258-pwa-mobile-push/reports/`.
- Cập nhật `docs/deployment.md` (biến môi trường OneSignal, header SW) và README mục cài app.

## Success Criteria
- [ ] Tất cả kịch bản đạt trên cả Android và iOS; tài liệu cập nhật.
