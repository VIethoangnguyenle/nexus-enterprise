---
phase: 3b
title: "Prohibitions in the in-memory graph"
status: done
priority: P2
effort: 1-2d
dependencies: [1]
---

# Phase 03b — Prohibitions in the in-memory graph

## Overview
Phần còn lại của phase 03. Mỗi lần ALLOW, PDP vẫn query DB để tìm prohibition
(`ProhibitionStore.FindForSubjects`, `pap_prohibition.go:120`) — trái nguyên tắc "runtime check
không chạm DB" (CLAUDE.md §3).

## Requirements
- `LoadGraph` / `ReloadGraph` nạp prohibitions vào RAM cùng graph.
- Create/Remove prohibition: DB trước, RAM sau, rồi phát EPP để policy-read và cache cùng cập nhật.
- Decision engine đánh giá prohibition từ RAM; thứ tự quyết định giữ nguyên (deny-override chỉ áp
  lên ALLOW, mặc định DENY).

## Success criteria
- [ ] Test vector: prohibition khớp → DENY dù có association; tạo prohibition trên policy → policy-read
      DENY sau EPP, không restart.
- [ ] Đường ALLOW không còn query DB (test với store đếm số lần gọi).
- [ ] Test hiện có cho prohibition fail-closed vẫn xanh.

## Risks
- Bộ nhớ: đo số prohibition hiện có trước khi làm.

## Spec
Modify `docs/specs/policy-decision-freshness/spec.md` + test vector trong
`backend/services/policy/internal/ngac/` cùng commit (CLAUDE.md §5).
