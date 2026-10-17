---
phase: 10
title: "Kiểm chứng + review cho mỗi PR"
status: pending
priority: P1
effort: "4h mỗi PR"
dependencies: [2, 3, 4, 5, 6, 7, 8, 9]
---

# Phase 10: Kiểm chứng + review

## Overview
Chạy cho PR 1 (sau 02–05) và PR 2 (sau 06–09).

## Implementation Steps
1. Cổng: `make build-check`, `make check-layering`, `make check-ngac`, `make test s=auth|messaging|drive` (theo PR),
   `cd frontend && npm test && npm run lint && npm run build`. Go tests cần `TEST_DATABASE_URL` (DB dev 5433) và
   `REDIS_ADDR`.
2. Schema (PR 1): `make db-migrate` trên DB đang chạy, kiểm bảng/cột.
3. Xem tận mắt trên `make dev` ở 390 / 820 / 1440 (ảnh chụp đính PR). PR 1: người dùng kiểm ô soạn trên iPhone PWA.
4. `ak:test` trên hành vi đã chạm; `ak:code-review` (tự review hoặc agy read-only — không dùng codex).
5. Cập nhật spec đã nêu ở `plan.md`; trạng thái phase trong `plan.md`.
6. `ak:git` commit (conventional, không nhắc plan/phase), không đụng các xoá `.claude/skills/` đang staged của người dùng.

## Success Criteria
- [ ] Mọi cổng xanh, có output trong PR.
- [ ] Ảnh chụp 3 khổ; số đo bàn phím iPhone trước/sau (PR 1).
