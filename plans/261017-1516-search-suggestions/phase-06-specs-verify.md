---
phase: 6
title: "Spec + kiểm tra cuối"
status: pending
priority: P1
effort: "2h"
dependencies: [3, 4, 5]
---

# Phase 06: Spec + kiểm tra cuối

## Overview
Cập nhật spec cho từng capability, chạy đủ các bước kiểm tra, xem trên trình duyệt thật rồi review.

## Related Code Files
- Modify: `docs/specs/chat-search/spec.md` (key lịch sử mới, việc chuyển key cũ, nhóm "Trò chuyện gần đây")
- Modify: `docs/specs/contacts-documents-settings-screens/spec.md` (Danh bạ, Tài liệu)
- Modify: `docs/specs/assets-screens/spec.md`
- Modify: `docs/specs/approval-screens/spec.md` (ô tìm mới + `?q=`)

## Implementation Steps
1. Mỗi spec thêm Requirement + Scenario, có cả scenario deny ("mất quyền thì không còn gợi ý") và "đăng xuất thì xoá".
2. `cd frontend && npm test && npm run lint && npm run build`.
3. `make dev`, xem bằng `/browse` ở cỡ 390 và 1280 trên cả 5 module, so với mockup đã duyệt. Xong thì `make dev-stop`.
4. `ak:test` rồi `ak:code-review`.

## Success Criteria
- [ ] Đủ mọi mục Acceptance trong plan.md.
- [ ] Ảnh chụp từng module khớp mockup.
