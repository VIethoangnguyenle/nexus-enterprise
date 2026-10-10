---
title: "Project-wide refactor — UI and logic"
description: "Redesign the UI and its motion from scratch via ak:brainstorm, make every screen realtime over WebSocket, and refactor the logic underneath, closing the live authorization gaps first."
status: in-progress
priority: P1
effort: 22-30d
issue:
branch: refactor/project-wide
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
   plan. Một bộ UI hoàn toàn mới qua `ak:brainstorm`: hướng **Tín hiệu** đã chọn,
   `DESIGN.md` đã viết, mockup màn cốt lõi ở `design/mockups/core-screens.html` chờ duyệt.
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
| 01 | [CI baseline](phase-01-ci-baseline.md) | P1 | 0.5d | — | done (chờ chạy trên GitHub) |
| 02 | [Close authorization gaps](phase-02-close-authorization-gaps.md) | P0 | 2-3d | 01 | done (PR #2) — gRPC caller identity moved to 02b |
| 02b | [gRPC caller identity](phase-02b-grpc-caller-identity.md) | P2 | 3-4d | 01 | done |
| 03 | [PDP correctness and freshness](phase-03-pdp-correctness-and-freshness.md) | P0 | 2-3d | 01 | done (PR #2) — in-RAM prohibitions moved to 03b |
| 03b | [Prohibitions in the in-memory graph](phase-03b-prohibitions-in-memory.md) | P2 | 1-2d | 01 | done |
| 04 | [Frontend data layer](phase-04-frontend-data-layer.md) | P1 | 2-3d | 01 | 04a done; 04b đi cùng từng nhóm màn 06 |
| 05 | [Realtime over WebSocket](phase-05-realtime-websocket.md) | P1 | 3-4d | 02, 04 | pending |
| 06 | [UI redesign and motion](phase-06-ui-redesign-and-motion.md) ★ | P1 | 6-8d | 04 (code); design starts now | in-progress |
| 07 | [NGAC model conformance](phase-07-ngac-model-conformance.md) | P2 | 2-3d | 02b, 03b | done |
| 08 | [Backend shared packages and layering](phase-08-backend-shared-packages-and-layering.md) | P2 | 3-4d | 02 | pending |
| 09 | [Tests, dead code, large files](phase-09-tests-dead-code-and-splits.md) | P3 | 1-2d | 04, 08 | pending |

```text
Track UI     : 06-design (brainstorm)────────────┐
Track FE     : 01 ─► 04 ─┬───────────────────────┴─► 06-code ─► 09
                         └─► 05 realtime (02 đã xong)
Track secure : 01 ─► 02b ─┬─► 07 ─► 08 ─────────────────────────► 09
               01 ─► 03b ─┘
```

**Thứ tự đã chốt 2026-10-10:** 01 → 04 (đường găng tới 05 và 06-code). 02b/03b làm trước 07,
không chặn 04/05 vì cổng gRPC chỉ nằm trong mạng nội bộ (đã xác nhận). Thiết kế của 06 song song với tất cả.

**UI là trọng tâm (người dùng nhấn mạnh 2026-10-10): refactor UI + animation + hiển thị.** Để mỗi màn
chỉ bị đụng một lần, 04 tách hai bước:
- **04a nền dùng chung** (~1d): query-key factory, `useActiveWorkspace` duy nhất, `MutationCache.onError`
  + toast, quyền chỉ qua TanStack Query.
- **04b theo từng nhóm màn, gộp với 06-code cùng một PR**: sửa data layer của domain đó **và** dựng lại
  màn theo mockup (display: không ID, tên + avatar, loading/empty/error; motion: enter/exit, bỏ
  `transition-all`, reduced-motion). Thứ tự nhóm màn theo phase 06.

Mỗi PR nhóm màn chỉ bắt đầu khi mockup của nhóm đó đã được duyệt.

## Capabilities in `docs/specs/` this plan touches

CLAUDE.md §4 yêu cầu mỗi plan đổi hành vi phải nêu capability:

| Capability | Action | Phase |
|---|---|---|
| `workspace-admin-authorization` | **new** | 02 |
| `resource-pep-coverage` | **new** — endpoint → op → OA, gồm WebSocket subscribe | 02 |
| `session-logout` | **new** | 02 |
| `policy-decision-freshness` | **new** — EPP tới mọi replica PDP, dạng cache key | 03 |
| `resource-pep-coverage` | modify — caller lấy từ metadata gRPC đã xác thực, không từ body | 02b |
| `policy-decision-freshness` | modify — prohibitions đánh giá trong RAM, invalidate qua EPP | 03b |
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
- UI: thiết kế trước (brainstorm → `DESIGN.md` + mockup đã duyệt), code chỉ render thiết kế. Không ID nào
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
2. ~~Thư viện animation~~ **Đã chốt 2026-10-09:** dùng `motion` cho exit/layout animation, CSS cho phần còn lại.
3. ~~Asset O nodes~~ **Đã chốt 2026-10-10:** check trên type OA, gỡ O node per-asset — theo quyết định
   kiến trúc trong CLAUDE.md ("graph không chứa object").
4. **Shard manager (phase 08):** đề xuất xoá (368 dòng chưa từng chạy production), thêm lại khi có số đo.
5. ~~Assets shell~~ **Đã chốt 2026-10-10:** gộp `/assets` vào shell `_workspace` (mockup `assets.html` §0).
6. ~~Hướng UI~~ **Đã chốt 2026-10-09:** B Tín hiệu, mượn bảng kẻ mảnh của A, light + dark ngay
   từ đầu. Nguồn thiết kế: `DESIGN.md` + `design/mockups/`.
7. ~~Hotfix 02/03~~ **Đã xong 2026-10-10:** phần chính của 02/03 đã merge trong PR #2; phần còn lại
   tách thành 02b/03b.
8. **Cổng gRPC (02b):** **Đã xác nhận 2026-10-10** chỉ nội bộ → 02b là phòng thủ chiều sâu, P2.
9. **Mockup nhóm còn lại:** **Đã duyệt 2026-10-10** `assets.html`, `admin.html`,
   `contacts-documents-settings.html`, `auth.html` là nguồn cho code.
10. **Duyệt yêu cầu tài sản:** **Đã chốt 2026-10-10** gán tài sản cụ thể ngay trong bước duyệt (một
    dialog, backend ghi nguyên tử).
11. **Op theo vùng tài nguyên (màn quyền của vai trò):** **Đã chốt 2026-10-10** backend trả danh sách
    op hợp lệ theo loại OA qua endpoint mới; client không hard-code.
12. **Toggle "Màu theo người":** **Đã chốt 2026-10-10** bỏ — màu là danh tính (DESIGN.md).
13. **Xác thực service-to-service (sau 02b):** metadata caller chưa ký — chỉ chặn client nội bộ quên
    danh tính, không chặn kẻ đã vào mạng nội bộ (gọi thẳng policy `CreateAssignment`). Đề xuất phase
    riêng: token nội bộ ngắn hạn do biên REST ký thay cho `x-caller-*`, hoặc mTLS; trong lúc chờ,
    `make dev` nên bind gRPC vào `127.0.0.1`. Chưa lên lịch — cần người dùng quyết.
14. **Quyền chia sẻ của thành viên:** **Đã chốt 2026-10-10** chia sẻ drive kiểm tra `share` (không còn
    `write`); thành viên được cấp `share` trên Documents và drive của channel (migration backfill).
    Người nhận share "Có thể sửa" không được chia sẻ tiếp.
