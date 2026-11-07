---
phase: 7
title: "Đo lại, cập nhật spec, review"
status: pending
priority: P2
effort: "4h"
dependencies: [1, 2, 3, 4, 5, 6]
---
# Phase 7: Đo lại, spec, review

## Mục tiêu
Chứng minh acceptance của plan bằng số liệu, ghi hành vi mới vào spec, review trước khi merge.

## Các bước
1. `make bench-messages` ở quy mô Mid; ghi `reports/after.md`, bảng so sánh với `reports/baseline.md` cho từng
   acceptance trong `plan.md`.
2. Tạo `docs/specs/chat-unread-counts/spec.md`: định nghĩa số chưa đọc (tin chính, reply theo dõi, mention, mức
   thông báo, tin xoá, đánh dấu chưa đọc), nguồn sự thật (seq), sự kiện activity và ai nhận được.
3. Cập nhật `docs/specs/chat-threads/spec.md` (phân trang reply, vị trí đọc theo seq),
   `docs/specs/realtime-event-delivery/spec.md` (`ConversationActivityEvent`, subscribe Redis theo nhu cầu, reconnect
   jitter), `docs/specs/chat-message-lifecycle/spec.md` (xoá tin trừ số chưa đọc).
4. Thêm một dòng vào `docs/specs/README.md` cho capability mới.
5. Chạy đủ: `make test`, `make build-check`, `make check-layering`, `make check-ngac`,
   `cd frontend && npm test && npm run lint && npm run typecheck:diff && npm run build`.
6. `ak:code-review` trên toàn nhánh; sửa mọi phát hiện có thật.
7. Deploy: migration 072 chạy qua `deploy/migrate.sh` (kiểm câu `CREATE INDEX CONCURRENTLY` chạy ngoài transaction như
   migration 050).

## Validation
Mọi lệnh ở bước 5 xanh; `reports/after.md` đạt cả 7 acceptance của `plan.md`, mục nào không đạt ghi rõ và quay lại phase
tương ứng.
