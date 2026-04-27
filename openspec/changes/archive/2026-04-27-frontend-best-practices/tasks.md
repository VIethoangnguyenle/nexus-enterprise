## 1. Foundation — Reusable Components & Shared Constants

- [x] 1.1 Create `components/ErrorState.tsx` — props: `title`, `message`, `onRetry`. Renders error icon, message, retry button
- [x] 1.2 Create `components/LoadingState.tsx` — props: `size`. Renders centered spinner
- [x] 1.3 Create `components/EmptyState.tsx` — props: `icon`, `title`, `description`, `action` (label+onClick). Renders empty state CTA
- [x] 1.4 Create `lib/constants.ts` — extract `ASSET_STATE_COLORS` and `REQUEST_STATUS_COLORS` from route pages

## 2. API Layer Fixes

- [x] 2.1 Add `apiUpload` helper to `api/client.ts` — FormData upload with auth injection, 401 auto-logout, error throwing (matching `apiFetch` contract)
- [x] 2.2 Fix `api/documents.ts` — replace raw `fetch()` in `documentApi.create` with `apiUpload`, remove duplicate `useAuthStore` import
- [x] 2.3 Type `api/assets.ts` — add `AssetHistory` and `AssetSummary` interfaces, replace `any[]` and `any` return types in `getHistory`/`getSummary`

## 3. Query Hook Hardening

- [x] 3.1 Convert `useAssetTransitions` to `queryOptions` factory pattern in `hooks/useAssets.ts`
- [x] 3.2 Convert `useAssetHistory` to `queryOptions` factory pattern in `hooks/useAssets.ts`
- [x] 3.3 Convert `useDocument` to `queryOptions` factory pattern in `hooks/useDocuments.ts`
- [x] 3.4 Replace `any` types in mutation hooks — `useCreateAssetType`, `useCreateAsset`, `useCreateAssetRequest` in `hooks/useAssets.ts`, `useCreateChannel` in `hooks/useMessaging.ts`
- [x] 3.5 Add `['asset-summary', wsId]` invalidation to `useCreateAsset` onSuccess

## 4. Route Pages — Error/Loading/Empty States

- [x] 4.1 Fix `routes/_workspace/assets.tsx` — add `error` + `refetch` destructuring, import `ErrorState`/`LoadingState`/`EmptyState`, replace inline JSX, import `ASSET_STATE_COLORS` from constants
- [x] 4.2 Fix `routes/_workspace/documents.tsx` — add error state with retry, use `LoadingState`/`EmptyState` components
- [x] 4.3 Fix `routes/_workspace/asset-dashboard.tsx` — add loading/error/empty states for `useAssetSummary` and `useAssetTypes` queries
- [x] 4.4 Fix `routes/_workspace/asset-requests.tsx` — add error state, fix reject button disabled state (`reject.isPending`), display mutation errors inline, import `REQUEST_STATUS_COLORS` from constants
- [x] 4.5 Fix `routes/_workspace/channels.$channelId.tsx` — add error state for messages query, display `send.error` inline near input
- [x] 4.6 Fix `routes/_workspace/assets_.$assetId.tsx` — add error state, show spinner on transition buttons when `isPending`, display `transition.error` inline, import `ASSET_STATE_COLORS` from constants

## 5. Navigation & Accessibility

- [x] 5.1 Refactor `components/Sidebar.tsx` — replace `div.sidebar-item` + `onClick` with `<Link>` from TanStack Router, use `activeProps` for active state, remove `isActive()` helper and `useNavigate`/`useLocation`
- [x] 5.2 Fix `routes/_auth/login.tsx` — replace `<a onClick={navigate}>Register</a>` with `<Link to="/register">Register</Link>`
- [x] 5.3 Fix `routes/_workspace.tsx` — remove unused `useNavigate` import

## 6. Store & Config Cleanup

- [x] 6.1 Fix `stores/auth.store.ts` — remove `window.location.href = '/login'` from `logout()`, let layout route handle redirect
- [x] 6.2 Create `tsconfig.json` — strict mode, path aliases, module resolution for Vite

## 7. Verification

- [x] 7.1 Build check: `npm run build` — verify zero TypeScript errors
- [x] 7.2 Lint check: `npm run lint` — verify zero ESLint errors
- [ ] 7.3 Visual review: `npm run dev` — verify all pages render correctly with loading/error/empty states
