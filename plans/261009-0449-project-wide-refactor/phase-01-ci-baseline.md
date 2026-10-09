---
phase: 1
title: "CI baseline"
status: pending
priority: P1
effort: 0.5d
dependencies: []
---

# Phase 01 — CI baseline

## Overview
Repo chưa có CI (`.github/workflows/` không tồn tại). Lint design-system đã khoá ở mức `error`,
có baseline typecheck, có docs-drift check, nhưng không gì chạy tự động. Mọi phase sau là refactor
lớn — cần lưới an toàn trước.

## Key insights
- `go test ./...` từ `backend/` không test service nào (CLAUDE.md §3) → CI phải chạy `make test`.
- `frontend/typecheck-baseline.txt` có 90 lỗi; `npm run typecheck:diff` chỉ fail khi lỗi mới xuất hiện.
- Audit nghi `frontend/src/routes/_workspace/drive.tsx:281-282` (`bg-amber-500/10`) đã lọt rule 4
  — CI sẽ xác nhận.

## Requirements
- Workflow chạy trên PR và push vào `main`.
- Job Go: `make build-check`, `make test` (Go 1.25).
- Job frontend: `npm ci`, `npm run lint`, `npm run typecheck:diff`, `npm test`, `npm run build`.
- Job docs: `scripts/check-docs-drift.sh`.
- Không job nào cần secret, Postgres hay AgentKit.

## Related files
- create `/home/user/nexus-enterprise/.github/workflows/ci.yml`
- modify `/home/user/nexus-enterprise/frontend/src/routes/_workspace/drive.tsx` nếu lint đỏ thật

## Implementation steps
1. Chạy local từng lệnh trên, ghi kết quả (đỏ/xanh) làm baseline.
2. Viết `ci.yml` với 3 job song song.
3. Lint đỏ có sẵn → sửa trong phase này (nhỏ) hoặc ghi vào baseline kèm lý do, không tắt rule.

## Success criteria
- [ ] CI xanh trên `main`.
- [ ] Một PR cố tình đưa vào `bg-red-500` bị CI chặn.

## Risks
- Test Go cần DB (`testutil`) → tách job integration hoặc dùng service container Postgres; quyết
  định sau khi chạy step 1.

## Spec
Không đổi hành vi hệ thống — không spec nào.
