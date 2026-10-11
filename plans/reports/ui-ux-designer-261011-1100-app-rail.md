# Menu chính: mở rộng / dải icon (app rail) — design report

Date 2026-10-11. Mockup: `design/mockups/app-rail.html`. Screenshots: `plans/reports/design-261011-1100-app-rail/` (28 frames = 7 states × 1280/1440 × light/dark, plus `1280-light-1-after-toggle-expanded.png`).
Decision change mid-task: "always compact" → **collapsible**: Thu gọn (dải 72px, icon + nhãn) ⇄ Mở rộng (232px, đúng sidebar hôm nay). Mockup follows the latest decision.

## Decisions

| Topic | Decision | Why |
|---|---|---|
| Rail width | **72px**, padding 12/4 → item 64 wide | 64 fits the longest label ("Thông báo", "Phê duyệt" at 11px/500, measured: no label overflows) + "99+" badge stays inside the column |
| Item | link 64×56; icon 20 (lucide, stroke 1.75) in a 48×28 highlight, radius 10; label 11px/14 under it, gap 4 | Same grammar as phone tab bar (§5: icon in 48×24 accent-wash r10, 11px label) → one nav language across devices. Radius 10 = surface radius rule; a full 999 pill is reserved for badges/avatars in §4 |
| Active | highlight `--color-accent-wash`, icon `--color-accent`, label `--color-ink` 600, `aria-current="page"` | Label in ink (not accent) keeps 11px text ≥ 4.5:1 on sunk in both themes |
| Hover | highlight `--color-hover`, label ink | |
| Labels | **No shortening needed.** Tin nhắn · Tài liệu · Phê duyệt · Tài sản · Danh bạ · Thông báo · Quản trị · Cài đặt all fit 64px (Playwright check `scrollWidth ≤ clientWidth`). Visible label = accessible name. Safety net: `text-overflow: ellipsis` + tooltip only if a future label overflows | Tooltip duplicating a visible label is noise |
| Count badge | pill 16px on icon top-right (`left: 50% + 4px`, top 2), 10px/600 tabular, ring 2px `--color-sunk`, `99+` above 99, hidden at 0. SR text ", 5 chưa đọc" / ", 3 chờ bạn" inside the link | Same as existing rail spec; grows rightward so 99+ stays inside 72 |
| Workspace switcher | rail: square logo 40 (r10, accent, initial, display font) in a 48 button; opens the existing **Đổi workspace** menu to the right (left 80, top 12). Menu header = open workspace **full name, wraps, never truncated** + subline (org name or "Workspace cá nhân" · role); then "Chuyển sang" list (names wrap too) + Tạo workspace. aria-label "Đổi workspace, đang ở {tên} ({dòng phụ})", tooltip name + subline | No truncated names in the rail; one menu for both states |
| Toggle | IconButton 32 (icon khung có cột trái) **alone on the top row in both states**, centre x = 36 so it never moves under the pointer. `aria-expanded` (true = mở rộng), aria-label "Thu gọn menu chính" / "Mở rộng menu chính", tooltip with shortcut, `aria-keyshortcuts="Control+Backslash"` | Old spot (end of workspace row, §5) cut the workspace name to ~88px ("Không gi…") and jumped position between states |
| Shortcut | **Ctrl+\\ (⌘\\ on macOS)** | Ctrl+B (VS Code) is Bold in the composer; Ctrl+\\ is free in browsers and used by Notion for the same job. Cheap, discoverable via tooltip, power users toggle when reading long threads |
| Bottom | Thông báo · Quản trị (khi có quyền) · Cài đặt as rail items, then person. Rail: avatar 32 + presence in a 48 button → account menu (avatar 40, name, role · Đang hoạt động, Cài đặt tài khoản, Đăng xuất), opens right/bottom-aligned. Expanded: today's row (avatar, name, role, IconButton Đăng xuất) | Rail has no room for name + logout; menu keeps Đăng xuất one click away |
| Thông báo | included in the foot per DESIGN.md §5 (NotificationPanel); **not yet in `AppSidebar.tsx`** | DESIGN.md is source of truth; see open question 3 |
| Tooltip | only where text is hidden: switcher, avatar, toggle (rail); nothing on items with visible labels. `--color-overlay` + `--shadow-overlay`, radius 8, 12px/500 (title 13/600), 8px right of target, `--z-tooltip`; hover 400ms, keyboard focus immediate, Esc dismisses (WCAG 1.4.13); fade+4px 160ms, reduced motion: no animation. Never the only label source: every target has aria-label | DESIGN.md has `--z-tooltip` but no tooltip spec |
| Focus | 2px `--color-focus` ring offset 2 around the 64×56 item (drawn in §3) | |
| Landmark | whole column is `<nav aria-label="Điều hướng chính">` (today: `<aside>` + inner nav "Phân hệ") | Brief requirement; one landmark, not two |
| Transition | contents fade out 100ms `--ease-in` → `grid-template-columns` 232⇄72 280ms `--ease-out-expo` → contents fade in 120ms `--ease-out`. Reduced motion: instant swap | Row and column layouts can't morph; crossfade hides the reflow. Uses §7 tokens |
| Memory | per person **and** device (localStorage key scoped by user id; today `prefs.sidebarCollapsed` — check whether it's server-synced) | User decision |
| **Default** | **Thu gọn** at every desktop width, one choice for all modules | Rail keeps labels so new users still read every destination; at 1280 with thread panel open the expanded layout leaves 400px for the stream, under the §5 minimum 480. Per-module default rejected: nav jumping width on every module switch is disorienting |
| Hover peek | **removed** ("Xem tạm" in §5) | Existed because the 64px rail had no labels |
| Phones (< 1024) | **unchanged**: top bar + bottom tab bar 3 tabs + Thêm; no toggle | |

## Chat stream width (measured in Playwright, Tin nhắn with 288 list)

| Window | Expanded | Collapsed | Gain | + thread panel 360, expanded / collapsed |
|---|---|---|---|---|
| 1024 | 504 | 664 | +160 (32%) | panel overlays |
| 1280 | 760 | 920 | +160 (21%) | **400 (< 480 min)** / 560 |
| 1440 | 920 | 1080 | +160 (17%) | 560 / 720 |

1024–1279: same two states, same default (collapsed), detail panel stays an overlay as §5 says.

## Verification

Playwright (frontend's dependency, Chromium), 1280×800 and 1440×900, light + dark: 0 console errors, page `scrollWidth == clientWidth`, no side/content/stream overflow in any frame, no rail label overflow, no badge outside the rail. Toggle by click and by Ctrl+\\ flips `.rail`, `aria-expanded`, aria-label. Screenshots reviewed visually.

Mockup notes: frames are fluid (frame = real shell width at that window). Shell/nav/stream/composer/menu CSS copied from `spaces.html`, switcher menu from `personal-workspace.html`; the rail block and tooltip are the only new CSS. The person's avatar in the expanded state is a `display: contents` button in the mockup (not interactive in today's app) — implementation can keep it non-interactive when expanded.

## DESIGN.md text proposal (not applied)

Replace the **Thu gọn sidebar (rail 64px)** bullet of §5 with:

> - **Menu chính: mở rộng hoặc dải icon** (mockup `design/mockups/app-rail.html`). Hai trạng thái, người dùng tự
>   chọn, nhớ theo người trên từng máy; **mặc định thu gọn** ở mọi bề rộng máy tính và một lựa chọn chung cho mọi
>   phân hệ. Cả cột là `<nav aria-label="Điều hướng chính">`.
>   - *Nút chuyển:* IconButton 32 (icon khung có cột trái) nằm riêng hàng trên cùng ở cả hai trạng thái, tâm cách
>     mép trái 36px nên không đổi chỗ. `aria-expanded` (true = mở rộng), `aria-label` "Thu gọn menu chính" /
>     "Mở rộng menu chính", phím tắt **Ctrl+\\** (⌘\\ trên macOS, `aria-keyshortcuts`), ghi trong tooltip.
>   - *Mở rộng (232px):* như hiện nay: workspace switcher (logo 32 + tên + dòng phụ + chevron), NavRow, đáy
>     Thông báo · Quản trị · Cài đặt · hàng người dùng có nút Đăng xuất.
>   - *Thu gọn (72px, đệm 12/4):* ô chữ cái workspace 40 (radius 10) trong nút 48; mỗi mục là link 64×56: icon 20
>     trong nền 48×28 radius 10 + nhãn 11px/500 bên dưới (đủ tên, không viết tắt). Đang chọn: nền
>     `--color-accent-wash`, icon `--color-accent`, nhãn `--color-ink` 600, `aria-current="page"`; hover nền
>     `--color-hover`. Số đếm: pill 16 ở góc trên phải icon, viền 2px `--color-sunk`, `99+`, ẩn khi 0; chữ ẩn
>     ", N chưa đọc" trong link. Đáy: Thông báo · Quản trị · Cài đặt, rồi avatar 32 (presence) mở menu tài
>     khoản: tên, vai trò, trạng thái, Cài đặt tài khoản, Đăng xuất.
>   - *Đổi workspace:* bấm ô chữ cái (hay tên khi mở rộng) mở cùng một menu; đầu menu ghi đầy đủ tên workspace
>     đang mở (xuống dòng, không cắt) + dòng phụ ("Workspace cá nhân" hoặc tên tổ chức) · vai trò; danh sách
>     "Chuyển sang" cũng xuống dòng thay vì cắt. Workspace cá nhân không có Tài sản, Phê duyệt, Quản trị.
>   - *Tooltip* chỉ ở chỗ chữ bị ẩn (ô workspace, avatar, nút chuyển): `--color-overlay`, `--shadow-overlay`,
>     radius 8, 12px, cách 8px bên phải, `--z-tooltip`; trỏ chuột 400ms, focus bàn phím hiện ngay, Esc ẩn.
>     Tooltip không bao giờ là nguồn tên duy nhất.
>   - *Chuyển trạng thái:* nội dung cột fade ra 100ms `--ease-in`, bề rộng cột 280ms `--ease-out-expo`, nội dung
>     fade vào 120ms; giảm chuyển động thì đổi ngay. Không còn "xem tạm" khi trỏ chuột vào dải.
>   - Dưới 1024 không có menu này: thanh trên + thanh đáy như dưới đây.

Also in §5: the layout diagram "Sidebar 232px (rail 64px)" → "(dải 72px)"; breakpoint line "768–1023 sidebar thành rail" → keep as the phone bar ("không đổi") or reword to say that the 72px rail is desktop-only. Add a **Tooltip** bullet to §6 with the spec above.

## Open questions

1. **Default state**: proposed *thu gọn* everywhere on desktop. Alternative: mở rộng ≥ 1280, thu gọn 1024–1279 (today's §5 default). Your call.
2. **Shortcut Ctrl+\\**: keep, or no shortcut?
3. **Thông báo** is in DESIGN.md §5 but not in `AppSidebar.tsx` yet: draw it in the rail now (as mocked) or leave it out until NotificationPanel ships?
4. Rail highlight shape: radius 10 (matches phone tab bar, as mocked) vs a full stadium pill like Google Chat (would also change the phone bar for consistency).
5. Is `prefs.sidebarCollapsed` server-synced? The decision says "per person on this device", which suggests localStorage keyed by user.
6. Switcher menu names now wrap instead of ending in "…" — apply the same to the phone sheet (`core-screens.html`)?

Status: DONE
Summary: Mocked both sidebar states (72px icon+label rail and today's 232px sidebar) with toggle, switcher/account menus, personal variant, tooltip/focus, in `design/mockups/app-rail.html`; verified at 1280/1440 light/dark with zero console errors or overflow. Recommended default is collapsed (+160px chat stream; avoids the 400px stream at 1280 with a thread open), pending the user's decision.
