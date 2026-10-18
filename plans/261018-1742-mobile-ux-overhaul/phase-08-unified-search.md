---
phase: 8
title: "Unified search"
status: completed
priority: P2
effort: "2d"
dependencies: [3]
---

# Phase 08: Unified search

## Overview
One search surface opened from the pill in every module (mockup §6): scope chips (current module default, Tất cả,
other modules), per-module filter chips, grouped results, matching actions, recent searches per scope.

## Requirements
- Functional: empty state = Tìm gần đây (per scope) · Gợi ý · Làm nhanh; typing in Tất cả = groups Hành động,
  Người, Trò chuyện, Tin nhắn, Tài liệu, Đề nghị, Sự kiện, Tài sản (personal: no Đề nghị/Tài sản), 1–3 each +
  "Xem tất cả · N", each group renders as it arrives; scoped = module filters (Chat: existing; Tài liệu: Loại, Người
  sửa, Ngày, Thư mục; Phê duyệt: Trạng thái, Loại, Người gửi, Ngày; Lịch: Lịch, Khoảng ngày; Tài sản: Loại, Trạng
  thái, Người giữ) + "Tìm ở mọi nơi" footer.
- Backend, on each service's existing prefix: approval `GET /api/approval/search` inside the existing tenant +
  membership middleware group; drive `GET /api/drive/search?workspace_id=…` with a batch read check per result and
  membership of that workspace; assets reuse `GET /workspaces/:id/assets?search=` adding the missing filters;
  people reuse the existing contacts search (auth); messages and events unchanged.

## Architecture
Client fans out in parallel (no aggregator, no gateway). Actions are a client registry filtered by workspace kind
and permissions. Existing `SearchSuggestions` / `useRecentSearches` extended, not replaced.

## Related Code Files
- Create: `frontend/src/components/search/UnifiedSearch.tsx`, `search-actions.ts`; approval `rest/search.go` + store query; drive search handler + store query; tests
- Modify: asset list handler (filters), `frontend/src/components/patterns/SearchSuggestions.tsx`, `PhoneSearchBar.tsx`, `frontend/src/hooks/useRecentSearches.ts`, `frontend/vite.config.js` and `deploy/nginx/default.conf` only if a prefix is new (both reuse existing prefixes), `docs/specs/resource-pep-coverage`, `docs/specs/unified-search`

## Implementation Steps
1. Tests first, deny first: approval search with a token for workspace A naming B → 403; drive search never returns an item the caller cannot read, nor one from another workspace; non-member → 403; personal workspace approval/asset search → empty by rule.
2. Endpoints (no SQL in transport; `make check-layering`).
3. Frontend overlay, scopes, filters, grouped streaming render, actions, recents.

## Success Criteria
- [ ] Deny tests for each new or extended endpoint; strict test runs green for approval, drive, asset
- [ ] Works in native dev and through nginx (e2e against the Docker stack)
- [ ] 390px e2e: empty, Tất cả, scoped with filter, both workspace kinds

## Risk Assessment
Slow group blocks the panel → groups render independently with skeletons; per-request timeout 3s.

## Rollback
Read-only endpoints; reverting the release removes them with no data left behind.
