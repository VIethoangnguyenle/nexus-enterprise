## Tasks

### CSS Foundation
- [x] Add `.chat-header`, `.chat-header__identity`, `.chat-header__tabs` to `index.css`
- [x] Add responsive overrides for ≤768px (compact padding, horizontal scroll for tabs)

### Channel Header Refactor
- [x] Split existing single-row header in `channels.$channelId.tsx` into 2 rows
- [x] Row 1 — Identity: channel icon (# in circle) + channel name (larger) + action icons
- [x] Row 2 — Tabs: Chat (active), Pinned, Files using `.pill-tab` classes
- [x] Wire Docs/Files tabs to open info panel with respective tab (reuse `openInfoTab`)

### Verification
- [x] `npm run build` passes
- [ ] Visual check: header matches Lark 2-row layout
- [ ] Responsive: header usable on ≤768px
