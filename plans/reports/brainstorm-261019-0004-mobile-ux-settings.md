---
type: brainstorm
date: 2026-10-19
topic: Mobile UX (installed iPhone PWA) — navigation, module screens, conversation and settings
status: awaiting decision on navigation model
evidence: plans/reports/mobile-ux-audit-261019/ (390×844 screenshots, dev stack on main 36edb19, account duc.tran@example.test, workspace "NovaPay (demo danh bạ)")
---

# Mobile UX brainstorm

## Summary

Audited every module at 390×844 on `main`. The phone app works but reads as the desktop app folded into one
column: moving between modules always costs the ☰ drawer, several screens carry two search boxes and clipped tab
rows, and settings are scattered across four entry points while modules and conversations have almost none.
Recommendation: a Lark-style global bottom bar, one settings model with three scopes (personal, per-module,
per-conversation), and a consistency pass on module headers. Design in `DESIGN.md` + mockups first, then code.

## Contract

- **Outcome:** on an installed iPhone PWA a person reaches any module in one tap, every module screen has the same
  header grammar (one search, visible tabs), each module and each conversation (group and DM) has its own settings
  reached where the person is, and personal settings are one grouped list in the iOS/Lark pattern.
- **Constraints:** design-first (`DESIGN.md` direction "Tín hiệu" + approved mockups before code); desktop layout
  (≥ 1024) unchanged; no raw identifiers on screen; group-management rights (who may post, add people) are NGAC
  decisions, so they need a spec + test vectors under the policy enforcement rules; settings persist server-side
  per user where they already do (`push_preferences`, `channel_notification_levels`, `calendar_user_settings`).
- **Non-goals:** a native app; tablet rail (768–1023 keeps the phone shell); redesign of desktop settings; new
  modules; changing what an operation may do in NGAC beyond the group-management rights named here.
- **Acceptance:** at 390 px every module is one tap from any list screen; no screen shows two search fields; no tab
  row is clipped without an affordance; every module has a "Cài đặt <module>" reachable from its header; group and
  DM info screens carry the Lark set of rows below; personal settings is a grouped list whose rows drill into
  pages; e2e screenshots at 390 px for each module and both conversation kinds.

## Evidence (screens in `mobile-ux-audit-261019/`)

| # | Finding | Where |
|---|---|---|
| 1 | Moving between modules always costs ☰ → drawer → row (2 taps + an overlay). The only bottom bar belongs to Chat, so leaving Chat removes it. | `03-chat-home`, `04-drawer` |
| 2 | Chat bottom bar is icon-only; "Trực tiếp" (bubble) and "Mục" (stack) are not readable without labels. | `03-chat-home` |
| 3 | Two search fields on one screen: top pill "Tìm tài liệu" + "Tìm trong Tài liệu"; "Tìm đề nghị" twice. Tài sản's pill says "Tài sản", not a search. | `06-drive`, `07-approval`, `08-assets` |
| 4 | Tab rows clip with no fade or scroll hint: "Phòng b…", "Loại tài sả…", settings "Thông báo" at the edge. | `07-approval`, `08-assets`, `10-settings` |
| 5 | Subtitles spend a line and truncate: "Đề nghị trong NovaPay (dem…", "Thiết bị, giấy phép và nội thất của NovaPay (d…". | `07-approval`, `08-assets` |
| 6 | Conversation header packs four 44 px icons (gọi, tìm, thread, thông tin) next to the name; name tap does nothing obvious. | `12-conversation` |
| 7 | Group info holds only Thông báo, ảnh, Đổi tên, Đã ghim. No members list, files/links, search, pin/star, mute shortcut, leave, or group management. DM info: Thông báo, "Trò chuyện với", Đã ghim. | `13-group-info`, `SpaceInfoPanel.tsx` |
| 8 | Settings has four entry points with overlapping meaning: drawer "Cài đặt"; account sheet "Cài đặt thông báo", "Cài đặt tài khoản", "Cài đặt"; Chat "Thêm" → "Thông báo của Chat". | `04-drawer`, `17-account-sheet` |
| 9 | Settings page is desktop tabs (Hồ sơ · Workspace · Giao diện · Thông báo). Hồ sơ is a long form with Lưu at the bottom; Giao diện offers "Thu gọn sidebar" (desktop-only); Thông báo only a push switch; Workspace (admin data) sits beside personal tabs; "Đăng xuất" appears twice. | `10-settings` |
| 10 | No module has its own settings (Chat, Lịch, Tài liệu, Phê duyệt, Tài sản, Danh bạ). Calendar has a settings table server-side but no phone entry. | all modules |
| 11 | Workspace picker shows two "Tạo workspace" buttons when the person has none. | `01-home` |

Not verified (headless has no touch): long-press sheets, swipe-back, keyboard-docked composer. The composer captured
is the two-row variant; the phone spec says a single pill with + and a round send. Check on a real iPhone.

## Navigation: options

| | A. Keep ☰ (Google Gmail/Drive style), fix consistency | B. Fixed global bottom bar (Google Workspace app style) | C. Lark-style bottom bar, customizable + "Thêm" |
|---|---|---|---|
| Shape | Drawer stays primary; only findings 2–11 are fixed. | 5 fixed tabs: Chat · Lịch · Tài liệu · Phê duyệt · Thêm. | 4 tabs the person picks (default Chat · Lịch · Tài liệu · Phê duyệt) + "Thêm" sheet with every module, Thông báo, Cài đặt, workspace switch; long-press to reorder. |
| Taps to switch module | 2 + overlay | 1 (4 modules), 2 (rest) | 1 for the 4 a person uses, 2 for the rest |
| Chat's own bar | stays | Chat sub-nav (Trang chủ · Trực tiếp · Mục · Thêm) becomes a segmented row under the title | same as B |
| Depends on | people living in one module | a fixed top-4 fitting every role | people having a stable personal top-4 |
| Fails first when | users hop modules often (ERP) | an approver lives in Phê duyệt + Tài sản | defaults wrong and nobody customizes |
| Cost to abandon | lowest | medium | medium (same as B + a preference) |

**Recommendation: C.** Nexus is an ERP: roles differ (approver, asset keeper, chat-heavy staff), which is why Lark
lets the bar be chosen. C costs one preference beyond B, keeps the drawer's content in "Thêm", and the Chat sub-nav
moves to a segmented control (Google Chat mobile pattern).

## Settings model (Lark/Google pattern, one page per scope, one source per setting)

1. **Cá nhân** (avatar → Cài đặt; replaces the four entry points): grouped list, each row drills in.
   Tài khoản & hồ sơ · Thông báo (chung, giờ yên lặng, then one row per module) · Giao diện (chủ đề, giảm chuyển
   động; no sidebar option on phone) · Ngôn ngữ & khu vực · Thiết bị & phiên · Giới thiệu (phiên bản) · Đăng xuất
   (once). Workspace data moves to Quản trị; "Rời workspace" stays under the workspace switcher.
2. **Module** ("⋯" in each module header → "Cài đặt <module>"; the same page as the module row under Cá nhân):
   - Chat: Enter để gửi, xem trước liên kết, mức thông báo mặc định cho nhóm mới, hiện đã xem.
   - Lịch: ngày đầu tuần, nhắc mặc định, múi giờ, lịch đang hiện (existing `calendar_user_settings`).
   - Tài liệu: chế độ xem mặc định (lưới/danh sách), sắp xếp mặc định, tải ảnh xem trước qua mạng di động.
   - Phê duyệt: thông báo theo loại, uỷ quyền khi vắng (needs backend + spec).
   - Tài sản, Danh bạ: thông báo và chế độ xem; keep minimal.
3. **Cuộc trò chuyện** (tap the header name, as Lark/Google Chat; info icon removed from the header):
   - Nhóm: ảnh · tên · mô tả; Thành viên (list, thêm, vai trò); Tệp · Ảnh · Liên kết; Tin đã ghim; Tìm trong nhóm;
     Thông báo; Gắn sao · Chuyển tới mục (decision 15: no pin-to-top); **Quản lý nhóm** (chỉ người có quyền: ai được gửi tin, ai
     được thêm người, ai được @tất cả, chuyển quyền chủ nhóm); Rời nhóm; Giải tán (chủ nhóm).
   - DM: hồ sơ người (mở thẻ danh bạ); Tệp · Ảnh · Liên kết; Tin đã ghim; Tìm; Thông báo; Gắn sao;
     Tạo nhóm với người này.

## Module header grammar (fixes 3–5)

Top bar keeps the pill search, which **is** the module's search (the in-page search field is removed on phone);
title row: title + one primary action; the subtitle line is dropped on phone (workspace name already in the
switcher); tab rows scroll with an edge fade and the active tab scrolled into view; "⋯" holds Cài đặt <module>.

## Suggested phases (for ak:plan)

1. Design: DESIGN.md §5 "Điều hướng di động" rewrite + mockups (bottom bar C, module header, settings three scopes,
   group/DM settings) — approval gate.
2. Shell: global bottom bar + Thêm sheet + bar preference; Chat sub-nav to segmented row.
3. Module header grammar across the six modules.
4. Personal settings list + module settings pages wired to existing stores; new preference keys where missing.
5. Conversation settings: group + DM screens on existing data (members, files, pins, notifications, leave).
6. Group management rights: NGAC spec + test vectors (deny cases) + backend + UI — policy-model change.
7. Verify on a real iPhone PWA (touch, long-press, keyboard) + 390 px e2e screenshots.

## Decisions (user, 2026-10-19)

1. Navigation: **C**, Lark-style customizable bottom bar + "Thêm".
2. Group management rights (who may post, add people, @all, transfer ownership): **in this round**, as its own
   policy-model phase (NGAC spec + deny test vectors + backend + UI).
3. Phê duyệt "uỷ quyền khi vắng": **later round**; Phê duyệt settings this round are notifications and view only.
4. Bottom-bar default: **one for everyone**, Chat · Lịch · Tài liệu · Phê duyệt; personal workspace swaps Phê duyệt
   for Danh bạ.

5. Blend Lark and Google Chat (2026-10-19): Lark's structure (module bar, customizable, Thêm grid, three-scope
   settings, group settings), Google Chat's form (floating pill bar + separate +, search pill, Home feed, filter
   chips). The bar must stay optimized for Chat: chips replace Chat's sub-tabs, the bar folds to icons on scroll,
   long-press on Chat opens a quick jump, and the bar is gone inside a conversation. Mockup shows A (recommended) and
   B (Chat keeps its own Google bar) for the user to pick.
6. Search for every task (mockup §6): one search everywhere — scope defaults to the open module, "Tất cả" groups
   people, conversations, messages, documents, requests, events, assets, plus matching actions; per-module filter
   chips. Only messages and events have server search today, so documents, approvals, assets and people need
   permission-checked search endpoints (backend work this round).
7. Swipe gestures (mockup §7): app-made edge-swipe back (iOS PWA has none), row swipes (Chat, Tài liệu, Tài sản,
   Thông báo), swipe-to-reply on messages, week/month swipe in Lịch, pull to refresh everywhere, drag down to close.
   Approval rows only reveal Duyệt…/Từ chối… — never act on a full swipe.

8. Two workspace kinds (mockup §8): one shell, different content. Settings split into **Tài khoản** (per person,
   shared by every workspace) and **Trong workspace này**. Personal: bar Chat · Lịch · Tài liệu · Danh bạ, no
   Phê duyệt/Tài sản/Quản trị/phòng ban, workspace name in Tôi, no "Rời". Org:
   Hồ sơ trong tổ chức (admin-set), Phê duyệt, Tài sản, Rời workspace; admins can lock some options (row shows the
   value + who locked it; server enforces).
9. Chat sections stay (mockup §1b): "Mục" is the second Chat chip and can be the opening view; existing behaviour
   kept (Đã gắn sao → your sections → Trực tiếp → Nhóm, fold state shared with desktop, per-section unread-only,
   move-to-section from long-press / swipe / conversation settings, manage sections in Cài đặt Chat).

10. Chat gestures (mockup §9): swipe a message right = quote, left = reply in thread; long-press then slide onto the
    reaction row; double-tap = quick 👍 (setting); drag the stream down to dismiss the keyboard; drag the composer
    handle up for a full-screen composer; image viewer swipe between / drag down to close; edge-swipe leaves a
    thread. Nothing irreversible (delete, edit) is a gesture.
11. Workspace kind is fixed at creation (user, 2026-10-19): **no conversion** personal ↔ org in either direction.
    The existing "Chuyển thành workspace tổ chức" UI is removed this round; the backend conversion path
    (`workspaces.converted_by/converted_at`) is to be retired in the plan (decide: drop route + keep columns, or a
    migration).

12. Drive on the phone (mockup §10, audit `20-drive`, `21-drive-file-menu`, `22-drive-tree`): entry chips Gần đây ·
    Thư mục · Gắn sao · Chia sẻ với tôi · Bản nháp replace the tree drawer (one "shared" entry); Truy cập nhanh
    cards; list view by default on phones (grid keeps real thumbnails only); folder screen with ← up, tappable
    path, sort + view in one row; file sheet with 4 quick actions (Chia sẻ, Sao chép liên kết, Gắn sao, Tải xuống)
    + Gửi vào trò chuyện, Chuyển, Tạo bản sao, Thông tin và hoạt động, Đổi tên, Thùng rác; multi-select with a dark
    action bar replacing the bottom bar; background uploads with a progress card. Starring and Truy cập nhanh need
    backend support; document scanning is a later-round proposal.

13. Bottom bar: **A** (user, 2026-10-19) — one Lark-content / Google-form bar for the whole app; B is dropped.
14. After plan red team (user, 2026-10-19): read-receipt option and its lock are dropped this round (no "seen by"
    screen exists); the only org lock is "Ai được tạo nhóm"; members keep thread replies when posting is restricted.
    A production bug found by the red team (drive copy across workspaces) is fixed separately before Tài liệu work.
    Plan: `../261018-1742-mobile-ux-overhaul/plan.md`.
15. Review fixes (user, 2026-10-19): (a) "Ai được tạo nhóm = chỉ quản trị viên" is implemented by **removing the right**,
    not by a prohibition: `create_channel` leaves the Members → Channels association and every custom role's Channels
    grant, Owners keep theirs, the roles screen refuses to grant it while locked, unlocking restores Members only, writes
    through EPP, lock state read from the graph. (b) The "Ghim" quick action is Gắn sao; no pin-to-top. (c) Quiet hours
    block push except when the person is mentioned by name, `@Tất cả` included. (d) `@Tất cả` in the workspace-wide
    channel is for workspace administrators only and not available in DMs.

## Unresolved questions

1. ~~Which options an org admin may lock (mockup §8)~~ — resolved by decision 14: only "Ai được tạo nhóm" this round.
