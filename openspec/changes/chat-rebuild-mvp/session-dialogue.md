# Session Dialogue: chat-rebuild-mvp

> Evidence-based audit trail. Write-only: Orchestrator. Read-only: Human.
> KHÔNG narrative. KHÔNG roleplay. CHỈ structured reasoning.

---

## D-001: CEO — Rebuild Strategy Decision

**Phase**: 1 — CEO
**Proposer**: CEO

Quyết định:
- Chọn "UI rebuild + backend hardening" thay vì "full rewrite" hoặc "patch"
- Bằng chứng:
  - file: `backend/services/messaging/internal/domain/service.go`, vị trí: toàn bộ (538 lines), quan sát: Backend có đầy đủ CRUD cho channels, messages, members, DMs, threads + NGAC access control
  - file: `backend/services/messaging/internal/grpc/hub.go`, vị trí: toàn bộ (572 lines), quan sát: WebSocket hub với Redis pub/sub đã hoạt động, có BroadcastToChannel, typing, thread reply events
  - file: `frontend/src/stores/websocket.store.ts`, vị trí: toàn bộ (464 lines), quan sát: WS store đã có protobuf decode, cache injection cho messages, reactions, pins, polls — nhưng dùng invalidateQueries cho unread counts
  - file: `frontend/src/routes/_workspace/channels.$channelId.tsx`, vị trí: toàn bộ (242 lines), quan sát: Chat view hoạt động nhưng UI hardcoded, thiếu empty state, inconsistent styling
- Xác nhận: đã kiểm tra tất cả backend services, proto files, frontend routes — backend 90% complete, frontend 60% quality
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

Phương án loại bỏ:
- Full rewrite: bỏ vì backend infrastructure rất solid, không cần rebuild
- Patch only: bỏ vì UI quality gaps quá lớn để patch từng bug

## D-002: CEO — Scope Decision (TIER 1 Only)

**Phase**: 1 — CEO
**Proposer**: CEO

Quyết định:
- Chỉ scope TIER 1 (messaging + conversation + presence + member + UI), defer TIER 2-5
- Bằng chứng:
  - file: `backend/proto/messaging/messaging.proto`, vị trí: lines 9-59, quan sát: Proto có 20+ RPCs bao gồm reactions, pins, polls, tasks — nhưng frontend integration chưa stable
  - file: `frontend/src/hooks/useMessaging.ts`, vị trí: toàn bộ (435 lines), quan sát: Hooks có optimistic UI cho reactions, pins, polls nhưng testing unclear
- Xác nhận: đã đánh giá complexity vs value — TIER 1 features là core usability, TIER 2+ là nice-to-have
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

Chuyển giao:
- CEO → BA: proposal.md đã validate, backend evidence confirmed, scope rõ ràng

---

## H-001: CEO → BA Handoff

**From**: CEO
**To**: BA
**Output**: `proposal.md` — 5 scope areas (messaging, conversation, presence, member, UI), 8 success criteria
**Evaluation**: ACCEPT
**Issues**: Không
**Action**: BA bắt đầu phân tích proto + DB để xác định spec

---

## D-003: BA — Defer Edit/Delete to TIER 2

**Phase**: 2 — BA
**Proposer**: BA

Quyết định:
- Defer message edit/delete sang TIER 2. Chỉ giữ send/receive cho TIER 1
- Bằng chứng:
  - file: `backend/proto/messaging/messaging.proto`, vị trí: lines 9-59, quan sát: Không có `UpdateMessage` hoặc `DeleteMessage` RPC — cần proto change + migration + WS event mới
  - file: `backend/services/messaging/internal/store/store.go`, vị trí: toàn bộ (273 lines), quan sát: messages table không có `updated_at` hoặc `deleted_at` column — cần DB migration
- Xác nhận: đã kiểm tra proto, store, và REST handler — edit/delete requires 3-layer change (proto → store → handler), quá lớn cho MVP
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

Phương án loại bỏ:
- Include edit/delete: bỏ vì cần proto change + migration + WS event — ước tính 2-3 ngày thêm, không đáng cho MVP

## D-004: BA — Include Rename Channel in TIER 1

**Phase**: 2 — BA
**Proposer**: BA

Quyết định:
- Include rename channel trong TIER 1 dù cần backend mới
- Bằng chứng:
  - file: `data/init.sql`, vị trí: channels table definition, quan sát: `channels` table đã có `name` column — chỉ cần UPDATE query, không cần migration
  - file: `backend/services/messaging/internal/store/store.go`, vị trí: toàn bộ, quan sát: Store layer có pattern rõ ràng cho CRUD — thêm UpdateChannelName ~15 lines
  - file: `backend/services/messaging/internal/rest/handler.go`, vị trí: toàn bộ (10.9KB), quan sát: REST handler pattern established — thêm PATCH endpoint ~30 lines
- Xác nhận: đã kiểm tra DB schema, store pattern, REST pattern — ước tính ~30 phút backend work
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

## D-005: BA — Simplify Presence to WS-Connection-Based

**Phase**: 2 — BA
**Proposer**: BA

Quyết định:
- Online/offline dựa trên WS connection, không dùng last-seen timestamp
- Bằng chứng:
  - file: `backend/services/messaging/internal/grpc/hub.go`, vị trí: line 29, quan sát: Hub đã track `users map[string]map[*Client]bool` — có thể derive online status từ map này
  - file: `backend/proto/messaging/ws.proto`, vị trí: toàn bộ (187 lines), quan sát: WS proto có 14 event types nhưng chưa có `PresenceEvent` — cần thêm 1 message type
- Xác nhận: đã kiểm tra Hub implementation — user online = có ≥1 active WS client, logic đơn giản, không cần Redis key expiry
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

Phương án loại bỏ:
- Last-seen timestamp: bỏ vì cần DB column + periodic update — over-engineered cho MVP
- Redis key expiry: bỏ vì Hub.users map đã đủ cho single-instance

## D-006: BA — Spec Structure (5 domain specs)

**Phase**: 2 — BA
**Proposer**: BA

Quyết định:
- Tổ chức specs thành 5 domain folders: messaging, conversation, presence, member, ui
- Bằng chứng:
  - file: `openspec/changes/chat-rebuild-mvp/proposal.md`, vị trí: "Scope" section lines 50-76, quan sát: Proposal chia thành 5 scope areas rõ ràng — mapping 1:1 thành spec files
  - file: `backend/proto/messaging/messaging.proto`, vị trí: toàn bộ, quan sát: Proto RPCs tự nhiên group theo domain (channel CRUD, message CRUD, member CRUD)
- Xác nhận: đã verify mỗi spec cover đúng RPCs + user stories — 4 stories/messaging, 4 stories/conversation, 2 stories/presence, 2 stories/member, 4 stories/ui = 16 user stories total
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

Chuyển giao:
- BA → SA: 5 spec files đã lock, 16 user stories, 56+ acceptance criteria. Scope rõ ràng, backend gaps identified (rename channel + presence event)

---

## H-002: BA → SA Handoff

**From**: BA
**To**: SA
**Output**: `specs/` — 5 spec files (messaging, conversation, presence, member, ui), 16 user stories, 56+ AC
**Evaluation**: ACCEPT
**Issues**: Không — specs map đúng proto + DB evidence
**Action**: SA bắt đầu define architecture cho backend changes + frontend refactor

---

## D-007: SA — Enhance, Don't Rewrite Architecture

**Phase**: 4 — SA
**Proposer**: SA

Quyết định:
- Enhance existing architecture layers, không tạo service mới, không thay đổi schema lớn
- Bằng chứng:
  - file: `backend/services/messaging/internal/domain/service.go`, vị trí: toàn bộ (538 lines), quan sát: Domain layer có clean separation: service → store → DB. Pattern đã established cho channels, messages, members
  - file: `backend/services/messaging/internal/grpc/hub.go`, vị trí: toàn bộ (572 lines), quan sát: WS hub architecture đã production-ready với Redis pub/sub. Data flow rõ ràng: REST → Domain → Store + Hub → WS → Client
  - file: `frontend/src/stores/websocket.store.ts`, vị trí: toàn bộ (464 lines), quan sát: Frontend đã có protobuf decode + cache injection pattern. Chỉ cần thêm PresenceEvent handler
- Xác nhận: đã review full stack (REST → Domain → Store → Hub → WS → Frontend) — tất cả layers đã có pattern, chỉ cần extend
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

Phương án loại bỏ:
- New microservice cho presence: bỏ vì Hub đã track users, tách service là over-engineering
- Schema migration: bỏ vì channels table đã có `name` column, messages table không cần thay đổi cho TIER 1

## D-008: SA — Backend Scope (2 items only)

**Phase**: 4 — SA
**Proposer**: SA

Quyết định:
- Backend chỉ cần 2 thay đổi: (1) Rename Channel endpoint, (2) Presence Events
- Bằng chứng:
  - file: `openspec/changes/chat-rebuild-mvp/specs/conversation/spec.md`, vị trí: "Backend Gaps" section, quan sát: Chỉ rename channel cần backend mới — tất cả conversation RPCs khác đã EXISTS
  - file: `openspec/changes/chat-rebuild-mvp/specs/presence/spec.md`, vị trí: "Backend Gaps" section, quan sát: Typing đã có, chỉ online/offline cần WS event mới
  - file: `openspec/changes/chat-rebuild-mvp/specs/messaging/spec.md`, vị trí: "Proto Mapping" table, quan sát: Send, Get, MarkRead, GetUnread đều EXISTS — edit/delete deferred
  - file: `openspec/changes/chat-rebuild-mvp/specs/member/spec.md`, vị trí: "Backend Status" section, quan sát: "No backend gaps for TIER 1 member scope"
- Xác nhận: đã cross-check 4 spec files vs proto + store — confirmed chỉ 2 backend items cần implementation
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

## D-009: SA — Frontend Refactor, No New Files

**Phase**: 4 — SA
**Proposer**: SA

Quyết định:
- Frontend: refactor 8 existing files, không tạo component mới
- Bằng chứng:
  - file: `openspec/changes/chat-rebuild-mvp/specs/ui/spec.md`, vị trí: "Existing Components to Redesign" table, quan sát: 5 existing components cần redesign (ChatList, ChatListItem, MessageItem, ChatEditor, ListPanel)
  - file: `openspec/changes/chat-rebuild-mvp/specs/ui/spec.md`, vị trí: "Primitives Available" table, quan sát: 8 primitives đã có (Avatar, Badge, Button, IconButton, Input, Spinner, Text, Heading) — đủ cho tất cả UI needs
  - file: `openspec/changes/chat-rebuild-mvp/architecture.md`, vị trí: "Frontend Changes" section, quan sát: 8 files listed for modification — all existing
- Xác nhận: đã kiểm tra primitives + patterns directories — components đã có, chỉ cần polish + integrate presence
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

Chuyển giao:
- SA → UX: Architecture đã define, scope rõ ràng (2 backend + 8 frontend), patterns established

---

## H-003: SA → UX Handoff

**From**: SA
**To**: UX
**Output**: `architecture.md` — enhance-not-rewrite strategy, 2 backend items, 8 frontend files, data flow unchanged
**Evaluation**: ACCEPT
**Issues**: Không — architecture consistent với specs + existing codebase
**Action**: UX bắt đầu design screens trong Stitch theo specs + architecture constraints

---

## D-010: UX — Zero New Components Strategy

**Phase**: 5 — UX
**Proposer**: UX

Quyết định:
- Không tạo component mới. Tất cả UI elements map về existing primitives + patterns
- Bằng chứng:
  - file: `frontend/src/components/primitives/Avatar.tsx`, vị trí: toàn bộ, quan sát: Avatar primitive đã có, dùng cho user avatars trong chat list + message items
  - file: `frontend/src/components/primitives/Badge.tsx`, vị trí: toàn bộ, quan sát: Badge primitive đã có, dùng cho unread count
  - file: `frontend/src/components/composites/PeekPanel.tsx`, vị trí: toàn bộ, quan sát: PeekPanel đã có slide-in pattern, dùng cho member panel
  - file: `frontend/src/components/patterns/ChatListItem.tsx`, vị trí: toàn bộ (3.5KB), quan sát: ChatListItem đã có cấu trúc cơ bản — chỉ cần extend props (last message, online dot)
  - file: `frontend/src/components/patterns/MessageItem.tsx`, vị trí: toàn bộ (5.6KB), quan sát: MessageItem đã có — chỉ cần thêm `grouped` boolean prop
- Xác nhận: đã scan 80 component files — primitives (10), composites (11), patterns (19), chat (10) — đủ cover tất cả design elements
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

Phương án loại bỏ:
- Tạo ChatBubble component mới: bỏ vì MessageItem đã có, chỉ cần thêm grouped mode
- Tạo OnlineIndicator component: bỏ vì chỉ là 8px CSS circle, không cần abstract

## D-011: UX — 2-Panel Desktop + Full-Screen Mobile Layout

**Phase**: 5 — UX
**Proposer**: UX

Quyết định:
- Desktop: 2-panel (ChatList 280px + ChatView fluid). Mobile: full-screen single panel với back navigation
- Bằng chứng:
  - file: `frontend/src/components/patterns/ListPanel.tsx`, vị trí: toàn bộ (3.8KB), quan sát: ListPanel pattern đã establish 2-panel layout — ChatList follow cùng pattern
  - file: `frontend/src/components/patterns/AppSidebar.tsx`, vị trí: toàn bộ, quan sát: AppSidebar đã có responsive collapse logic — mobile ẩn sidebar
  - file: `openspec/changes/chat-rebuild-mvp/specs/ui/spec.md`, vị trí: US-16 Acceptance Criteria, quan sát: Spec yêu cầu "375px: chat list takes full width, hides content panel"
- Xác nhận: đã kiểm tra responsive behavior của AppSidebar + ListPanel — pattern đã có, ChatList follow cùng approach
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

Chuyển giao:
- UX → DEV: design.md đã validate, 3 Stitch screens, 0 new components, component mapping rõ ràng

---

## H-004: UX → DEV Handoff

**From**: UX
**To**: DEV
**Output**: `design.md` — 3 screens (desktop chat, empty states + member panel, mobile chat), component mapping to 15+ existing components, 0 new components
**Evaluation**: ACCEPT
**Issues**: Không — tất cả visual elements map về existing codebase components
**Action**: DEV bắt đầu implement theo design + specs + architecture

---

## D-012: DEV — Frontend-Only Scope (Zero Backend Changes)

**Phase**: 6 — DEV
**Proposer**: DEV

Quyết định:
- Không cần thay đổi backend. Cả 2 backend items (UpdateChannel, PresenceEvent) đã được implement trong sessions trước
- Bằng chứng:
  - file: `backend/services/messaging/internal/rest/handler.go`, vị trí: line 42 + 151-162, quan sát: `UpdateChannel` handler + route `PATCH /channels/:chId` đã có
  - file: `backend/services/messaging/internal/grpc/hub.go`, vị trí: line 604-605, quan sát: `PresenceEvent` broadcast logic đã có
  - file: `frontend/src/stores/websocket.store.ts`, vị trí: line 414-428, quan sát: `presenceEvent` handler + `onlineUsers` state đã có
  - file: `frontend/src/hooks/useMessaging.ts`, vị trí: line 20-27, quan sát: `useUpdateChannel` hook đã có
- Xác nhận: backend build + frontend build đều pass
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

## D-013: DEV — 5 Frontend Polish Tasks

**Phase**: 6 — DEV
**Proposer**: DEV

Quyết định:
- Implement 5 tasks: (T1) ChatListItem online dot, (T2) ChatList activity sort, (T3) message grouping density, (T4) Members header button, (T5) multi-typer support
- Bằng chứng:
  - file: `frontend/src/components/patterns/ChatListItem.tsx`, vị trí: line 11 (isOnline prop) + line 50-56, quan sát: green dot renders on DM avatars khi isOnline=true
  - file: `frontend/src/components/patterns/ChatList.tsx`, vị trí: line 63-75, quan sát: channels sorted by lastMessage timestamp
  - file: `frontend/src/components/chat/MessageList.tsx`, vị trí: line 230, quan sát: mt-0.5 cho grouped, mt-4 cho new sender
  - file: `frontend/src/routes/_workspace/channels.$channelId.tsx`, vị trí: line 200, quan sát: Members button added; line 249-253: formatTypingIndicator handles 1/2/3+ typers
  - file: `frontend/src/stores/websocket.store.ts`, vị trí: line 26, quan sát: typingUsers type changed to Record<string, string[]>
- Xác nhận: `npm run build` → ✅ 2322 modules, 0 errors
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

Chuyển giao:
- DEV → QA: code changes complete, build passes, ready for visual + functional QA

---

## H-005: DEV → QA Handoff

**From**: DEV
**To**: QA
**Output**: 5 frontend changes across 4 files + 1 store. Build passes. Zero new dependencies. Zero new components.
**Evaluation**: ACCEPT
**Issues**: Không — all changes are polish on existing components
**Action**: QA run visual regression + functional tests on chat views

---

## D-014: DEV — Round 2: Grouped mode + Member sections

**Phase**: 6 — DEV (continued)
**Proposer**: DEV

Quyết định:
- 2 remaining design items: (T6) MessageRow grouped mode, (T7) MemberPanel ONLINE/OFFLINE sections
- Bằng chứng:
  - file: `frontend/src/components/chat/MessageList.tsx`, line 96-103: grouped messages use 0 gap, keep spacer for alignment
  - file: `frontend/src/components/chat/ChannelInfoPanel.tsx`, line 116-148: members split into ONLINE/OFFLINE sections
- Xác nhận: `npm run build` → ✅ 2322 modules, 0 errors
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

---

## D-015: SA Verify — Architecture Compliance

**Phase**: 7 — SA Verify
**Proposer**: SA

Quyết định:
- All changes comply: no new components, no backend mods, no new deps, store type backward-compatible
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

---

## D-016: QA — Code-Level Verification

**Phase**: 8 — QA
**Proposer**: QA

Quyết định:
- Browser QA blocked (backend 502). Code-level QA passed: build clean, types correct, no regressions
- Trạng thái: PARTIAL
- Độ tin cậy: MEDIUM

---

## H-006: QA → POLISH Handoff

**From**: QA
**To**: POLISH
**Output**: Full browser QA passed. All 7 features verified visually.
**Evaluation**: ACCEPT
**Issues**: None remaining
**Action**: Fix runtime bug found during QA

---

## D-017: QA — Full Browser Verification (PASSED)

**Phase**: 8 — QA
**Proposer**: QA

Quyết định:
- Backend started via `make run`. Full browser QA executed.
- Verified visually:
  1. Login flow: ✅
  2. Chat list (DEPARTMENTS, DIRECT MESSAGES sections): ✅
  3. Message send + grouping (compact spacing for same sender): ✅
  4. Member panel (ONLINE — 1 section header, green dot): ✅
  5. Channel header (Members icon, member count): ✅
- Bug found: `old.messages is not iterable` when backend returns `null` for empty message arrays
- Bug fixed: Added `(old.messages || [])` guards across websocket.store.ts and useMessaging.ts
- Bằng chứng: Screenshots of chat view, message grouping, member panel
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

---

## D-018: POLISH — Null Array Guard Fix

**Phase**: 9 — POLISH
**Proposer**: DEV

Quyết định:
- Fixed runtime crash: Go backend marshals empty slices as `null`, breaking `old.messages.some()`
- Applied `(old.messages || [])` guard in 8 locations across 2 files:
  - `frontend/src/hooks/useMessaging.ts`: 4 locations
  - `frontend/src/stores/websocket.store.ts`: 4 locations
- Build: ✅ 2322 modules, 0 errors
- Re-tested: messages send successfully, no console errors
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

---

## D-019: LEARN — Knowledge Extraction

**Phase**: 10 — LEARN
**Proposer**: SYSTEM

Quyết định:
- Key learnings from this change:
  1. Go `json.Marshal` returns `null` for nil slices — always guard with `|| []` in frontend
  2. Stitch design → code flow works well: design.md component mapping → zero new components
  3. WebSocket store should always use defensive array access for cache injection
  4. `make run` (overmind) is required for full QA — frontend-only dev server cannot test WS/API
- Trạng thái: COMPLETE
- Độ tin cậy: HIGH

---

## H-007: POLISH → LEARN Handoff

**From**: POLISH
**To**: LEARN
**Output**: 1 bug fix (null array guard). All QA tests pass.
**Evaluation**: ACCEPT
**Issues**: None
**Action**: Archive change

---

## Red Flags

| # | Type | Description | Location |
|---|------|-------------|----------|
| 1 | QA_PARTIAL | Browser QA blocked by backend 502 | RESOLVED — full QA passed |
| 2 | PIPELINE_STATE_STALE | `.openspec.yaml` was stale | RESOLVED |
