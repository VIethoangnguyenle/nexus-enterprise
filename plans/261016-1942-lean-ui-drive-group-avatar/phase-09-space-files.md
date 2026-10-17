---
phase: 9
title: "Tab Tệp trong chat"
status: pending
priority: P2
effort: "5h"
dependencies: [7]
---

# Phase 09: Tab Tệp (`spaces/SpaceFiles.tsx`)

## Overview
Thay bảng 4 cột bằng `FileGrid` + chip lọc + nhóm thời gian; vẫn mở `FileViewer`, vẫn "Mở trong Tài liệu".

## Requirements
- Nguồn dữ liệu giữ nguyên (`useChannelDrive` + thư mục con).
- Chip (radio): Tất cả · Ảnh & video · Tài liệu · Khác (theo `kindOf`). Rỗng theo chip: "Chưa có ảnh nào trong nhóm." …
- Nhóm Hôm nay / Tuần này / Tháng này / Cũ hơn (`lib/file-groups.ts`); ảnh/video ô vuông, còn lại `FileCard`.
- Toggle Lưới/Danh sách nhớ riêng (`space-files`).
- Trạng thái lỗi/rỗng hiện có giữ chữ.

## Implementation Steps
1. Sửa `SpaceFiles.test.tsx` trước: chip lọc, nhóm thời gian, mở viewer đúng index trong danh sách đã lọc, Esc trả focus.
2. Sửa component.
3. Spec `docs/specs/chat-attachments/spec.md` (mục tab Tệp).

## Related Code Files
- Modify: `frontend/src/components/spaces/SpaceFiles.tsx` (+ test)

## Success Criteria
- [ ] Acceptance 5 + 8 của brainstorm cho tab Tệp; xem tận mắt 390 / 820 / 1440.

## Risk Assessment
- Viewer phải nhận danh sách **đã lọc** để ←/→ không nhảy sang tệp bị ẩn.
