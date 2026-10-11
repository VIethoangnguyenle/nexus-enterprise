# lark-messaging-layout

## Purpose
Maximise readable message density and let a thread be read alongside its channel rather than instead of it.

## Status

**Superseded 2026-10-11 by in-line threading** (`chat-threads`, plan `261010-1559-chat-inline-threads` phase 04).
The topic-card layout this spec described is gone: `TopicStream` was replaced by `ConversationStream`, a
chronological two-sided stream with a thread-summary row under each message. What survives is the second
requirement (a thread opens beside the channel, never instead of it); the first (dense list without bubble
backgrounds) is replaced by the bubble layout in DESIGN.md "Chat: luồng in-line". The history below is kept as
the record of what was verified before the switch.

Verified against code 2026-07-31. Thread peek panel exists (`routes/_workspace/channels.$channelId.tsx`).
The "dense, no bubble backgrounds" requirement is a visual property that static inspection cannot
confirm or refute — it needs a human or a visual-diff check.

**Bổ sung 2026-08-01 — di cư design system.** Câu trên vẫn đúng, nhưng giờ đã có phương pháp trả
lời nó: đọc `getComputedStyle` trên trình duyệt, hoặc khi không mở được trình duyệt thì so vị trí
byte của hai selector chọi nhau trong `dist/assets/*.css` — thứ tự trong stylesheet đã build chính
là thứ quyết định class nào thắng. Cách này đã tìm ra **hai khuyết tật trong chính module chat mà
lint không thể thấy**, vì `className` nằm trên component đã là primitive nên không có thẻ thô nào
để rule bắt: nút gửi reply của `ThreadPanel` render trong suốt và vuông thay vì tròn màu primary,
và màu đỏ khi hover nút xoá thành viên ở `ChannelInfoPanel` chưa từng hiện lần nào. Cả hai đã sửa.

`patterns/MessageItem.tsx` đã bị xoá — không còn nơi nào import, `components/chat` đã thay thế.
Yêu cầu "không nền bong bóng" vẫn **chưa được kiểm chứng bằng mắt**; đợt này chỉ chứng minh được
rằng các class liên quan thực sự thắng trong cascade, không chứng minh được thiết kế đúng ý.

## Requirements

### Requirement: Dense Chat View (superseded)
~~The messaging view SHALL display messages in a high-density list without chat bubble backgrounds or gradients.~~
Replaced 2026-10-11: groups and direct messages show messages on two sides in bubbles (yours on the right in the
accent wash, others on the left with avatar and name), grouped by sender within five minutes. See `chat-threads`
and DESIGN.md "Chat: luồng in-line, thread, ô soạn".

### Requirement: Thread Peek Panel
Threads SHALL open in a dedicated right-side peek panel rather than overlaying the main view or navigating to a new page, allowing simultaneous context of the main channel and thread. Below 1024px the panel is a full-screen sheet.

#### Scenario: Opening a thread
- **WHEN** user clicks "Trả lời trong thread" on a message, or its thread-summary row
- **THEN** a side panel SHALL slide in from the right to display the thread
