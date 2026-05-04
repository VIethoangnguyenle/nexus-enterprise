# gstack Activation

## What

Kích hoạt gstack (reference skills) thành executable workflow. Hiện tại 35+ skills tồn tại trong `.agent/skills/` nhưng không có cơ chế sử dụng — agents không biết khi nào load skill nào, không có index, không có conflict resolution.

## Why

- gstack hiện tại là **decorative noise** — label "gstack thinking" xuất hiện ở 6/6 agents nhưng chỉ DEV viết code
- DEV instruction quá vague ("read relevant golang-* skills") → agent đoán hoặc skip
- Workflow skills mới (plan-eng-review, review, qa) đã import nhưng chưa mapped vào pipeline
- Không có conflict resolution khi skill mâu thuẫn với specs/architecture/knowledge

## Scope

- Tạo INDEX.md — skill catalog cho discovery
- Update DEV skill — trigger-based loading với task analysis
- Xóa gstack khỏi non-DEV agents — giảm noise
- Map workflow skills vào pipeline phases
- Thêm conflict resolution matrix

## Out of Scope

- Không tạo skills mới
- Không restructure thư mục (skills giữ nguyên vị trí)
- Không thay đổi pipeline phases
- Không thay đổi Knowledge Layer
