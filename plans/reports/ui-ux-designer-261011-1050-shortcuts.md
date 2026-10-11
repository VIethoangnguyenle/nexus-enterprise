# Lối tắt "Được nhắc tới" + "Đã gắn sao": design report

Date: 2026-10-11 · Mockup: `design/mockups/shortcuts.html` · Screenshots: `plans/reports/design-261011-1050-shortcuts/` (52 files, 1440 + 390, light + dark)

## What was designed
1. **Được nhắc tới view** (desktop main pane, phone full screen): date-grouped list of mention cards, newest first; unread = ink text + 600 names + 8px accent dot + sr "Chưa đọc"; read = ink-muted, no dot. Card: conversation icon 24 + name · (replies) thread icon + "Trả lời trong thread của X: “root…”" · Avatar 32 + sender + time · bubble, 3-line clamp. Your @mention = accent 600 on accent-wash, radius 4; other @mentions accent text only (same as stream). States: skeleton 4, empty, error, "Tải thêm" paging (25), end line, realtime insert with person wash.
2. **Open behaviour**: main message → conversation, message scrolled to centre, bubble flashes 2px accent ring for 2.4s (reuses push.html `hl`). Reply → conversation scrolled to the root, thread summary `aria-expanded`, thread panel opens, reply flashes in panel. Phone: stream / full-screen thread slides up, back returns to list position. Opening = read (optimistic), shortcut count drops.
3. **Đã gắn sao view**: part "Cuộc trò chuyện" (navigator `.rl` rows + always-visible 28px star toggle) and part "Tin nhắn" (mention-style cards, no unread state, star toggle top-right outside the link). Per-part mini empty states, combined empty, loading, error. Unstar → row/card collapses 220ms + toast with Hoàn tác.
4. **Starring UI**: row ⋯ menu, header name-menu (from notification-levels.html), phone long-press sheets, Thêm menu ("Gắn sao tin" second, after Theo dõi thread), message long-press sheet. Star mark: 12px accent after time on messages (`role="img"`, "Bạn đã gắn sao"), 16px after title in conversation header. Toasts for all four outcomes + error.
5. **Shortcut count**: accent pill on "Được nhắc tới".
6. **Phone**: Tin nhắn tab = Trang chủ, so the other two shortcuts become two 64px tiles (icon + count / label) at the top of the list.

## Decisions (and why)
- **Starred = system section in the navigator**, directly under the shortcuts, above user sections (Google Chat). Hidden when empty, collapsible with count pill, no ⋯ / +. A starred conversation shows **only** there, keeping DESIGN.md's "one conversation, one place" rule; it keeps its section membership and returns there on unstar. Row ⋯ of a starred row = Bỏ gắn sao · Tắt thông báo only (no "Chuyển tới mục"; unstar first). Rejected: duplicating rows (breaks the one-place rule, double unread counts); star-removes-from-section (destroys user organisation silently).
- **Empty user section** after all its rows got starred shows the existing `.sec-hint` text, not a blank header.
- **No per-row star in the navigator Starred section** (header says it); star marks appear where state isn't implied: conversation header, messages, search results.
- **Count, not dot** on Được nhắc tới: mentions address you by name, so the number is useful; it is a subset of Trang chủ's count, never added to the Tin nhắn tab total. Đã gắn sao has no count.
- **Viewing the list doesn't mark read**: marking from a list would push the conversation read receipt past unseen messages. Only opening marks read.
- **No star button on the hover bar**: bar already has 4 (5 for own); starring is personal and rarer than react/reply; Thêm menu + long-press sheet are enough. Revisit if usage shows otherwise.
- **"Tải thêm" button, not infinite scroll**: consistent with NotificationPanel; keeps keyboard/screen-reader position stable.
- **Starred messages ordered by `starred_at`**, shown with message time (§8 format), no date dividers (would mix message date with starred date).
- Message-star toast offers **Xem** (discoverability of the view) instead of Hoàn tác; unstar toasts offer Hoàn tác.

## DESIGN.md paragraph proposal (for the main session to apply under "Chat: cột điều hướng và mục")
> - **Lối tắt Được nhắc tới / Đã gắn sao** (mockup `design/mockups/shortcuts.html`): "Được nhắc tới" có pill đếm = số tin nhắc tên bạn chưa đọc (99+); là tập con của Trang chủ, không cộng vào tab Tin nhắn. Màn Được nhắc tới: mọi tin (tin chính và trả lời trong thread) nhắc tên bạn ở các cuộc trò chuyện bạn đọc được, mới nhất trên, chia theo vạch ngày; mỗi mục là liên kết: icon cuộc trò chuyện 24 + tên 13px · (trả lời) "Trả lời trong thread của X: “tin gốc…”" 12px mờ · Avatar 32 + tên + giờ · bong bóng rút gọn 3 dòng; @tên bạn là chữ accent 600 trên `--color-accent-wash` bo 4 (cả trong luồng). Chưa đọc: chữ `--color-ink`, tên 600, chấm 8 accent + chữ ẩn; đã đọc: `--color-ink-muted`. Xem danh sách không đánh dấu đã đọc. Bấm: mở cuộc trò chuyện, tin vào giữa, bong bóng loé viền accent 2,4s; trả lời thì mở thêm panel thread và loé trong panel. 25 mỗi trang + "Tải thêm"; Skeleton 4; Rỗng "Chưa ai nhắc tên bạn…" + "Về Trang chủ"; Lỗi + "Thử lại"; tin nhắc mới chèn đầu với lớp phủ màu người.
> - **Đã gắn sao**: gắn sao là của riêng bạn, áp cho cuộc trò chuyện và từng tin. Cột điều hướng có mục hệ thống **Đã gắn sao** ngay dưới lối tắt (ẩn khi trống, thu gọn được, không ⋯/+); cuộc trò chuyện đã gắn sao chỉ hiện ở đó, vẫn nhớ mục cũ và về lại khi bỏ sao; menu ⋯ của nó chỉ có Bỏ gắn sao · Tắt thông báo. Gắn/bỏ sao cuộc trò chuyện: menu ⋯, menu tên ở đầu cuộc trò chuyện (đã gắn thì sao 16 accent sau tên), sheet nhấn giữ. Gắn sao tin: menu Thêm (sau Theo dõi thread) và sheet nhấn giữ; tin đã gắn có sao 12 accent sau giờ. Màn Đã gắn sao: phần Cuộc trò chuyện (hàng như cột điều hướng + IconButton sao `aria-pressed`) và phần Tin nhắn (thẻ như Được nhắc tới, xếp theo lúc gắn sao), mỗi phần có trạng thái rỗng riêng. Mọi thao tác lạc quan; toast "Đã gắn sao “…”"/"Đã bỏ gắn sao …" + Hoàn tác, gắn sao tin + "Xem"; lỗi hoàn lại + toast lỗi + Thử lại.
> - **Điện thoại**: tab Tin nhắn là Trang chủ nên hai lối tắt còn lại là hai ô cao 64 (icon + pill đếm / nhãn) đầu danh sách; hai màn mở toàn màn có Quay lại.

Also: in "Chat trên điện thoại" long-press-message list add "Gắn sao tin / Bỏ gắn sao tin" after "Theo dõi / Bỏ theo dõi thread"; in "Hover actions" Thêm menu order add "Gắn sao tin / Bỏ gắn sao tin" after "Theo dõi thread".

## Backend contract notes
- Count badge needs unread-mention total. Unread list already reports mentions per conversation (commit c9829c8); sum client-side **only if** it includes thread-reply mentions. Otherwise add a total to `/api/messages/mentions` or the unread list.
- `GET /api/messages/mentions` items need: message (sender display name, text, created_at, parent/root id + root sender + root excerpt for replies), channel (name, kind, colour/initial source), `unread`. Cursor paging, 25.
- `GET /api/messages/starred` same plus `starred_at`; attachment count for the "1 tệp" line.
- Starred conversation must keep its section membership (`starred` independent of `section_id`).

## Verification
Playwright (frontend's playwright, chromium) at 1440×900 and 390×844, light + dark: 0 console errors, no horizontal overflow (scrollWidth == innerWidth), no phone target under 44px (tab bar / emoji row excluded, both pre-existing), no UUID-like text. Exercised: realtime insert, open main mention, open thread reply, paging, empty/loading/error, unstar message + toast, row menu star → section move, header menu, Thêm menu (opens upward near bottom), star message, phone long-press sheets, phone open + flash. One earlier run logged one console error that did not reproduce in 3 later runs; most likely the Google Fonts fetch.

## Not touched
DESIGN.md, app code, app-rail.html, type-scale.html, file-viewer.html. No commit.

## Unresolved questions
1. Does `@mọi người` / @all (if it exists) count as a mention of you? Mockup assumes only direct @you.
2. Muted conversations: notification-levels says mentions still surface (@ pill). Assumed they appear here and count in the badge. Confirm.
3. Starring a deleted message, or losing access to a starred conversation: assumed the server drops it from lists. Should the star row be deleted, or kept and just hidden?
4. Should "Được nhắc tới" get "Đánh dấu tất cả đã đọc" or an "Chưa đọc" filter later? Left out (not in contract).
5. Navigator Starred section order: same order rule as other sections, or by `starred_at`? Mockup uses most recently starred first.
6. Is the decision "starred hides from its user section" OK with the user? It is a Google Chat parity call, but it does change where a sectioned conversation shows.
