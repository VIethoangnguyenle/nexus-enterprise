# Tasks: Fix Workspace Select Flow

## Implementation

- [x] **T1: Fix login redirect** — `login.tsx` dòng 60: đổi `/documents` → `/workspace-select`
- [x] **T2: Auto-skip logic** — `workspace-select.tsx`: thêm useEffect auto-navigate cho 0/1 workspace cases
- [x] **T3: Remove hardcoded metadata** — `workspace-select.tsx`: bỏ "Enterprise Tier" / "Free Tier · 1 Member", dùng generic subtitle
- [x] **T4: Forward search params** — Đảm bảo deep-link params được forward từ login → workspace-select → target page

## Verification

- [x] **V1: Build passes** — `npx vite build` thành công
- [ ] **V2: Manual test** — Login flow → thấy workspace-select → chọn → vào /documents
- [ ] **V3: Single workspace auto-skip** — Verify user với 1 workspace auto-redirect
