## Tasks

### Phase 1: Primitives & Composites
- [x] `Button.tsx` — Verify size scale uses spacing standard, remove non-scale values
- [x] `Input.tsx` — Standardize padding, replace raw `text-sm` with `text-body`
- [x] `Select.tsx` — Replace raw `text-sm` with `text-body`, standardize padding
- [x] `Textarea.tsx` — Replace raw `text-sm` with `text-body`, standardize padding
- [x] `Text.tsx` — Verify variant mapping uses design token utilities
- [x] `Avatar.tsx` — Verify size consistency, replace raw `text-xs`/`text-sm`
- [x] `DataTable.tsx` — Replace inline `style={{ height }}` with `h-9`, replace raw `text-xs`
- [x] `Tabs.tsx` — Replace raw `text-sm` with `text-small-ui`
- [x] `Timeline.tsx` — Replace `text-[0.7rem]` and raw `text-xs` with token utilities
- [x] `Breadcrumbs.tsx` — Replace raw `text-sm` with `text-small`
- [x] `EmptyState.tsx` — Verify spacing, icon size, typography hierarchy
- [x] `Modal.tsx` — Replace `rounded-[var()]` with `rounded-xl`

### Phase 2: Patterns (shared layout)
- [x] `LarkRail.tsx` — Audit spacing consistency in nav items
- [x] `ListPanel.tsx` — Replace raw `text-xs` with `text-caption`
- [x] `ChatListItem.tsx` — Replace raw `text-xs`/`text-sm` with token utilities
- [x] `MessageItem.tsx` — Replace raw `text-xs` with `text-caption`
- [x] `FilePreviewCard.tsx` — Replace raw `text-xs`/`text-sm` with token utilities
- [x] `ChatList.tsx` — Replace raw `text-xs` with `text-caption`
- [x] `ChatInput.tsx` — Replace raw `text-sm` with `text-body`
- [x] `ImagePreviewCard.tsx` — Replace arbitrary text sizes
- [x] `MobileNav.tsx` — Replace gap-0.5 with gap-1

### Phase 3: Chat module
- [x] `channels.$channelId.tsx` — Replace raw `text-xs`/`text-sm`/`text-2xl` with tokens
- [x] `ChatEditor.tsx` — Replace raw `text-sm` with `text-body`
- [x] `MessageContent.tsx` — Replace raw `text-sm` with `text-body`
- [x] `ReactionBar.tsx` — Replace raw `text-xs` with `text-caption`
- [x] `ChannelInfoPanel.tsx` — Replace raw `text-xs`/`text-sm`, standardize section label
- [x] `CreateChannelModal.tsx` — Replace raw `text-sm` with token utilities
- [x] `InviteMemberForm.tsx` — Replace raw `text-sm` with token utilities
- [x] `EmojiPicker.tsx` — Audit spacing, standardize size
- [x] `EditorToolbar.tsx` — Replace `text-[12px]` with `text-caption`

### Phase 4: Drive module
- [x] `drive.tsx` — Replace raw text sizes, replace `text-[0.65rem]` with `text-overline`
- [x] `DriveSidebar.tsx` — Replace raw `text-xs`/`text-sm` with token utilities
- [x] `DriveFileRow.tsx` — Remove inline `style={{ height }}`, replace raw text sizes
- [x] `DriveFileList.tsx` — Replace `text-[0.65rem]` with `text-overline`
- [x] `DriveTreePanel.tsx` — Replace `text-[0.7rem]` with `text-overline`
- [x] `DriveContextPanel.tsx` — Replace arbitrary font sizes with tokens

### Phase 5: Assets module
- [x] `_assets.tsx` — Replace `text-[0.65rem]` with `text-overline`, audit sidebar spacing
- [x] `list.tsx` — Replace `text-[0.65rem]` filter labels, standardize filter buttons
- [x] `dashboard.tsx` — Replace `text-2xl font-bold` → `text-title`, `text-xs` → `text-overline`
- [x] `types.tsx` — Audit grid card spacing consistency
- [x] `requests.tsx` — Audit table header/body typography
- [x] `$assetId.tsx` — Standardize detail panel typography
- [x] `request/new.tsx` — Replace arbitrary text sizes, audit form spacing
- [x] `NotificationBell.tsx` — Replace raw `text-xs`/`text-sm`

### Phase 6: Auth & Workspace shell
- [x] `_auth.tsx` — Replace raw `text-2xl font-bold` with `text-title`
- [x] `login.tsx` — Replace raw text sizes with token utilities
- [x] `OtpInput.tsx` — Standardize input sizing, replace raw `text-sm`
- [x] `_workspace.tsx` — Audit mobile nav spacing, replace raw text sizes
- [x] `documents.tsx` — Remove inline `style={{ height: 36 }}`, replace raw `text-xs`
- [x] `settings.tsx` — Build proper empty state using EmptyState component

### Phase 7: Verification
- [x] `vite build` passes (0 errors)
- [ ] Self-correction checklist at 375px (mobile)
- [ ] Self-correction checklist at 1280px (desktop)
- [x] Zero `text-[arbitrary]` values remain (grep verify) — only 1 color value remains (acceptable)
- [x] Zero raw `text-xs`/`text-sm`/`text-lg`/`text-2xl` in route pages (grep verify)
- [x] Zero `rounded-[var()]` / `duration-[var()]` remain
- [x] Zero `style={{ height: N }}` for fixed rows (replaced with `h-N`)
- [x] All non-scale spacing (0.5/1.5/2.5) eliminated
