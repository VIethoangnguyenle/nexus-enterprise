## Why

Frontend hiện tại hoạt động tốt về mặt kiến trúc (queryOptions factory, Zustand chỉ cho UI, WebSocket→Query bridge) nhưng **thiếu kỷ luật ở lớp UX và type safety** — phần mà user thực sự nhìn thấy. Khi API fail, user thấy blank screen hoặc số `0` giả. Khi mutation fail, không có feedback. Sidebar dùng `div` thay vì `<Link>` — mất accessibility cơ bản.

Audit dựa trên [frontend-best-practices](file:///home/zane/Desktop/ngac/.agent/skills/frontend-best-practices/SKILL.md) skill (9 reference documents) cho thấy **22 violations, tổng compliance 74%**. Đây không phải refactor lớn — đây là hardening pass để đưa code từ "chạy được" lên "production-ready".

## What Changes

- **Reusable UI state components** — `ErrorState`, `LoadingState`, `EmptyState` components để enforce state machine pattern (Loading → Error/Empty/Data) nhất quán trên toàn app
- **Error handling trên mọi route page** — Thêm `error` + `refetch` destructuring vào 6 route pages hiện thiếu, kèm retry button
- **Mutation error feedback** — Display `mutation.error` trên tất cả pages có mutations, disable buttons khi `isPending`
- **Type safety** — Replace `any` types trong 4 mutation hooks, type `getHistory`/`getSummary` API responses, thêm `queryOptions` factory cho 3 inline queries
- **Navigation accessibility** — Sidebar chuyển từ `onClick` divs sang `<Link>` components (TanStack Router), fix `<a>` tags trong auth pages
- **Shared constants** — Extract `STATE_COLORS` duplicated 3 files → `lib/constants.ts`
- **documentApi.create fix** — Fix raw `fetch()` bypass sang dùng pattern nhất quán với `apiFetch`
- **Auth store cleanup** — Xóa `window.location.href` redirect, để layout route handle
- **TypeScript config** — Thêm `tsconfig.json` với strict mode

## Capabilities

### Modified Capabilities
- `frontend-error-handling`: Enforce Loading→Error/Empty/Data state machine trên mọi query-backed component. Reusable `ErrorState`/`LoadingState`/`EmptyState` components. Mutation error display trên mọi mutation-using page.
- `frontend-type-safety`: Replace `any` types, add `queryOptions` factory pattern cho remaining inline queries, type API responses, thêm `tsconfig.json` strict
- `frontend-navigation`: Replace non-semantic `div` onClick navigation với `<Link>` components. Fix `<a>` tags. Improve sidebar accessibility.
- `frontend-code-quality`: Extract shared constants, fix `documentApi.create` error handling, cleanup auth store redirect logic

## Impact

- **Modified files (6 route pages)**: `assets.tsx`, `documents.tsx`, `asset-dashboard.tsx`, `asset-requests.tsx`, `channels.$channelId.tsx`, `assets_.$assetId.tsx` — thêm error/empty states
- **New files (3 components)**: `components/ErrorState.tsx`, `components/LoadingState.tsx`, `components/EmptyState.tsx`
- **New files (2 config)**: `lib/constants.ts`, `tsconfig.json`
- **Modified files (hooks)**: `useAssets.ts`, `useDocuments.ts`, `useMessaging.ts` — queryOptions factory + type fixes
- **Modified files (api)**: `documents.ts` — fix error handling
- **Modified files (components)**: `Sidebar.tsx` — div→Link
- **Modified files (routes)**: `_auth/login.tsx`, `_workspace.tsx` — navigation fixes
- **Modified files (stores)**: `auth.store.ts` — remove hard redirect
- **No backend changes**
- **No new dependencies**
