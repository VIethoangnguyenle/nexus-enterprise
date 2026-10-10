---
phase: 1
title: "Design: Chat in-line threads, attachments, formatting"
status: done-awaiting-approval
date: 2026-10-10
---

# Phase 01 report: Chat design (design only, no app code)

Files touched: `DESIGN.md`, `design/mockups/spaces.html`, `design/mockups/tokens.css` (+1 token). Nothing committed.

## DESIGN.md
- **§3 Typography:** added `--font-mono` (mirrors the app's existing `frontend/src/index.css` value), used only for code and code blocks inside messages.
- **§6 Components, new subsection "Chat: luồng in-line, thread, ô soạn"** (the old topic-card model only lived in the mockup, so this replaces it as the source of truth):
  - Single model for spaces and DMs. Enter sends to the stream. Thread panel, Thread list and Members share the 360 detail slot.
  - MessageStream row anatomy. **Gom tin:** same sender, ≤ 5 min; explicit list of what breaks a group. Time appears in the gutter on hover or focus.
  - Date dividers (Hôm nay / Hôm qua / weekday / full date). **"Tin mới"** divider: accent line, label on the right. Where a conversation opens: at the end if unread fits one screen, otherwise at the divider + 48px. Read-receipt rule.
  - **JumpPill:** one pill with two states, "Tin mới nhất" ↓ (with count) and "Nhảy tới tin chưa đọc" ↑ (only after the divider was skipped), plus the auto-scroll rules.
  - **ThreadSummary:** ≤ 3 most recent replier avatars, "N trả lời", "Lần cuối hh:mm", "M mới" only when following. Active state when open in the panel; full aria-label.
  - Follow rules: auto on author / reply / mention, manual toggle.
  - **Hover actions:** react, reply in thread, pin, more. More menu for wave 1 = follow toggle + copy text. Wave 3 order reserved: Trích dẫn, Đánh dấu chưa đọc, Sửa (own messages), Xoá (own or space manager). Items from unshipped waves are **not shown**. Touch uses long-press with a bottom sheet.
  - **ThreadPanel:** header toggle Theo dõi / Đang theo dõi (`aria-pressed`, aria-live, no toast), root message, "N trả lời", in-thread "Tin mới", reply composer, drag and drop into the panel.
  - **ThreadsPanel:** header button with badge, radio filters Tất cả / Đang theo dõi / Chưa đọc, item anatomy, back navigation, skeleton / per-filter empty / error copy.
  - Stream loading / empty / error / load-older states.
  - **Composer:** layer order, hint copy, send-disabled reasons. Attach menu (Tải lên từ máy / Chọn từ Tài liệu via Picker, no paths or IDs), paste. **Drop zone** (files only). **AttachmentTray:** thumbnails and chips, progress, remove = cancel, error reason + Thử lại, 20-file cap message, one message on send. **FormattingToolbar:** "A" toggle, 10 buttons in 4 groups, active state, shortcut tooltips, roving tabindex, horizontal scroll under 768. Rendered formatting styles. **LinkDialog:** fields, http(s)/mailto validation, error copy, Enter/Esc.
  - Attachments in a message: 1 / 2 / 3 / 4 / >4 (+N) image layouts, file card by type. **ImageViewer:** scope is one message, prev/next hidden at the ends, ←/→/Esc, focus return.
- **§7 Motion:** 14 new rows using only existing tokens: hover actions, toolbar, popovers, drop zone, tray chip in/out + FLIP, upload progress, jump pill, smooth scroll, "Tin mới" exit, counter and pill changes, panel content swap, viewer open/close, image change. Realtime gains three Chat rules: new main-stream message; a reply in a **followed** thread gets the person-hue wash, halo, "M mới" and "vừa trả lời", and the list item moves up; a reply in a **non-followed** thread is a quiet count update with no wash and no "mới". Reduced-motion line extended.

## Mockup `design/mockups/spaces.html`
- Header intro and TOC updated. Home, Create, Members and Browse are unchanged apart from renumbering to 5, 6, 7 and one copy fix in Create ("trả lời trong thread").
- **§2 rewritten, "Nhóm: luồng in-line và thread":** same shell, navigator and tabs. Shows grouping (×4), dividers "Hôm qua" / "Hôm nay", the "Tin mới" divider, 4 thread summaries (A open, B followed with "2 mới", C not followed, D yours), a 3-image grid, file cards, a pinned label, reactions, a persistent hover bar on Yến's message with the Thêm menu (wave-3 items tagged "đợt 3"), the header Thread button with badge, and the JumpPill. The thread panel is data-driven: the root is cloned from the stream, replies are grouped, there is an in-thread "Tin mới" divider, and the follow toggle works.
- **§3 new, "Panel Thread của nhóm":** 5 threads, working filters, a state switcher for data / loading / empty (per filter) / error, and a toggle that opens and closes the panel.
- **§4 new, "Ô soạn tin":** four panes. (A) toolbar with Bold active, toggles and exclusive groups, link dialog with live validation and insert. (B) tray holding an image paused at 64%, a finished xlsx and a failed pdf with Thử lại; send disabled with a reason shown. Real drag-drop and the file picker also work. (C) drop zone. (D) message display: 1 image, 2 images, 2×2 with +2, three file cards, a rich-text message. Full-screen ImageViewer.
- Mock photos are inline SVG data URIs (paper statements on a desk). Hex colours appear only inside that image data.

## Demos (all clicked by Playwright, 0 console errors)
- §2:
  - Vinh replies in followed thread D: summary becomes "3 trả lời · 1 mới · vừa trả lời" with a person-1 wash and halo; header badge goes 1→2.
  - Ngọc replies in non-followed thread C: count and avatars update, no wash, no "mới".
  - Follow toggle works.
  - JumpPill cycle verified: "Tin mới nhất" → "Nhảy tới tin chưa đọc" → "Tin mới nhất".
  - Clicking B's summary opens B and clears its "mới"; badge goes 2→1.
  - Panel close/open works. Thêm menu works.
- §3: open/close, the 3 filters, the 4 states.
- §4:
  - Toolbar toggle and button states work. Link: an invalid URL shows the error; a valid URL is inserted.
  - "Thêm 3 tệp" runs simulated progress. "Gửi tin kèm tệp" retries the failed file, waits for uploads, then sends **one** message with text + image grid + file cards.
  - Drop zone toggle and attach menu work.
  - Viewer: arrows step 1→3→6, next hides at the end, Esc closes and returns focus to the tile. The viewer also opens on the newly sent grid.
- Reduced-motion toggle exercised.
- 390px: `scrollWidth` = 390 before and after the demos. The app frames scroll inside their own `.scroll` container, as in the existing mockups.

## Screenshots
`/tmp/claude-0/-home-user-nexus-enterprise/a7cd393a-1a02-5aff-964e-4cf3daacb857/scratchpad/chat-design-shots/` (64 PNG, each in light and dark):
- `s2-stream-*-1440`, `s3-threads-*-1440`, `s4-composer-*-1440`
- §2 demo states: `s2-demo-*`, `s2-jump-cycle-*`, `s2-reduced-motion-*`
- §3 demo states: `s3-filter-unread-*`, `s3-state-{empty,loading,error}-*`, `s3-panel-closed-*`
- §4 demo states: `s4-{fmt-off,link-invalid,link-inserted,tray-uploading,tray-sent,attach-menu,drop-hidden,viewer-1,viewer-3}-*`
- 390px: `m390-{top,s2,s3,s4,viewer}-*`

## Open questions
1. **Wash on non-followed thread replies:** I chose no wash (quiet update) so busy spaces don't flicker constantly. This narrows the §7 rule "every change by someone else washes". Keep it, or wash everything?
2. **Header "Thread" button vs. "Thêm người":** with the panel open at 1280–1440 the header is tight. Should "Thêm người" collapse to an IconButton while a detail panel is open?
3. **Send with a failed file:** currently blocked until the user retries or removes it. The alternative is sending only the successful files, which is what Google Chat does in some clients.
4. **ImageViewer background:** it uses `--color-sunk`, so it is light in the light theme. A dark-always viewer would need a new token. Is light acceptable?
5. **"Đồng thời gửi vào nhóm" when replying in a thread:** not designed. The plan's default is no; please confirm.
