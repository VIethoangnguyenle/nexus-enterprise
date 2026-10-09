---
title: "Project-wide refactor — UI and logic"
description: "Redesign the UI and its motion on Stitch, make every screen realtime over WebSocket, and refactor the logic underneath, closing the live authorization gaps first."
status: pending
priority: P1
effort: 22-30d
issue:
branch: claude/inspiring-cray-32y4n1
tags: [refactor, frontend, backend, ui, realtime, auth, tech-debt, critical]
blockedBy: []
blocks: []
created: 2026-10-09
---

# Project-wide refactor — UI and logic

## Overview

Audit toàn repo (frontend ~17.4k dòng / 148 file, backend ~22.5k dòng Go / 8 service), sắp thành
9 phase. **Plan này chỉ lập kế hoạch, chưa sửa code.** Mỗi phase chạy bằng `ak:cook --tdd`, một PR
mỗi phase (phase 06 thì một PR mỗi nhóm màn).

Ưu tiên theo yêu cầu người dùng:

1. **UI đẹp, gồm cả animation** → [phase 06](phase-06-ui-redesign-and-motion.md), trọng tâm của
   plan. Phần thiết kế trên Stitch bắt đầu ngay, song song mọi thứ khác.
2. **Realtime qua WebSocket** cho mọi màn → [phase 05](phase-05-realtime-websocket.md).
3. Logic: lớp dữ liệu frontend, NGAC, backend layering, test.

Audit cũng tìm ra lỗ hổng phân quyền **có thật, đang chạy**. Chúng nhỏ về khối lượng nhưng phải
đi trước khi mở rộng realtime (phase 02–03), vì realtime chỉ phát thêm dữ liệu qua đúng các đường
đang hở:

- `POST /api/workspaces/:id/permissions` không có policy check → user đã đăng nhập bất kỳ tự cấp
  quyền cho mình (`backend/services/workspace/internal/domain/service.go:506`). Cả service
  workspace không gọi policy lần nào.
- WebSocket `Subscribe` không check membership (`messaging/internal/grpc/hub.go:470-475`) → đọc
  được tin nhắn mọi channel; approval event phát cho mọi user mọi tenant (`hub.go:565-591`).
- PDP **fail-open** khi query prohibition lỗi, và kết quả được cache
  (`backend/services/policy/internal/ngac/pdp_decision_engine.go:129-132`).
- `policy-read` không bao giờ refresh graph → revoke không có hiệu lực tới khi restart.
- Logout ở frontend không revoke refresh cookie, không xoá query cache.

## Phases

| # | Phase | Priority | Effort | Depends on | Status |
|---|---|---|---|---|---|
| 01 | [CI baseline](phase-01-ci-baseline.md) | P1 | 0.5d | — | pending |
| 02 | [Close authorization gaps](phase-02-close-authorization-gaps.md) | P0 | 2-3d | 01 | pending |
| 03 | [PDP correctness and freshness](phase-03-pdp-correctness-and-freshness.md) | P0 | 2-3d | 01 | pending |
| 04 | [Frontend data layer](phase-04-frontend-data-layer.md) | P1 | 2-3d | 01 | pending |
| 05 | [Realtime over WebSocket](phase-05-realtime-websocket.md) | P1 | 3-4d | 02, 04 | pending |
| 06 | [UI redesign and motion](phase-06-ui-redesign-and-motion.md) ★ | P1 | 6-8d | 04 (code); design starts now | pending |
| 07 | [NGAC model conformance](phase-07-ngac-model-conformance.md) | P2 | 2-3d | 02, 03 | pending |
| 08 | [Backend shared packages and layering](phase-08-backend-shared-packages-and-layering.md) | P2 | 3-4d | 02 | pending |
| 09 | [Tests, dead code, large files](phase-09-tests-dead-code-and-splits.md) | P3 | 1-2d | 04, 08 | pending |

```text
Track UI     : 06-design (Stitch) ──────────────┐
Track FE     : 01 ─► 04 ─────────────────────────┴─► 06-code ─► 09
Track secure : 01 ─► 02 ─┬─► 05 realtime
                   03 ───┴─► 07 ─► 08 ──────────────────────────► 09
```

02/03 chạy song song (`ak:worktree`); 04 song song với 02/03; thiết kế của 06 song song với tất cả.

## Capabilities in `docs/specs/` this plan touches

CLAUDE.md §4 yêu cầu mỗi plan đổi hành vi phải nêu capability:

| Capability | Action | Phase |
|---|---|---|
| `workspace-admin-authorization` | **new** | 02 |
| `resource-pep-coverage` | **new** — endpoint → op → OA, gồm WebSocket subscribe | 02 |
| `session-logout` | **new** | 02 |
| `policy-decision-freshness` | **new** — EPP tới mọi replica PDP, dạng cache key | 03 |
| `batch-access-check` | modify — prohibitions fail closed | 03 |
| `drive-permission-engine` | modify — đóng divergence đang mở (tenant trong cache key) | 03 |
| `drive-tree-navigation` | modify — folder path trong URL | 04 |
| `realtime-event-delivery` | **new** — fan-out user/channel/workspace, tenant scope, seq, resync | 05 |
| `drive-realtime-sync` | modify — ghi divergence (chưa có producer), rồi đóng | 05 |
| `drive-context-panel` | modify — owner/actor hiển thị bằng tên | 06 |
| `lark-sidebar-layout` | modify nếu shell đổi khi gộp assets | 06 |
| `tenant-ngac-init` | modify — tên node keyed theo ID | 07 |
| `asset-authorization` | **new** — check trên type OA, bỏ O node per-asset | 07 |

## Ground rules for every phase

- `ak:cook --tdd`: test đỏ trước. Mọi nhánh PDP mới phải có test **deny** (CLAUDE.md §5).
- Thay đổi policy model → spec + test vector trong `backend/services/policy/internal/ngac/` cùng
  commit; dùng skill `ngac-policy-change`.
- Chuỗi NGAC chỉ đến từ `backend/ngac`. Ghi graph phải đi qua EPP.
- UI: thiết kế trên Stitch trước (`.stitch/WORKFLOW.md`), code chỉ render thiết kế. Không ID nào
  lên màn hình. Mọi chuyển động dùng motion token.
- Proto dùng chung đổi → `make proto` **và** `npm run proto:gen`.
- Route REST mới → `frontend/vite.config.js` (cả khối regex nếu nằm dưới `/api/workspaces/:id/`).
- Schema đổi → `make db-migrate` và kiểm chứng trên DB đang chạy.
- Xong phase: `ak:test` → `ak:code-review` → với 02/03/05 thêm `doubt-driven-development`.

## Non-goals

- Không đổi stack chính (React/TanStack/Zustand/Tailwind, Go/Echo/pgx), không thêm gateway, không
  đổi 8 operation NGAC. Ngoại lệ có thể: một thư viện animation (Unresolved Q2).
- Không gộp service, không đổi ranh giới module Go.

## Unresolved questions

1. **Workspace authz (phase 02):** đề xuất `invite` cho invite/remove member, `manage` cho
   permission/role/folder/department.
2. **Thư viện animation (phase 06):** thêm `motion` (Framer Motion, ~30–40KB gz) cho exit/layout
   animation, hay chỉ CSS + View Transitions API (nhẹ, nhưng exit animation khó hơn)? Đề xuất: `motion`.
3. **Asset O nodes (phase 07):** chuyển sang check trên type OA (đề xuất) hay giữ O node và ghi
   ngoại lệ vào spec?
4. **Shard manager (phase 08):** đề xuất xoá (368 dòng chưa từng chạy production), thêm lại khi có số đo.
5. **Assets shell (phase 06):** gộp `/assets` vào layout `_workspace` (cần Stitch screen mới cho
   nav) hay giữ layout riêng nhưng dùng chung guard/WebSocket/logout?
6. **Stitch:** session này không có Stitch MCP. Thiết kế phase 06 do người dùng làm trên Stitch,
   hay cấu hình Stitch MCP để agent làm?
7. Phase 02/03 có tách ra làm ngay như hotfix trước khi duyệt phần còn lại?
