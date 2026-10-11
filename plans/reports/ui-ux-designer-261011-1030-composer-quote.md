# Composer: pending quote (Trích dẫn đang chờ)

Mockup: `design/mockups/composer-quote.html` (tokens.css; desktop CSS copied from spaces.html, phone CSS from chat-mobile.html scoped under `.mob` because both files use the same class names at different sizes).
Screenshots: `plans/reports/design-261011-1030-composer-quote/{desktop,phone}-{390,1440}-{light,dark}.png`.

## Verification (Playwright, chromium)
- 390x844 and 1440x900, light and dark: 0 console errors, scrollWidth = clientWidth (no horizontal overflow).
- Esc in the editor removes the strip and focus stays in the editor. Gửi in the "Đã gõ chữ" pane renders the sent bubble with the existing `.q` block.
- Targets: desktop ✕ is 32 visual + `::after` inset -6 = 44 hit. Phone ✕ is 44, and Huỷ on the edit bar is 49x44.

## Decisions
1. **Strip anatomy.** It is the first row of the composer, above the formatting toolbar and the file tray. Background `--color-sunk`, radius 10, min height 44. Quote icon 12 accent + Avatar 20 + name 600, then a one-line excerpt at 13px, muted, ending in "…". It uses the same visual language as the sent QuoteBlock, cut to one line. Pressing the text goes to the original, like the sent block. ✕ IconButton has `aria-label="Bỏ trích dẫn"` and the tooltip "Bỏ trích dẫn (Esc)". The container is `role="region"` with `aria-label="Đang trích dẫn tin của <tên>"` (own message: "tin của bạn", strip shows "Bạn"). On phone it is a raised card with radius 16 above the pill composer (radius 22), and ✕ is 44.
2. **Choosing Trích dẫn.** The strip enters with fade + 4px rise, 160ms (reduced motion: fade only). Focus moves to the editor and typed text is kept. Live announcement: "Đang trích dẫn tin của X. Esc để bỏ." Choosing another message replaces the quote, since there is one quote per composer.
3. **Cancel.** ✕ or Esc. Esc closes popovers, the mention list and the link dialog first. Focus returns to the editor, with the announcement "Đã bỏ trích dẫn". While a quote is pending, ↑ does not open "edit last message" because the composer is not empty.
4. **Send.** A quote alone is not a message. Gửi stays disabled with no text and no files. Sending clears the strip.
5. **Message with no text.** The excerpt becomes an icon 13 + summary: "3 ảnh", the file name (one file), "2 tệp", "Bình chọn: <câu hỏi>", "Công việc: <tiêu đề>". If the message has text, the text is quoted.
6. **Quoted message deleted while pending → send without the quote (not blocked).** The strip turns unfilled with a 1px line, the same look as a deleted bubble. It shows "Tin nhắn đã bị xoá" in italic plus "Tin của bạn sẽ được gửi không kèm trích dẫn.", and it no longer links to the original. Live announcement: "Tin bạn đang trích dẫn đã bị xoá".
   - Why: the spec refuses a non-live target (400), so the quote cannot be kept. Blocking would add a step where the user has nothing to decide, and losing the typed reply would be worse. The visible state warns before sending, so nothing is dropped silently.
   - Race case (delete lands after Gửi is pressed): the server returns 400. The client keeps the text, switches the strip to the deleted state, and the user presses Gửi again. It does **not** auto-resend without the quote.
   - Copy: the brief asked for "Tin này đã bị xoá". I used **"Tin nhắn đã bị xoá"** to match the existing `DELETED_QUOTE` string and the deleted-bubble copy, so one deletion reads the same everywhere.
7. **Thread panel.** The quote goes into the composer where the quoted message lives: a reply in the panel goes to the panel composer. In the panel the strip is still `--color-sunk` (the panel composer is `--color-base`).
8. **Editing + quote.**
   - Desktop: editing happens in place in the bubble, so the composer and its pending quote are untouched. The edit box offers no "Trích dẫn" because editing cannot change a sent message's quote.
   - Phone: the composer is shared, so there is **one context bar at a time**. "Đang sửa tin" (pen icon, the original text excerpt, Huỷ, 2px accent inner ring) takes the strip's place. The pending quote and draft are set aside, with the note "Trích dẫn tin của X sẽ quay lại sau khi sửa xong." Lưu or Huỷ restores them exactly.
   - While editing on phone: "Trích dẫn" is hidden from the long-press sheet, the + button is disabled (editing adds no files), and the send button's label is "Lưu".
   - So quoting is effectively disabled during a phone edit, and coexists on desktop.

## Proposed DESIGN.md paragraph (not applied; under §6 Chat, after "Ô soạn")
- **Trích dẫn đang chờ:** chọn "Trích dẫn khi trả lời" (menu Thêm) hoặc "Trích dẫn" (nhấn giữ) thì hàng đầu của ô soạn (trên thanh định dạng và hàng chờ tệp) hiện dải trích dẫn: nền `--color-sunk`, radius 10, cao tối thiểu 44, icon trích dẫn 12 `--color-accent` + Avatar 20 + tên 600 + một dòng trích 13px mờ cắt "…" (tin không có chữ: icon + "3 ảnh" / tên tệp / "2 tệp" / "Bình chọn: …" / "Công việc: …"); bấm dải = tới tin gốc; IconButton ✕ "Bỏ trích dẫn" (vùng chạm 44). Dải là `region` "Đang trích dẫn tin của <tên>" ("của bạn" với tin mình). Chọn xong focus vào ô nhập; ✕ hoặc Esc (sau popover, gợi ý nhắc tên) bỏ trích dẫn và trả focus. Mỗi ô soạn một trích dẫn, chọn tin khác thì thay; trích tin trong panel thread thì vào ô soạn của panel. Trích dẫn không tự thành tin (Gửi vẫn cần chữ hoặc tệp); có trích dẫn chờ thì ↑ không mở sửa tin. Tin được trích bị xoá khi đang chờ: dải không nền, viền 1px `--color-line`, "Tin nhắn đã bị xoá" nghiêng + "Tin của bạn sẽ được gửi không kèm trích dẫn.", Gửi gửi không kèm trích dẫn; nếu xoá trúng lúc đang gửi thì giữ chữ, dải chuyển trạng thái đã xoá, người dùng gửi lại. Điện thoại: dải là thẻ radius 16 trên ô soạn, ✕ 44; khi sửa tin, thanh "Đang sửa tin" (viền trong 2px accent, Huỷ) thay chỗ dải, trích dẫn và bản nháp được cất và quay lại sau Lưu/Huỷ; trong lúc sửa sheet nhấn giữ không có "Trích dẫn", nút + tắt, nút gửi là "Lưu". Mockup `design/mockups/composer-quote.html`.

## Notes for implementation (read-only observations)
- The sent `QuoteBlock` renders `quote.text ?? ''`, so a quote of a message with only attachments or a card shows an empty second line today. The same summary rule (decision 5) should apply to the sent block. The server returns only "sender name + opening of text", so it would need to say what the attachments are (count/type) or the card title.
- The "Đang sửa tin" phone edit bar was described in DESIGN.md but not drawn anywhere before. It is drawn here.

## Open questions
1. Is the pending quote saved with the conversation's draft when switching conversations or reloading? Drafts are not specified in DESIGN.md yet.
2. Does the backend return enough for the attachment/card summary in `QuotedMessage` (counts, types, card title)? If not, does this become a spec change to chat-message-lifecycle?
3. Copy: confirm "Tin nhắn đã bị xoá" (consistent) instead of the brief's "Tin này đã bị xoá".
4. Can you quote a main-stream message from inside an open thread panel (the server allows it, same channel)? This mockup routes the quote to the surface where the message lives. Confirm.
