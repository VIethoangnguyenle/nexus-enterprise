---
phase: 5
title: "Kiểm chứng đợt 1"
status: pending
priority: P1
effort: "0.5d"
dependencies: [4]
---

# Phase 05: Kiểm chứng đợt 1

## Overview
Chứng minh 4 tiêu chí nghiệm thu của đợt 1 trên stack thật, review, rồi merge đợt 1 riêng.

## Requirements
- Kịch bản 2 người (Playwright, 2 context, stack local: Postgres + Redis + 8 service + Vite):
  A theo dõi thread; B trả lời → A thấy "1 mới" ≤ 1s; thread A không theo dõi không đổi; A mở thread → về 0.
- Mở nhóm có tin chưa đọc → vạch "Tin mới" + nút nhảy hoạt động.
- Toàn bộ `make test` (0 skip), frontend gates, `scripts/check-docs-drift.sh`, CI trên PR.
- `ak:code-review` trên diff; `doubt-driven-development` cho phần quyền nếu review còn nghi vấn.

## Related Code Files
- Create: kịch bản e2e trong thư mục test e2e sẵn có của repo (xác định khi làm; không tạo khung mới nếu đã có)

## Implementation Steps
1. Dựng stack local; chạy kịch bản; lưu ảnh chụp.
2. Chạy toàn bộ gate; review; sửa phát hiện.
3. PR → CI xanh → merge (người dùng duyệt).

## Success Criteria
- [ ] 4 tiêu chí nghiệm thu đợt 1 có bằng chứng (ảnh/log test).
- [ ] CI xanh trên PR; người dùng duyệt merge.

## Risk Assessment
- Không dựng được Redpanda/MinIO local → kịch bản chat không cần chúng; ghi rõ phần không chạy được.
