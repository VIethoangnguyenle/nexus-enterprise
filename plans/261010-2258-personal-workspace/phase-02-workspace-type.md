---
phase: 2
title: "Loại workspace đáng tin"
status: pending
priority: P1
effort: "1d"
dependencies: [1]
---

# Phase 02: Loại workspace đáng tin

## Overview
`workspaces.type` hiện chỉ được gán 'organization' khi Google `hd` có tên miền (`auth/internal/store/identity.go`);
mọi đường khác mặc định 'personal' → cột không tin được. Proto `Workspace` và `frontend/src/api/workspaces.ts` không trả loại.

## Requirements
- Gán loại ngay lúc tạo: tự tạo khi đăng nhập lần đầu = `personal`; "Tạo workspace" và Google theo tên miền =
  `organization`. CHECK constraint `type IN ('personal','organization')`, NOT NULL.
- Migration (số tiếp theo lúc làm; plan chat dùng 037–040): backfill theo quyết định 1; in số lượng mỗi loại.
- Proto `Workspace.type` (enum hoặc string) → `make proto` + `npm run proto:gen`; API `/api/workspaces` và
  `/api/me` trả loại; hook frontend `useWorkspaceType()`.
- Spec `tenant-identity`.

## Implementation Steps
1. Test đỏ (store/domain auth): mỗi đường tạo ra đúng loại; backfill đúng trên dữ liệu mẫu.
2. Backup DB → migration → `make db-migrate` → kiểm chứng `SELECT type, count(*)`.
3. Hiện thực; gates.

## Success Criteria
- [ ] Mọi workspace có loại đúng; proto hai phía đã sinh lại; test xanh.
