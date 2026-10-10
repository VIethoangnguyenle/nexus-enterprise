---
phase: 3
title: "Chặn ở server"
status: pending
priority: P1
effort: "1.5d"
dependencies: [2]
---

# Phase 03: Chặn ở server

## Overview
Ẩn ở UI chưa đủ: mọi API Tài sản và Phê duyệt (gồm duyệt tài liệu) phải từ chối ở workspace cá nhân.

## Requirements
- Kiểm tra loại workspace ở tầng domain của `asset`, `approval` và đường duyệt trong `documents` trước khi kiểm
  quyền NGAC; lỗi domain → 403 qua `httputil.MapDomainError` (thông điệp không lộ ID).
- Không đổi mô hình NGAC (loại workspace là thuộc tính nghiệp vụ, không phải node) — nếu cần đổi đồ thị thì theo
  skill `ngac-policy-change` + spec + test vector (CLAUDE.md §5).
- Test **deny** cho từng endpoint ở workspace cá nhân và test allow ở workspace tổ chức.
- Spec `asset-authorization`, `workspace-type`.

## Success Criteria
- [ ] Mọi endpoint asset/approval/duyệt tài liệu trả 403 ở workspace cá nhân (test); tổ chức không đổi.
