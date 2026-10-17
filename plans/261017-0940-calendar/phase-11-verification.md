---
phase: 11
title: "Kiểm chứng + tài liệu"
status: pending
priority: P1
effort: "0.5d"
dependencies: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
---

# Phase 11: Kiểm chứng

## Implementation Steps
1. `make build-check`, `make check-layering`, `make check-ngac`, `make test s=calendar`, `make test s=policy`,
   `make test s=messaging`, frontend test + lint + production bundle; ghi lỗi drift cũ (2 notification test) nếu còn, không che.
2. Smoke dev stack (`make dev`) theo "Tiêu chí hoàn thành" của plan; chụp tuần/tháng/điện thoại so mockup.
3. `ak:code-review` toàn diff; sửa finding.
4. Tài liệu: spec calendar khớp code; `docs/deployment.md` nếu có service/image mới; index specs.

## Success Criteria
- [ ] Mọi lệnh xanh; smoke đạt; review không còn finding mức cao.
