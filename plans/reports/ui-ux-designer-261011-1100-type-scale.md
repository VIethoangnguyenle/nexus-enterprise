# Đề xuất cỡ chữ chat: to hơn Google Chat một chút (2026-10-11)

Yêu cầu: "So với cỡ chữ Google Chat thì Nexus cần chữ to hơn 1 xíu". Chỉ là đề xuất. Chưa sửa code app hay DESIGN.md, chưa commit.

- Mockup: `design/mockups/type-scale.html`. Hai bản "Hiện tại" và "Đề xuất" dùng cùng markup và chỉ khác biến cỡ chữ. Dòng số đo dưới mỗi khung là `getComputedStyle` lấy thẳng từ trình duyệt. Có nút chuyển Theo máy/Sáng/Tối.
- Ảnh chụp: `plans/reports/design-261011-1100-type-scale/type-scale-{1440,390}-{light,dark}.png`
- Đã kiểm tra bằng Playwright ở 1440x900 và 390x844, cả sáng và tối: không có lỗi console, `scrollWidth == clientWidth` (không tràn ngang). Ở 390, khung desktop cuộn ngang trong vùng `.scroll` của nó, đây là chủ ý. Chiều cao hàng đo được: điều hướng 36, tóm tắt thread 32/36, hàng danh sách trên điện thoại 64. Ba con số này không đổi giữa hai bản.

## 1. Hiện tại (code đang chạy, không phải mockup)

| Vai trò | px (cỡ / dòng) | Nguồn |
|---|---|---|
| Tin nhắn, máy tính | 14 / 21,7 (1.55) | `chat/MessageContent.tsx` `text-sm leading-[1.55]` |
| Tin nhắn, điện thoại | **14** / 21,7 | cùng class. DESIGN.md và chat-mobile.html ghi 15, **code lệch** |
| Tên người gửi | 14 / 600 (kế thừa body) | `spaces/MessageBlock.tsx` |
| Giờ (đầu cụm và giờ ở lề) | 12 / 16 | `text-xs` |
| Nhãn "đã sửa" | 11 | `text-2xs` |
| Hàng điều hướng (cao 36) | 14 | `spaces/ChatNavigator.tsx` `h-9 text-sm` |
| Tiêu đề mục | 13 / 600 | `ChatNavigator`, `PhoneChatList` `text-small` |
| Hàng tóm tắt thread | 13 / 18 | `spaces/ThreadSummary.tsx` `text-small` |
| Trích dẫn trong tin | 13 / 1.375 | `spaces/QuoteBlock.tsx` `text-small leading-snug` |
| Code trong tin | 13 mono | `.message-html code` 0.8125rem |
| Ô soạn | 16 / 24 | `.chat-editor-content` (index.css) |
| Hàng danh sách điện thoại: tên / xem trước / meta | 14 / 14 / 12 | `spaces/ConversationRow.tsx`; avatar 40 (DESIGN.md ghi 44) |
| Tiêu đề kênh / dòng phụ | 19 display / 13 | header SpaceView |
| Chữ giao diện, bảng, mô tả | 14 / 21 | `text-sm`, body 0.875rem/1.5 |
| Chữ nhỏ / caption | 13 / 18, 12 / 16,8 | `text-small`, `text-caption`, `text-xs` |

## 2. Mốc so sánh: Google Chat web (ước lượng)

Các số dưới đây **là ước lượng từ hiểu biết chung về giao diện Material 3 hiện tại** (Google Sans Text, trước đây là Roboto). Tôi chưa đo trên phiên đăng nhập thật.

- Tin nhắn: 14 / 20. Tên người gửi: 14 / 500. Giờ: 12.
- Hàng điều hướng: 14 (hàng cao khoảng 32). Tiêu đề mục: khoảng 14 / 500.
- Ô soạn: 14 / 20. "N câu trả lời": khoảng 12–13.
- App Android: tin nhắn khoảng 15–16sp, tuỳ cỡ chữ hệ thống.

Kết luận: Nexus hiện **ngang** Google Chat ở tin nhắn, tên và hàng điều hướng. Tiêu đề mục và hàng thread của Nexus còn **nhỏ hơn**. Chỉ có ô soạn là to hơn.

## 3. Đề xuất

Nguyên tắc: tăng 1px ở những chỗ đọc nhiều. Tin nhắn trên điện thoại tăng 2px để bằng ô soạn 16. Không đổi `text-sm` toàn cục, nên bảng, form và trang quản trị giữ 14.

### Token (`frontend/src/index.css` `@theme`, `design/mockups/tokens.css` không có cỡ chữ)

```css
/* thêm, các token khác giữ nguyên */
--text-md: 0.9375rem;               /* 15px */
--text-md--line-height: 1.4375rem;  /* 23px = 1.53, ≥ 1.5 cho dấu chồng tiếng Việt */
```

### Bảng thay đổi

| Vai trò | Cũ → mới | Component / class |
|---|---|---|
| Tin nhắn, máy tính | 14/21,7 → **15/23** | `MessageContent.tsx` hai nhánh: `text-sm leading-[1.55]` → `text-base md:text-md` (bỏ luôn giá trị tuỳ ý `leading-[…]`); placeholder tin đã xoá trong `MessageBlock` (`text-sm italic`) cũng đổi theo |
| Tin nhắn, điện thoại | 14 → **16/24** | như trên (`text-base` dưới md) |
| Tên người gửi | 14 → **15** /600 | `MessageBlock` hàng `who`: thêm `text-md` |
| Hàng điều hướng | 14 → **15** (giữ h-9) | `ChatNavigator.tsx` các hàng `h-9 … text-sm` (dòng 75, 132, 187) → `text-md` |
| Tiêu đề mục | 13 → **14** /600 | `ChatNavigator` dòng 263, `PhoneChatList` dòng 148: `text-small` → `text-sm` |
| Tóm tắt thread | 13/18 → **14/20** (giữ min-h 32/36) | `ThreadSummary.tsx`: `text-small` → `text-sm` |
| Trích dẫn | 13 → **14** | `QuoteBlock.tsx` frame: `text-small` → `text-sm` |
| Code trong tin | 13 → **14** | `.message-html code` 0.8125rem → 0.875rem (giữ `.chat-editor-content code` khớp) |
| "đã sửa" | 11 → **12** | `MessageBlock` dòng 173: `text-2xs` → `text-xs` |
| Danh sách điện thoại: tên / xem trước | 14/14 → **16/14** | `ConversationRow.tsx`: tên `max-md:text-base md:text-md`, xem trước `text-sm` (feed Trang chủ trên máy tính dùng chung: tên 15, xem trước 14) |
| Giờ, badge, chữ phụ 12 | giữ | `text-xs` |
| Ô soạn | giữ 16/24 | |
| Chữ giao diện, bảng, admin | giữ 14/21 | `text-sm` |
| Sidebar chính, rail, thanh đáy | giữ 14/500, 11 | |

`ThreadPanel` và `ThreadsPanel` dùng chung `MessageBlock` và `MessageContent`, nên tự lên 15 trong khe 360.

### Đoạn đề xuất cho DESIGN.md §3 (thay bảng scale)

> | `text-xs` | 12 / 16 | 500 | nhãn phụ, giờ, badge, "đã sửa" |
> | `text-sm` | 14 / 21 | 400 | mặc định giao diện: ô bảng, mô tả, form; tiêu đề mục chat, hàng tóm tắt thread, trích dẫn |
> | `text-md` | 15 / 23 | 400 | **đọc chat trên máy tính**: tin nhắn, tên người gửi (600), hàng cột điều hướng, tên hàng ở feed |
> | `text-base` | 16 / 24 | 400 | input và ô soạn (tránh zoom mobile), **tin nhắn và tên hàng danh sách trên điện thoại**, đọc dài |
> | … (lg/xl/2xl giữ) |
>
> Chữ chat cố ý to hơn Google Chat (14px) 1px trên máy tính và 2px trên điện thoại. Màn dày dữ liệu (bảng, quản trị) giữ 14. Mono trong tin là 14.

Kèm theo cần sửa: §6 "Luồng" (`text-sm` line-height 1.55 → `text-md`), "Chat trên điện thoại" (chữ 15px → 16px), "Tiêu đề mục" 13px → 14px, "Hiển thị định dạng trong tin" code 13px → 14px, nhãn "đã sửa" 11px → 12px.

## 4. Rủi ro bố cục

- **Hàng 36 ở cột điều hướng:** chữ 15 với dòng 21 vẫn vừa và đã đo được 36. Tên dài bị cắt sớm hơn khoảng 7%, tức mất khoảng 2 ký tự trước dấu "…" trong cột 288 (thấy trong mockup: "Kế toán quý 4 · đối chiếu cô…").
- **Hàng 64 trên điện thoại:** tên 22 + 2 + xem trước 20 = 44, vừa khít phần đệm 10 trên dưới. Nếu sau này có người bật cỡ chữ hệ thống thì hàng phải được giãn (`min-h`, không được đặt `h` cố định). `ConversationRow` hiện dùng padding nên không bị vấn đề này.
- **Mật độ luồng:** tin nhiều dòng cao thêm khoảng 7% trên máy tính và khoảng 14% trên điện thoại. Ở khung 780 của mockup, bản đề xuất vừa đủ cùng 3 tin mà bản hiện tại còn trống phía trên. Trên thực tế mỗi màn hiện ít hơn khoảng một tin trên mười.
- **Căn avatar:** avatar 32 căn với dòng tên. Tên 15/21 cao hơn 14/19,6 khoảng 1,4px nên bong bóng đầu tụt xuống 1–2px. Không thấy lệch trong ảnh chụp.
- **`max-w-[min(70ch,85%)]`:** `ch` lớn theo cỡ chữ nên trần 70ch rộng thêm khoảng 7%. Trong cột 480–520 thì giới hạn 85% vẫn là ràng buộc thắng, nên không đổi gì.
- **Test:** chưa thấy test nào assert theo tên class cỡ chữ (grep `text-sm|text-small` trong `*.test.tsx` không có kết quả). Vẫn cần chạy `npm test` và cập nhật snapshot nếu có.
- **Lệch sẵn có (không do đề xuất này):** DESIGN.md và chat-mobile.html ghi tin nhắn trên điện thoại 15px và avatar hàng 44, nhưng code đang là 14 và 40. Đề xuất sửa luôn phần cỡ chữ. Avatar 44 tách thành việc riêng.
- **Rail (app-rail.html, đang có người làm):** không bị ảnh hưởng. Nhãn sidebar chính giữ 14/500, tooltip, badge 10–11 và chiều rộng 64/232 giữ nguyên. Chỉ cần quyết định nếu Zane muốn nhãn sidebar chính cũng lên 15 cho đồng bộ với cột điều hướng; tôi đề xuất **không**, để giữ thứ bậc giữa điều hướng app và nội dung.

## Câu hỏi chưa giải quyết

1. Có giữ 14 cho bảng, form và trang quản trị như đề xuất không, hay muốn tăng 1px toàn app (`text-sm` 14 → 15)? Tăng toàn app đơn giản hơn nhưng làm xê dịch mọi bảng 44px và các trang admin dày.
2. Tin nhắn trên điện thoại chọn 16 (bằng ô soạn, đề xuất) hay 15 (đúng như DESIGN.md hiện ghi)?
3. Có cần đo lại Google Chat trên phiên đăng nhập thật để chốt mốc không? Các số ở §2 hiện là ước lượng.
4. Tiêu đề kênh (19) và dòng phụ header (13) đang giữ nguyên. Có muốn dòng phụ lên 14 không?

Status: DONE
Summary: Đã đo cỡ chữ đang chạy trong code và so với Google Chat (ước lượng). Đề xuất thêm một token `text-md` 15/23 cho chữ chat trên máy tính, `text-base` 16/24 cho tin nhắn trên điện thoại và nâng các chữ phụ trong chat 13 → 14, trong khi chữ giao diện và bảng giữ 14. Mockup so sánh và ảnh chụp sáng/tối ở 1440 và 390 đã qua kiểm tra Playwright: không lỗi, không tràn.
