---
phase: 2
title: "Loại workspace đáng tin"
status: completed
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
- [x] Mọi workspace có loại đúng; proto hai phía đã sinh lại; test xanh.

## Kết quả (2026-10-11)
- Loại gán lúc tạo: đăng nhập lần đầu (OTP, Google không `hd`) = `personal`; "Tạo workspace" và Google theo
  tên miền = `organization`. `CreateWorkspace` từ chối loại thiếu/lạ trước mọi ghi đồ thị/DB.
- Migration 044: backfill một lần (đánh dấu bằng comment cột), DB dev 37 personal / 23 organization; bỏ
  DEFAULT 'personal' — insert không ghi loại thì lỗi. Mọi fixture test ghi rõ loại.
- `workspace.proto` chỉ Go dùng (frontend sinh TS từ `ws.proto`), nên chỉ `make proto`.
- API: `GET /api/workspaces`, `GET /api/me/workspaces`, `GET /api/me` (`current_tenant.type`); hook
  `useWorkspaceType()` (không đoán khi chưa biết).
- Review: `plans/reports/code-reviewer-261011-0845-workspace-type.md` — M1 (default) và M2 (fake) đã sửa.
