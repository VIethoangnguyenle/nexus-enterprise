---
phase: 6
title: "Spec + kiểm tra cuối"
status: completed
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
- [x] Đủ mọi mục Acceptance trong plan.md, trừ phần xem trên trình duyệt thật ở dưới.
- [ ] Ảnh chụp từng module khớp mockup. **Chưa làm**, xem bên dưới.

## Thực tế khi làm (2026-10-17)
- Spec: thêm capability mới `docs/specs/search-suggestions` cho phần dùng chung (panel, lịch sử tìm, chỉ gợi ý thứ
  đọc được, chỉ tải khi cần, giới hạn đã biết) và ghi vào `docs/specs/README.md`. Cập nhật `chat-search`,
  `contacts-documents-settings-screens`, `assets-screens`, `approval-screens` với phần riêng của từng module.
- Đã chạy: vitest 3126/3126, `npm run lint` ok, `typecheck:diff` không tăng lỗi, `vite build` ok. Review độc lập
  sau mỗi phase (02, 03, 04+05), các lỗi high/medium đều đã sửa.
- **Chưa làm: xem trên trình duyệt thật ở cỡ 390/1280.** `/browse` bị hook `scout-block` của AgentKit chặn (đường
  dẫn có `dist`), Aside không có trên máy này, và app cần đăng nhập. Zane cần tự xem, hoặc cho phép đường browse.
- Còn mở (ghi ở spec `search-suggestions` Known limits): back của Android trên 4 màn ngoài Chat, ô tìm trên điện
  thoại chưa đổi thành viên thuốc 52px, Chat trên máy tính chưa có gợi ý.
