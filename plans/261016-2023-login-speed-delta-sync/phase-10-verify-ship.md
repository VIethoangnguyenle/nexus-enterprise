---
phase: 10
title: "Đo trước/sau, kiểm chứng, review, một commit, deploy"
status: pending
priority: P1
effort: "4h"
dependencies: [1, 2, 3, 4, 5, 6, 7, 8, 9]
---

# Phase 10: Kiểm chứng và ship

## Implementation Steps
1. Mốc trước (đã có một phần trong brainstorm report): kích thước tải, đếm 72 h đợt chậm, p95 theo route.
2. Cổng: `make build-check`, `make check-layering`, `make check-ngac`, `make test` (env `TEST_DATABASE_URL` 5433 +
   `REDIS_ADDR`; ghi rõ test fail có sẵn do lệch DB dev), `cd frontend && npm test && npm run lint && npm run build`.
3. `ak:code-review` toàn bộ diff (tự review / agy read-only; không codex).
4. **Một commit** theo yêu cầu người dùng: squash các nhánh (phase 01, 02, 03–09, cùng các nhánh khác người dùng muốn
   gom), message conventional nêu hành vi, không nhắc plan/phase; không đụng các xoá `.claude/skills/` đang staged.
5. Merge `main` (người dùng yêu cầu) → CI → duyệt `production` → `deploy.sh`.
6. Sau deploy: `curl -H 'Accept-Encoding: gzip'` thấy `Content-Encoding`; đếm lại đợt chậm sau ≥ 1 khoảng idle;
   Lighthouse mobile + Web Inspector iPhone PWA; ghi số vào report; cập nhật trạng thái plan.

## Success Criteria
- [ ] Acceptance của plan.md đạt, có số.
