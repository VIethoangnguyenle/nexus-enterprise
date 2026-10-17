---
title: "Gợi ý khi chưa gõ cho ô tìm"
status: in-progress
priority: P2
mode: auto (client only, không có researcher)
blockedBy: []
blocks: []
brainstorm: ../reports/brainstorm-261017-1516-search-suggestions.md
specs: [chat-search, contacts-documents-settings-screens, assets-screens, approval-screens]
---

# Gợi ý khi chưa gõ cho ô tìm

Focus vào ô tìm còn trống của Chat, Tài liệu, Danh bạ, Tài sản hoặc Phê duyệt thì một panel gợi ý hiện ra: "Tìm gần
đây" của module đó và 1–2 nhóm gợi ý theo ngữ cảnh. Gõ chữ thì panel nhường chỗ cho kết quả như hiện nay. Chỉ làm phía
client (phương án A, Zane đã duyệt 2026-10-17): không thêm endpoint, migration, node hay operation NGAC.

## Phases

| # | Phase | Status | Depends |
|---|---|---|---|
| 01 | [Thiết kế: DESIGN.md + mockup](phase-01-design.md) | completed (duyệt 2026-10-17) | — |
| 02 | [Nền: lịch sử tìm theo module + panel gợi ý](phase-02-foundation.md) | completed (review xong, đã sửa H1/M1/M2/M3) | 01 |
| 03 | [Chat + Danh bạ](phase-03-chat-contacts.md) | completed (review xong, đã sửa M1–M3) | 02 |
| 04 | [Tài liệu + Tài sản](phase-04-drive-assets.md) | completed (review gộp với 05) | 02 |
| 05 | [Phê duyệt: ô tìm + gợi ý](phase-05-approval.md) | pending | 02 |
| 06 | [Spec + kiểm tra cuối](phase-06-specs-verify.md) | pending | 03, 04, 05 |

Phase 03, 04, 05 không dùng chung file, có thể làm song song sau phase 02.

## Nguồn gợi ý cho từng module (đều từ query đã qua PEP)

| Module | Tìm gần đây | Gợi ý theo ngữ cảnh |
|---|---|---|
| Chat | có (giữ lịch sử cũ) | Cuộc trò chuyện vừa có tin mới (danh sách conversation đã tải) |
| Danh bạ | có | Người vừa nhắn riêng với mình (DM theo thời gian tin cuối) |
| Tài liệu | có | Tệp sửa gần nhất trong thư mục đang mở; "Được chia sẻ với tôi" (`/api/drive/shared-with-me`) |
| Tài sản | có | Tài sản mình đang giữ; hoạt động gần đây (`/assets/activity`) |
| Phê duyệt | có | Đề nghị chờ mình duyệt (`/pending`); đề nghị của mình (`/my-requests`) |

## Acceptance
- Ở cả 5 module, focus ô trống là panel hiện ra. "Tìm gần đây" xoá được từng mục; chạm một mục thì chạy lại lần tìm đó;
  chạm một gợi ý thì mở đúng thứ đó.
- Có test deny: thứ mình đã mất quyền đọc (không còn trong dữ liệu query trả về) không được gợi ý. Lịch sử chỉ lưu chuỗi
  đã gõ, không lưu tên hay ID của thực thể.
- Đăng xuất xong thì không còn key `ngac-recent-searches:*` hay `ngac-chat-recent-searches:*` nào.
- Lịch sử Chat cũ vẫn còn sau khi đổi sang key mới.
- Không ID nào lên màn hình. Panel dùng được bằng bàn phím (↑/↓, Enter, Esc) và có role listbox đúng chuẩn.
- `npm test`, `npm run lint`, `npm run build` xanh. 4 spec được cập nhật.

## Ràng buộc
- Thiết kế trước (CLAUDE.md §3): code chỉ bắt đầu sau khi phase 01 được duyệt.
- TDD (`ak:cook --tdd`): viết test fail trước.
- TanStack Query giữ server state; lịch sử tìm là localStorage theo từng người dùng, bọc try/catch.

## Non-goals
Typeahead, ⌘K / tìm toàn cục, sửa chính tả, đồng bộ lịch sử giữa các máy, endpoint "sửa gần đây" cho Drive, tìm `?q=`
phía máy chủ cho Phê duyệt.
