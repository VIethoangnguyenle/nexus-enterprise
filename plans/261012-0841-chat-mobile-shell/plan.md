---
title: "Chat điện thoại theo hướng Google Chat + menu ☰"
description: "Thay vỏ điện thoại: ô tìm viên thuốc có ☰ và avatar, menu ☰ sang mọi module, thanh đáy chỉ của Chat (Trang chủ · Trực tiếp · Mục · Thêm) chỉ icon, nút + riêng; Trang chủ là dòng hoạt động có xem trước và dòng trả lời thread; tìm tin trên mọi cuộc trò chuyện."
status: pending
priority: P0
effort: 7-9d
issue:
branch: claude/inspiring-cray-32y4n1
tags: [feature, frontend, backend, mobile, chat]
blockedBy: []
blocks: [261010-2258-pwa-mobile-push]
created: 2026-10-12
---

# Chat điện thoại theo hướng Google Chat + menu ☰

Nguồn: [brainstorm](../reports/brainstorm-261012-0827-chat-mobile-google-style.md); mockup **đã duyệt** 2026-10-12
(`design/mockups/chat-mobile-home.html`, "Chốt nhé, lên plan đi"); DESIGN.md §5 "Điều hướng di động" và §6 "Chat trên
điện thoại" đã cập nhật theo mockup. Mobile là ưu tiên số 1 (người dùng, 2026-10-10).

## Mục tiêu (contract)
- **Outcome:** dưới 1024px mọi màn danh sách có ô tìm viên thuốc (☰ · gợi ý theo module · avatar); ☰ mở ngăn kéo sang
  Chat, Tài liệu, Phê duyệt, Tài sản, Danh bạ, Thông báo, Quản trị, Cài đặt và đổi workspace; Chat có thanh đáy nổi chỉ
  icon (Trang chủ · Trực tiếp · Mục · Thêm) + nút + riêng; Trang chủ/Trực tiếp hiện "Người gửi: đoạn đầu" và dòng trả
  lời thread mới nhất ngay khi mở app; Mục gập/mở có lọc chưa đọc theo mục; tìm người, nhóm và tin nhắn trên mọi cuộc
  trò chuyện.
- **Constraints:** CLAUDE.md §5 (test deny cho mọi nhánh quyền mới, route mới vào `vite.config.js`, migration có
  backup + `make db-migrate` + kiểm trên DB thật, không ID trên UI); xem trước và kết quả tìm chỉ cho kênh người gọi còn
  quyền `read` (cùng `readableChannelIDs` của unread/mentions); giữ nguyên màn trong cuộc trò chuyện
  (`chat-mobile.html`); workspace cá nhân không có Tài sản/Phê duyệt/Quản trị; bố cục ≥ 1024px không đổi.
- **Non-goals:** bố cục máy tính và rail 768–1023; đổi màn luồng/thread/ô soạn; app native; tìm trong nội dung tệp.
- **Acceptance:** khớp mockup ở 390px sáng/tối (ảnh Playwright từng tab, ngăn kéo, tìm, sheet); không cuộn ngang; hàng
  cuối không bị thanh đáy che; mở app lạnh thấy xem trước ở mọi hàng có tin; người bị gỡ khỏi nhóm không nhận xem trước
  hay kết quả tìm của nhóm (test deny Go + kiểm trên stack thật); ☰ có ở mọi màn danh sách của mọi module; Go tests,
  vitest, lint, typecheck, build, `make check-layering`, `make check-ngac` xanh.

## Quyết định
- Thanh đáy chuyên Chat, chỉ icon (người dùng chọn 2026-10-12). Module khác qua ☰ (người dùng yêu cầu thêm).
- Thêm = lối tắt Chat; module không lặp ở Thêm.
- **Mặc định khi người dùng chốt mà chưa trả lời 2 câu mở** (đổi được trước khi làm phase liên quan):
  tìm tin toàn cục **làm trong plan này** (phase 03 + 06, mockup đã vẽ); **768–1023 dùng vỏ điện thoại** như hiện nay
  (`useBottomBarLayout` < 1024) cho tới khi rail được làm.
- Tab Chat trên điện thoại là search param `tab` của `/channels` (`home` mặc định · `direct` · `sections` · `more`),
  máy tính bỏ qua param — không thêm route, không đụng màn máy tính.

## Phases

| # | Phase | Priority | Effort | Depends on | Status |
|---|---|---|---|---|---|
| 01 | [Thiết kế: mockup + DESIGN.md](phase-01-design.md) | P0 | — | — | completed |
| 02 | [API tóm tắt hoạt động + thread theo dõi xuyên kênh](phase-02-activity-api.md) | P0 | 1-1.5d | 01 | completed |
| 03 | [API tìm tin trên mọi cuộc trò chuyện](phase-03-search-api.md) | P1 | 1-1.5d | 01 | pending |
| 04 | [Vỏ điện thoại: ô tìm, menu ☰, sheet tài khoản, thanh đáy Chat, nút +](phase-04-phone-shell.md) | P0 | 2d | 01 | pending |
| 05 | [Tab Trang chủ, Trực tiếp, Mục, Thêm](phase-05-chat-tabs.md) | P0 | 2d | 02, 04 | pending |
| 06 | [Màn Tìm trong chat](phase-06-chat-search-screen.md) | P1 | 1d | 03, 04 | pending |
| 07 | [Kiểm chứng](phase-07-verification.md) | P0 | 0.5-1d | 05, 06 | pending |

```text
01 ──┬─► 02 API hoạt động ─────────┐
     ├─► 04 vỏ điện thoại ──┬──────┴─► 05 tab Chat ──┐
     └─► 03 API tìm ────────┴────────► 06 màn Tìm ───┴─► 07 kiểm chứng
```
02, 03, 04 chạy song song được (khác service/khác file). 05 và 06 cùng sửa `HomeView`/`PhoneChatList` khu vực đầu
danh sách → làm tuần tự 05 rồi 06.

## Capabilities in `docs/specs/`
| Capability | Action | Phase |
|---|---|---|
| `chat-activity-feed` | **new** — tin mới nhất + trả lời thread mới nhất mỗi cuộc trò chuyện đọc được | 02 |
| `chat-threads` | modify — danh sách thread đang theo dõi trên mọi cuộc trò chuyện | 02 |
| `chat-search` | **new** — tìm tin trên mọi cuộc trò chuyện đọc được | 03 |
| `lark-sidebar-layout` | modify — requirement "Phone Navigation And Workspace Switching" → ô tìm + menu ☰ + thanh đáy Chat | 04 |
| `chat-shortcuts` | modify — trên điện thoại hai lối tắt ở tab Thêm | 05 |
| `chat-sections` | modify — tab Mục, lọc chưa đọc theo mục | 05 |

## Liên quan
- `261010-2258-pwa-mobile-push` phase 06 (kiểm trên máy thật) nên chạy **sau** plan này (vỏ điện thoại đổi) → `blocks`.
- `261010-1559-chat-inline-threads` phase 09 ("Trang chủ ưu tiên thread"): Trang chủ mới của plan này thay phần Trang chủ
  trên điện thoại; phần máy tính của phase 09 giữ nguyên.
- `261011-1100-app-rail`: chỉ ≥ 1024, không đụng nhau; dùng chung danh sách điểm đến (phase 04 tách ra `lib/`).

## Ground rules
- `ak:cook --tdd`; mỗi phase backend có test deny; không ID/UUID ra màn hình.
- Không đặt mã plan/phase vào code, test, commit.
- Không commit khi gate đỏ; commit theo phase, đẩy lên `claude/inspiring-cray-32y4n1`.

## Unresolved questions
- (Đã đặt mặc định ở trên) Tìm tin toàn cục làm ngay hay để sau; máy tính bảng dùng vỏ điện thoại hay chờ rail.
