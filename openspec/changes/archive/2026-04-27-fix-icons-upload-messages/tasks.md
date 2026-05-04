# Tasks — Fix Icons, Upload & Message Order

## Phase 1: Dependencies & Proxy Fix
- [x] **T1.1** Install lucide-react
- [x] **T1.2** Fix Vite proxy routing
- [x] **T1.3** Verify proxy — all services responding ✓

---

## Phase 2: Drive Upload Fix
- [x] **T2.1** Fix `filename` → `name` field mismatch
- [x] **T2.2** Apply `RequireClaims` to drive REST handlers
- [x] **T2.3** Build drive service ✓
- [x] **T2.4** Verify drive page renders with SVG icons ✓

---

## Phase 3: Icon Migration — AppRail & Core
- [x] **T3.1** AppRail: Lucide icons with per-module color
- [x] **T3.2** AppRail logout: LogOut icon
- [x] **T3.3** ListPanel: FileText, Settings, HardDrive, LayoutDashboard, ClipboardList, Tag, FileEdit
- [x] **T3.4** NotificationBell: Bell icon

---

## Phase 4: Icon Migration — Chat Components
- [x] **T4.1** ChatEditor: Paperclip, Smile, Send icons
- [x] **T4.2** HoverActionBar: MessageSquare, Smile, Pin, MoreHorizontal
- [x] **T4.3** ChannelInfoPanel: Users, Pin, FolderOpen, Settings, Search tabs
- [x] **T4.4** Channel header buttons: Search, Pin, Users, FolderOpen

---

## Phase 5: Icon Migration — Drive, Documents & Assets
- [x] **T5.1** Create `getFileIcon()` utility
- [x] **T5.2** drive.tsx: Lucide file/folder icons
- [x] **T5.3** FilePreviewCard: dynamic file-type icons
- [x] **T5.4** documents.tsx: FileText, Upload, Download icons
- [x] **T5.5** ErrorState: AlertTriangle icon
- [x] **T5.6** _assets.tsx layout: LayoutDashboard, Package, ClipboardList, Tag, Settings, LogOut, ArrowLeft
- [x] **T5.7** Asset dashboard stats: Package, CheckCircle, Clock, Tag
- [x] **T5.8** Asset requests: ClipboardList, Check, X
- [x] **T5.9** Asset types: Tag icon
- [x] **T5.10** Asset list: Package icon
- [x] **T5.11** ChannelDrivePanel: FolderOpen, X, Paperclip
- [x] **T5.12** ChatInput: Paperclip icon
- [x] **T5.13** MessageItem: MessageSquare icon

---

## Phase 6: Message Order & Verification
- [x] **T6.1** `.reverse()` applied to messages array
- [x] **T6.2** Browser verified: icons are all SVG Lucide across modules
- [x] **T6.3** Vite build passes: `✓ built in 586ms`
- [x] **T6.4** Full smoke test: `make run` → all services up → browser verified ✓

---

## Summary

| Phase | Tasks | Status |
|-------|-------|--------|
| 1 | 3 | ✅ Done |
| 2 | 4 | ✅ Done |
| 3 | 4 | ✅ Done |
| 4 | 4 | ✅ Done |
| 5 | 13 | ✅ Done |
| 6 | 4 | ✅ Done |
| **Total** | **32** | **All Complete** |
