# Nexus Hub Design System: Tín hiệu

> **Nguồn thiết kế duy nhất của frontend.** Chọn ngày 2026-10-09 từ
> [brainstorm](plans/261009-0449-project-wide-refactor/reports/brainstorm-261009-0457-new-ui-direction.html)
> (hướng B, mượn bảng kẻ mảnh của hướng A). Mockup đã duyệt nằm trong `design/mockups/`.
> `.stitch/` là lịch sử, không dùng làm nguồn nữa. Code render đúng file này và mockup; không
> thiết kế trong code. Muốn đổi thiết kế: sửa file này trước, trong cùng PR.

## 1. Thesis

Nexus Hub là nơi nhiều người cùng sửa một thứ: tin nhắn, thư mục, đề nghị phê duyệt. Câu hỏi
người dùng đặt ra liên tục là **"ai vừa làm gì"**, nên đó là bản sắc của giao diện: mỗi người có
một màu riêng, và mỗi thay đổi đến từ người khác mang màu đó trong chốc lát rồi tan.

- **Cảnh dùng:** nhân viên khối vận hành, kế toán, hành chính ở văn phòng sáng, laptop 13 đến
  15 inch, mở cả ngày, chuyển liên tục giữa chat, tài liệu và phê duyệt; thỉnh thoảng duyệt trên
  điện thoại giữa hai cuộc họp. → **Light là mặc định**, dark đầy đủ cho buổi tối và người thích.
- **Register:** product UI. Quen thuộc, đáng tin, đọc nhanh. Không có hiệu ứng trang trí; chuyển
  động chỉ để báo trạng thái, nguồn gốc thay đổi, hoặc quan hệ không gian.
- **Một chiều được đẩy tới cực:** chuyển động mang danh tính (realtime). Mọi thứ khác giữ yên.
- Dials: variance 4 · motion 5 · density 5 (bảng ở Tài liệu, Quản trị: density 7).

## 2. Color

OKLCH. Restrained: nền trung tính ngả ngọc lam (hue 190–200), một accent ngọc lam, 8 màu người
dùng. Accent ≤ 10% bề mặt, chỉ cho hành động chính, mục đang chọn và trạng thái cần chú ý. Không
`#000`/`#fff` thuần. Không `rgba()` rời rạc: overlay là token.

### Surfaces (nổi bằng sắc độ, không dùng viền cho container)

| Token | Light | Dark | Dùng cho |
|---|---|---|---|
| `--color-sunk` | `oklch(94.6% 0.011 190)` | `oklch(15.5% 0.012 200)` | sidebar, vùng chìm, track |
| `--color-base` | `oklch(97.4% 0.007 190)` | `oklch(18% 0.014 200)` | nền trang, luồng chat |
| `--color-raised` | `oklch(99.3% 0.003 190)` | `oklch(22.5% 0.016 200)` | panel, card, composer, mục chọn |
| `--color-overlay` | `oklch(99.6% 0.002 190)` | `oklch(26% 0.018 200)` | popover, modal, toast |
| `--color-hover` | `oklch(93.4% 0.012 190)` | `oklch(25% 0.016 200)` | hover trên sunk/base |
| `--color-scrim` | `oklch(23% 0.02 200 / 0.32)` | `oklch(8% 0.01 200 / 0.6)` | nền sau modal |

### Ink

| Token | Light | Dark | Dùng cho |
|---|---|---|---|
| `--color-ink` | `oklch(23% 0.02 200)` | `oklch(94% 0.008 190)` | chữ chính |
| `--color-ink-muted` | `oklch(46% 0.02 200)` | `oklch(73% 0.016 195)` | chữ phụ, metadata, placeholder (≥ 4.5:1) |
| `--color-ink-subtle` | `oklch(62% 0.015 200)` | `oklch(58% 0.014 200)` | chỉ cho disabled và icon trang trí |
| `--color-line` | `oklch(89.5% 0.01 195)` | `oklch(30% 0.014 200)` | đường kẻ 1px trong bảng, divider |

### Accent

| Token | Light | Dark |
|---|---|---|
| `--color-accent` | `oklch(50% 0.1 180)` | `oklch(76% 0.11 178)` |
| `--color-accent-hover` | `oklch(45% 0.1 180)` | `oklch(81% 0.1 178)` |
| `--color-accent-wash` | `oklch(93% 0.035 180)` | `oklch(29% 0.05 180)` |
| `--color-on-accent` | `oklch(98.5% 0.005 180)` | `oklch(18% 0.014 200)` |
| `--color-focus` | `oklch(55% 0.12 180)` | `oklch(78% 0.12 178)` |

### Semantic (tách khỏi accent; luôn đi cùng nhãn chữ, không chỉ màu)

| Token | Light fg / wash | Dark fg / wash |
|---|---|---|
| `success` | `oklch(49% 0.11 150)` / `oklch(94% 0.04 150)` | `oklch(76% 0.12 150)` / `oklch(28% 0.05 150)` |
| `warning` | `oklch(50% 0.12 65)` / `oklch(94.5% 0.05 75)` | `oklch(80% 0.12 70)` / `oklch(29% 0.05 70)` |
| `danger` | `oklch(52% 0.17 25)` / `oklch(94.5% 0.035 25)` | `oklch(72% 0.15 25)` / `oklch(29% 0.06 25)` |
| `info` | `oklch(50% 0.1 240)` / `oklch(94% 0.03 240)` | `oklch(76% 0.1 240)` / `oklch(29% 0.05 240)` |

Token: `--color-{success,warning,danger,info}` và `--color-{…}-wash`.

### Person hues

8 màu, gán **tất định** theo người dùng (hash của user id → 0..7, tính ở client, id không bao giờ
hiển thị). Dùng cho nền avatar, vòng presence, lớp phủ realtime. Không dùng cho trạng thái.

| # | Hue | `--color-person-N` (avatar fill, light & dark) | `--color-person-N-wash` light / dark |
|---|---|---|---|
| 1 | 45 cam đất | `oklch(52% 0.13 45)` | `oklch(93% 0.04 45)` / `oklch(30% 0.06 45)` |
| 2 | 95 vàng rêu | `oklch(50% 0.1 95)` | `oklch(94% 0.045 95)` / `oklch(30% 0.05 95)` |
| 3 | 140 lá | `oklch(50% 0.12 140)` | `oklch(93.5% 0.04 140)` / `oklch(30% 0.05 140)` |
| 4 | 205 biển | `oklch(50% 0.09 205)` | `oklch(93.5% 0.03 205)` / `oklch(30% 0.045 205)` |
| 5 | 245 lam | `oklch(50% 0.12 245)` | `oklch(93.5% 0.035 245)` / `oklch(30% 0.06 245)` |
| 6 | 285 tím | `oklch(51% 0.13 285)` | `oklch(93.5% 0.035 285)` / `oklch(30% 0.06 285)` |
| 7 | 325 mận | `oklch(51% 0.13 325)` | `oklch(93.5% 0.035 325)` / `oklch(30% 0.06 325)` |
| 8 | 355 hồng | `oklch(53% 0.13 355)` | `oklch(94% 0.035 355)` / `oklch(30% 0.06 355)` |

Chữ trên avatar: `--color-on-accent` light value (`oklch(98.5% …)`) ở cả hai theme (≥ 4.5:1 trên L ≤ 53%).

## 3. Typography

- **Display:** Bricolage Grotesque (opsz 12–96, 600–700). Chỉ cho tên workspace, tiêu đề trang,
  tiêu đề panel, số tiền lớn. Không dùng cho nút, nhãn, bảng.
- **Body:** Be Vietnam Pro (400, 500, 600). Mọi thứ còn lại.
- Tự host qua `@fontsource/bricolage-grotesque` và `@fontsource/be-vietnam-pro`, chỉ subset
  `latin` + `vietnamese`; preload Be Vietnam Pro 400.
- Fallback: `"Be Vietnam Pro", "Segoe UI", "Helvetica Neue", Arial, sans-serif`.
- **Mono** (`--font-mono`: `"JetBrains Mono", ui-monospace, "SF Mono", Menlo, Monaco, monospace`, không tải
  webfont): chỉ cho code và khối code trong tin nhắn, 13px.
- `font-variant-numeric: tabular-nums` cho giờ, số tiền, dung lượng, bộ đếm, cột bảng.
- Tiếng Việt có dấu chồng (ặ, ẫ, ổ): line-height thân chữ ≥ 1.5, tiêu đề ≥ 1.2, không tracking âm
  quá `-0.01em`.

Scale 1.2, `rem`:

| Token | Size / line | Weight | Dùng cho |
|---|---|---|---|
| `text-xs` | 12 / 16 | 500 | nhãn phụ, giờ, badge |
| `text-sm` | 14 / 21 | 400 | mặc định: tin nhắn, ô bảng, mô tả |
| `text-base` | 16 / 24 | 400 | input (tránh zoom mobile), đọc dài |
| `text-lg` | 19 / 26 | 600 display | tiêu đề panel, tên kênh |
| `text-xl` | 23 / 30 | 700 display | tiêu đề trang |
| `text-2xl` | 28 / 34 | 600 display | số tiền, con số trọng tâm |

Nhãn viết hoa (`label`): 12px, 600, `letter-spacing: 0.06em`, tối đa một nhãn cho mỗi panel.

## 4. Space, radius, depth, layers

- **Spacing:** 4, 8, 12, 16, 20, 24, 32, 48, 64. Không giá trị lẻ.
- **Radius:** control 8px · surface (panel, card, mục nav, tin nhắn hover) 10px · overlay 12px ·
  pill 999px chỉ cho badge, trạng thái, avatar. Bo lồng nhau = bo cha − padding cha.
- **Depth:** một chiến lược: **nổi bằng sắc độ** (`sunk → base → raised → overlay`). Container
  không có viền. Đường kẻ `--color-line` chỉ trong bảng và divider danh sách. Shadow chỉ cho
  overlay: `--shadow-overlay: 0 1px 2px oklch(23% 0.02 200 / 0.06), 0 8px 24px oklch(23% 0.02 200 / 0.1), 0 24px 48px oklch(23% 0.02 200 / 0.08)`.
- **Z-index:** `--z-dropdown 10` · `--z-sticky 20` · `--z-backdrop 30` · `--z-modal 40` ·
  `--z-toast 50` · `--z-tooltip 60`. Không `z-[…]` tuỳ ý.

## 5. Layout

```
┌─────────┬──────────────┬───────────────────────────┬──────────────┐
│ Sidebar │ List panel   │ Content                   │ Detail panel │
│ 232px   │ 280px        │ flex, min 480px           │ 360px        │
│ (rail   │ (kênh, cây   │                           │ (có điều     │
│  64px)  │  thư mục…)   │                           │  kiện)       │
└─────────┴──────────────┴───────────────────────────┴──────────────┘
```

- **Sidebar** (`--color-sunk`): workspace switcher (tên + avatar, display font), 5 mục chính
  **Tin nhắn · Tài liệu · Phê duyệt · Tài sản · Danh bạ**, đáy: **Thông báo** (mở NotificationPanel,
  §6), **Quản trị** (khi có quyền),
  **Cài đặt**, người dùng hiện tại. Tài sản nằm trong shell này, không có layout riêng.
- **Thu gọn sidebar (rail 64px)** (mockup `spaces.html` §3, nút "Thu gọn/mở menu chính"): IconButton
  "Thu gọn menu chính" (icon khung có cột trái) ở cuối hàng workspace, `aria-expanded`. Thu gọn: cột còn
  64px, chỉ icon 20; logo workspace và nút mở rộng xếp dọc ở đầu; avatar người dùng ở đáy; số chưa đọc
  thành badge 16 trên góc icon (viền 2px màu sidebar); tên mục hiện bằng tooltip, tên vẫn là accessible
  name của link. Đổi bề rộng `grid-template-columns` 280ms `--ease-out-expo`, chữ fade 120ms; giảm chuyển
  động thì đổi ngay. **Xem tạm:** trỏ chuột vào rail 300ms thì sidebar mở 232px **nổi đè** lên list panel
  (`--shadow-overlay`, không đẩy layout), rời chuột thì thu lại. Trạng thái nhớ theo thiết bị; 1024–1279
  mặc định thu gọn, ≥ 1280 mặc định mở. Phần chừa ra dành cho content (luồng chat).
- **List panel** chỉ ở Tin nhắn và Tài liệu. **Detail panel** mở bằng chọn mục, đóng bằng Esc.
- **Breakpoints:** ≥ 1280 đủ 4 cột · 1024–1279 detail panel thành overlay bên phải ·
  768–1023 sidebar thành rail, list panel thành cột thay thế content (rail chưa làm: hiện thanh
  đáy dùng cho cả dưới 1024) · < 768 thanh đáy 3 tab có nhãn + "Thêm", mỗi màn một cột, detail
  panel thành sheet từ dưới lên.
- **Điều hướng di động (dưới 1024 cho tới khi rail 64px của 768–1023 được làm):** thanh trên + thanh đáy 3 tab có nhãn **Tin nhắn · Tài liệu · Phê duyệt** (kèm
  số chờ duyệt) + mục **Thêm** mở sheet đáy. Không có avatar nổi; mọi thứ còn lại vào qua Thêm.
  - *Thanh trên:* `--color-base`, không viền, cao 48 + `env(safe-area-inset-top)`. Trái: **Nexus**
    (font display 700) + " · " + tên workspace (mờ, cắt bằng …). Phải: IconButton tìm kiếm (icon
    20, vùng chạm ≥ 44, `aria-label` "Tìm kiếm") đưa focus vào ô tìm kiếm có sẵn của màn đang xem;
    trong một cuộc trò chuyện nó mở bảng Tìm của cuộc trò chuyện. Màn không có tìm kiếm riêng
    (Trang chủ, Phê duyệt, Cài đặt…) thì không có nút. Tiêu đề màn nằm ngay dưới thanh. Không có nút
    Menu: danh sách trò chuyện là Trang chủ của Tin nhắn (có "Trò chuyện mới": nhắn trực tiếp,
    tạo nhóm), mọi thứ còn lại vào qua tab, Thêm hoặc đổi workspace trong sheet.
  - *Thanh đáy:* `--color-raised`, không viền, `--z-sticky`; 4 cột đều nhau. Mỗi tab cao 48, thanh
    đệm 4 trên và dưới (cao 56) + `env(safe-area-inset-bottom)`; vùng chạm cả ô ≥ 44. Mỗi tab một
    icon lucide 20 stroke 1.75 trên nhãn 11px/500 (`text-2xs`); chỉ 4 icon trên cả thanh
    (Thêm dùng dấu ba chấm). Đang chọn: nhãn 600 `--color-accent`, icon nằm trong nền
    `--color-accent-wash` (radius 10, 48×24), `aria-current="page"`; Thêm cũng sáng khi màn hiện tại
    là Tài sản, Danh bạ, Quản trị hoặc Cài đặt. Số chờ duyệt: badge pill accent 18px ở góc trên phải
    icon, `99+` khi quá 99, ẩn khi 0, `aria-label` "Phê duyệt, 3 chờ bạn". Chỉ Phê duyệt có số, và chỉ lấy số khi thanh đáy đang hiện. Có thông báo chưa đọc thì Thêm mang
    chấm 8px `--color-accent` (viền 2px `--color-raised`, không số, không làm Thêm sáng), `aria-label`
    "Thêm, 3 thông báo chưa đọc". Toast nổi ngay trên thanh đáy (cách 16 + safe-area).
  - *Sheet Thêm:* `--color-overlay`, radius 12 hai góc trên, `--shadow-overlay`, đệm 16 +
    safe-area, cao tối đa 85% màn, nội dung cuộn; nằm trên thanh đáy (`--z-modal`), nền sau là
    `--color-scrim` (`--z-backdrop`). Thứ tự từ trên: hàng tay nắm 36×4 (`--color-line`, giữa) + IconButton Đóng bên phải · **hàng
    workspace + người**: ô chữ cái workspace 40, tên workspace (font display), dưới là avatar 20 +
    tên hiển thị + vai trò mờ, chevron phải; bấm mở danh sách đổi workspace · **Thông báo · Tài sản ·
    Danh bạ · Quản trị · Cài đặt** (hàng cao 48, icon 18 + nhãn, như NavRow; Thông báo có pill số chưa
    đọc cuối hàng và thay nội dung sheet bằng danh sách thông báo, như danh sách workspace: quay lại ·
    "Thông báo" · Đóng, sheet giữ cao 85%) · divider `--color-line` ·
    **Đăng xuất**. Hàng đổi workspace thay nội dung sheet bằng danh sách workspace người đó vào
    được (tên + vai trò, dấu tích ở workspace đang mở, nút quay lại); chọn một workspace gắn lại
    phiên vào workspace đó, làm mới dữ liệu rồi đóng sheet.
  - *Hành vi:* modal đầy đủ (focus trap, focus vào hàng đầu khi mở, trả focus về nút Thêm, nền sau
    inert). Đóng bằng Esc, chạm scrim, nút Đóng, hoặc kéo tay nắm xuống quá 25% chiều cao hay vuốt
    nhanh. Chọn một mục điều hướng cũng đóng. Chuyển động theo §7: scrim fade 220ms, sheet
    `translateY(100%)` → 0 trong 280ms expo; ra 210ms ease-in; đổi giữa menu và danh sách workspace
    là fade 160ms. Giảm chuyển động: bỏ translate, chỉ fade ≤ 120ms (kéo xuống vẫn theo ngón tay).

## 6. Components

Primitive sống ở `frontend/src/components/primitives/`; không component nào tự vẽ lại chúng.

- **Button:** `primary` (accent), `secondary` (`--color-hover` nền), `ghost`, `danger`. Cao 36
  (`sm` 32), padding ngang 14, radius 8, 14px/600. Hover: nền sáng/tối một bậc; press
  `scale(0.98)` 100ms; focus ring 2px `--color-focus` offset 2px. Loading: spinner thay icon,
  giữ nguyên chiều rộng. Một nút primary mỗi view.
- **IconButton:** 32×32 hit area ≥ 44 trên touch (`::before` inset −6px), `aria-label` bắt buộc.
- **Input / Select / Textarea:** cao 40, 16px, nền `--color-raised`, không viền; focus = ring.
  Label luôn có (không dùng placeholder làm label). Lỗi dưới ô, `aria-describedby`, validate khi blur.
- **Avatar:** tròn, 20/24/32/40 (64 chỉ cho hồ sơ trong Danh bạ và Cài đặt), nền `--color-person-N`, 2 chữ cái đầu của **tên hiển thị**
  (không bao giờ của id). Có ảnh thì dùng ảnh. Presence: chấm 8px `success` ở góc dưới phải.
- **PersonChip:** avatar 20 + tên hiển thị (+ vai trò mờ). Là cách duy nhất hiển thị một người.
- **Badge / StatusPill:** pill, 12px/600, nền wash semantic + chữ fg semantic. Nhãn đọc được
  ("Đang chờ"), không mã trạng thái.
- **NavRow:** cao 36, radius 10, icon 18 (lucide, stroke 1.75) + nhãn + bộ đếm pill accent.
  Đang chọn: nền `--color-raised`, chữ 600. Hover: `--color-hover`.
- **Table (bảng kẻ mảnh):** header 12px/600 `ink-muted`, hàng cao 44, divider `--color-line`
  1px, không zebra, không viền ngoài. Hàng chọn: `--color-accent-wash`. Cột số và giờ
  `tabular-nums`, căn phải. Hàng là `button`/`a` thật, điều hướng bàn phím ↑↓ Enter.
- **Tree:** một component cho cây thư mục, phòng ban, chọn thư mục. Thụt 16px mỗi cấp.
- **Panel:** `--color-raised`, radius 10, padding 16, margin 12 khỏi mép.
- **Modal:** `--color-overlay`, radius 12, shadow overlay, max 520px, focus trap + trả focus,
  Esc đóng. Modal là lựa chọn cuối: ưu tiên detail panel hoặc thao tác tại chỗ + Undo.
- **Popover / Menu:** Popover API hoặc portal; không `absolute` trong `overflow: hidden`.
- **Toast:** đáy giữa (mobile) / đáy phải (desktop), tối đa 3, tự tắt 5s, có Undo khi thao tác
  hoàn tác được. Lỗi mutation luôn ra toast với câu giải thích cách sửa.
- **Skeleton:** một kiểu shimmer, cùng hình dạng nội dung thật. Không spinner toàn trang.
- **Empty / Error state:** icon 24 + một câu nói chuyện gì xảy ra + một hành động. Mọi query đều có.
- **Picker** (người, vai trò, phòng ban, thư mục): ô tìm kiếm + danh sách PersonChip/Tree. Không
  bao giờ có ô nhập ID.
- **ApprovalChain:** dọc; mỗi bước: avatar người duyệt, tên, chức danh mờ, StatusPill, giờ.
- **NotificationPanel** (mockup `design/mockups/notifications.html`): desktop là popover lớn cạnh
  sidebar, rộng 380, cách mép 12, cao trọn khung, `--color-overlay`, radius 12, `--shadow-overlay`;
  không scrim, Esc / bấm ngoài / Đóng thì đóng, trả focus về NavRow. Dưới 1024 nằm trong sheet Thêm
  (§5). Đầu: tiêu đề `text-lg` + "N chưa đọc" mờ + Button `ghost sm` "Đánh dấu tất cả đã đọc"
  (disabled khi 0). Nhóm "Hôm nay" / "Trước đó" bằng divider ngày của chat. 25 mỗi trang, cuối là
  "Tải thêm" + "Đã hiện X trong Y". Có Skeleton, Empty ("Chưa có thông báo nào…" + "Xem đề nghị của
  bạn") và Error ("Không tải được thông báo…" + "Thử lại").
- **NotificationRow:** liên kết thật: Avatar 32 của người gây ra (không có người: ô icon 32 radius 10
  `--color-sunk`) + câu **ai đã làm gì cái gì** (tên người và tên đối tượng, không bao giờ id; dựng ở
  client theo loại, không in `title`/`body` của server) + lý do nếu có (khối `--color-sunk`, tối đa 2
  dòng) + meta 12px: icon miền + "Phê duyệt" / "Tài sản" + giờ tương đối. Chưa đọc: chữ `--color-ink`,
  tên 600, chấm 8px `--color-accent` cuối hàng + chữ ẩn "Chưa đọc"; đã đọc: chữ `--color-ink-muted`,
  không chấm; không tô nền. Bấm: đánh dấu đã đọc rồi mở đối tượng; trỏ chuột vào hàng chưa đọc thì chấm
  thành IconButton "Đánh dấu đã đọc" (không có trên màn chạm). Đánh dấu là lạc quan, lỗi thì hoàn lại +
  toast. Realtime theo §7: bảng mở thì chèn hàng + lớp phủ màu người + nhãn "vừa duyệt / vừa giao…";
  bảng đóng thì toast có "Xem"; gộp ≥ 3 trong 2000ms; đang mở đúng đối tượng thì đánh dấu đã đọc, không
  toast. Bộ đếm đổi số bằng fade 160ms. Bảng mở: `translateX(-16px) scale(.985)` → 0, 280ms expo; đóng
  210ms ease-in.

### Chat: cột điều hướng và mục (mockup `design/mockups/spaces.html` §1)

Thứ tự trong cột: nút "Trò chuyện mới" · lối tắt (Trang chủ, Được nhắc tới, Đã gắn sao) · **mục của bạn** ·
Tin nhắn trực tiếp · Nhóm · "Duyệt nhóm" · "Tạo mục".

- **Mục (ChatSection):** người dùng tự tạo để gom nhóm và tin nhắn trực tiếp, như Google Chat. Mục là **của
  riêng từng người** trong từng workspace; người khác không thấy và không bị ảnh hưởng. Tên 1–40 ký tự, không
  trùng (không phân biệt hoa thường) trong các mục của bạn. Một cuộc trò chuyện nằm trong **tối đa một** mục;
  đã vào mục thì không lặp lại ở Tin nhắn trực tiếp hay Nhóm. Mục chỉ là cách sắp xếp, không đổi quyền hay
  thành viên.
- **Tiêu đề mục:** `button` có `aria-expanded`, chevron 16 xoay −90° khi thu gọn (160ms), tên 13px/600
  `--color-ink` (mục mặc định `--color-ink-muted`). Thu gọn thì cuối tiêu đề hiện pill đếm = tổng chưa đọc
  trong mục. Mục của bạn có IconButton 28 "⋯" (hiện khi hover hoặc focus) mở menu: **Đổi tên mục · Chuyển
  lên · Chuyển xuống** (mục đầu/cuối thì mục tương ứng disabled) · divider · **Xoá mục** (`danger`). Mục mặc
  định có nút "+" (Nhắn tin trực tiếp / Tạo nhóm) như cũ, không đổi tên hay xoá được. Trạng thái thu gọn
  nhớ theo thiết bị.
- **Tạo / đổi tên:** popover nổi trong cột (không modal): ô "Tên mục" + Huỷ / Tạo (Lưu); Enter xác nhận,
  Esc đóng, lỗi nói rõ ("Nhập tên mục", "Bạn đã có mục tên này"). Mở từ "Tạo mục" cuối cột, từ menu "Trò
  chuyện mới" hoặc "Mục mới…" trong menu của một cuộc trò chuyện (khi đó cuộc trò chuyện vào luôn mục vừa
  tạo). Mục mới nằm cuối các mục của bạn; rỗng thì hiện gợi ý 13px mờ cách mở menu ⋯ để chuyển vào.
- **Chuyển cuộc trò chuyện:** mỗi hàng có IconButton 28 "⋯" ở mép phải, hiện khi hover hoặc focus (thay chỗ
  pill chưa đọc trong lúc đó). Menu: nhãn "Chuyển tới mục" + danh sách mục của bạn (`menuitemradio`, dấu ✓ ở
  mục hiện tại) · "Mục mới…" · divider · "Bỏ khỏi mục" (chỉ khi đang ở trong mục; về lại Tin nhắn trực tiếp
  hoặc Nhóm) · Gắn sao · Tắt thông báo. Trên máy tính kéo thả hàng vào tiêu đề mục cũng chuyển được; bàn
  phím và màn chạm (nhấn giữ hàng) dùng menu. Chuyển xong: hàng fade vào chỗ mới, mục đích tự mở, toast "Đã
  chuyển … vào “…”".
- **Xoá mục:** không hỏi xác nhận vì không mất dữ liệu: các cuộc trò chuyện trở về Tin nhắn trực tiếp hoặc
  Nhóm, toast "Đã xoá mục “…”" có **Hoàn tác** (5s) khôi phục mục, vị trí và các cuộc trò chuyện.
- **Mobile:** tab Tin nhắn dùng cùng thứ tự và cùng mục; menu ⋯ thành sheet đáy.

### Chat: luồng in-line, thread, ô soạn (mockup `design/mockups/spaces.html` §2–§5; §2 là tin nhắn trực tiếp với tin hai bên)

Nhóm và tin nhắn trực tiếp dùng **chung một mô hình in-line threading** như Google Chat. Không có thẻ chủ đề:
mỗi tin là một hàng trong luồng theo thời gian; tin nào có trả lời thì có hàng tóm tắt thread ngay dưới. Enter
trong ô soạn chính gửi một tin vào luồng, không mở gì thêm; trả lời đi vào thread qua hover action hoặc hàng
tóm tắt. Panel thread, panel Thread của nhóm và panel Thành viên dùng chung khe detail panel 360px (§5): mở cái
này thì cái kia đóng.

- **Luồng (MessageStream):** nền `--color-base`, cũ trên mới dưới, tin xếp **hai bên**. Tin của người khác
  bên trái: Avatar 32 + tên hiển thị 600 + giờ `text-xs` mờ tabular phía trên bong bóng. **Tin của bạn bên
  phải**: không avatar, không tên (chữ ẩn "Bạn" cho trình đọc màn hình), chỉ giờ ở trên, căn phải. Bong bóng:
  `text-sm` line-height 1.55, padding 7 × 12, radius 16, rộng tối đa min(70ch, 85%) cột tin, co theo nội
  dung; người khác nền `--color-sunk`, của bạn nền `--color-accent-wash`; góc phía người gửi của bong bóng
  đầu nhóm là 4 (trên trái với người khác, trên phải với bạn). `code` và khối code trong bong bóng nền
  `--color-base`. Đính kèm (lưới ảnh, thẻ tệp), cảm xúc và hàng tóm tắt thread nằm ngoài bong bóng, bám
  theo bên của tin. Hàng tin padding 8 × 14, không tô nền khi hover (bong bóng đã đủ khối); lớp phủ
  realtime (§7) vẫn phủ cả hàng. Tin đã ghim có nhãn "Đã ghim" (icon 12) sau giờ. Khoảng cách giữa hai bong bóng
  cùng cụm là 4px. Trích dẫn (đợt 3) là khối `--color-raised` radius 10 nằm trong bong bóng: tên người được trích +
  tối đa 2 dòng, bấm thì cuộn tới tin gốc. Áp dụng như nhau trong
  panel thread và tin nhắn trực tiếp.
- **Gom tin:** tin liền sau của **cùng người gửi** trong **≤ 5 phút** kể từ tin trước của họ thì ẩn avatar và
  tên, padding trên 2; giờ của dòng đó hiện ở lề ngoài (cột avatar bên trái với người khác, cột 32 bên phải
  với bạn; 11px mờ, tabular) khi hover hoặc focus. Ngắt nhóm khi:
  khác người, cách > 5 phút, có vạch ngày hoặc vạch "Tin mới" ở giữa, tin trước có hàng tóm tắt thread, hoặc tin
  trước là tin hệ thống (công việc, thành viên vào/ra). Gom chỉ là cách hiển thị: mỗi dòng vẫn là một tin riêng
  với hover actions, cảm xúc và thread riêng. Áp dụng như nhau trong panel thread.
- **Vạch ngày:** nhãn giữa 12px/600 mờ, đường `--color-line` hai bên, `role="separator"`: "Hôm nay", "Hôm qua",
  thứ + ngày trong 7 ngày ("Thứ Hai, 06/10"), cũ hơn thì ngày đầy đủ "28/09/2026".
- **Vạch "Tin mới":** trước tin chưa đọc đầu tiên của luồng (tin của người khác, sau read receipt của bạn). Đường
  1px `--color-accent` chạy từ trái, nhãn "Tin mới" 12px/600 `--color-accent` ở cuối bên phải (khác hình với vạch
  ngày nằm giữa). Vạch giữ nguyên vị trí suốt lần xem; mất khi rời cuộc trò chuyện rồi quay lại, hoặc khi bạn gửi
  tin (fade 220ms). Tin của chính mình không bao giờ chưa đọc.
- **Mở cuộc trò chuyện:** tin chưa đọc vừa trong một màn → mở ở cuối luồng (vạch nằm trong màn); dài hơn một màn →
  mở tại vạch, vạch cách đỉnh luồng 48px để còn một dòng ngữ cảnh. Read receipt tiến tới tin mới nhất **đã hiện
  trên màn**; tới cuối luồng thì số chưa đọc của cuộc trò chuyện về 0.
- **Nút nổi (JumpPill):** một pill duy nhất, giữa đáy luồng, cách ô soạn 12; `--color-overlay`, `--shadow-overlay`,
  cao 36, 13px/600, icon mũi tên 16, `--z-sticky`. Hai trạng thái, không bao giờ hiện cùng lúc:
  - **"Tin mới nhất" ↓** (+ pill đếm accent khi có tin chưa đọc phía dưới, ví dụ "3"): khi bạn cách cuối luồng hơn
    nửa màn, hoặc có tin mới đến trong lúc bạn đang cuộn lên. Bấm → cuộn tới cuối.
  - **"Nhảy tới tin chưa đọc" ↑:** khi vạch "Tin mới" nằm trên màn hình và bạn đã bỏ qua nó (bấm "Tin mới nhất",
    mở từ thông báo hoặc kết quả tìm kiếm). Bấm → cuộn để vạch cách đỉnh 48px.
  - Đang ở cuối và không bỏ qua gì → ẩn. Tin mới đến khi bạn đang ở cuối → tự cuộn, không hiện nút. Tin bạn gửi →
    luôn cuộn tới cuối.
- **Hàng tóm tắt thread (ThreadSummary):** dưới nội dung của tin có ≥ 1 trả lời, thẳng lề chữ; là `button` cao
  32, radius 8, hover `--color-raised`. Thứ tự: cụm Avatar 20 của **tối đa 3 người trả lời gần nhất** (mới nhất
  bên trái, viền 2px màu nền) · **"N trả lời"** 13px/600 `--color-accent` · "Lần cuối 09:20" mờ (giờ theo §8; hôm
  qua "Hôm qua 17:05", cũ hơn "06/10") · pill **"M mới"** (pill đếm accent) **chỉ khi bạn đang theo dõi thread
  và có trả lời chưa đọc**. Thread không theo dõi không bao giờ có "mới", số trả lời vẫn đúng. Thread đang mở ở
  panel: nền `--color-accent-wash`, `aria-expanded="true"`. `aria-label` đọc đủ: "Thread, 3 trả lời, lần cuối
  09:20, 2 trả lời mới". Tin chưa có trả lời không có hàng này.
- **Theo dõi thread:** tự theo dõi khi bạn viết tin gốc, trả lời trong thread hoặc được nhắc tên trong thread; tự
  bỏ bằng nút Theo dõi hoặc menu Thêm. Theo dõi nghĩa là có "mới" và thông báo cho trả lời mới; mức thông báo
  theo nhóm thuộc đợt 4.
- **Hover actions:** thanh nổi ở góc trên phải hàng tin (trên −14, phải 12; tin của bạn: phải 52, ngay trên góc bong bóng), `--color-overlay`, radius 10,
  `--shadow-overlay` (overlay duy nhất trong một tin), IconButton 32 có tooltip: **Bày tỏ cảm xúc · Trả lời
  trong thread · Ghim / Bỏ ghim · Thêm**. Hiện khi hover hoặc khi focus nằm trong hàng (Tab tới được). Trong
  panel thread không có "Trả lời trong thread". Tin của bạn (chữ, không phải thẻ) có thêm IconButton **Sửa tin**
  (bút chì) trước Thêm. **Menu Thêm** theo thứ tự: Theo dõi thread / Bỏ theo dõi thread, Sao chép văn bản, Trích
  dẫn khi trả lời, Đánh dấu chưa đọc (tin người khác), Sửa tin (tin của bạn), divider, **Xoá tin** (`danger`; tin
  của bạn, hoặc tin người khác khi bạn quản lý nhóm). Mục chưa làm thì **không hiện** (không có mục disabled chờ
  tính năng). Menu mở lên trên khi gần đáy luồng. Màn chạm: nhấn giữ tin mở sheet đáy với hàng cảm xúc nhanh và
  cùng các mục.
- **Sửa tin** (mockup §2 "Sửa tin cuối của bạn"): mở bằng nút bút chì, Thêm → Sửa tin, hoặc **↑ khi ô soạn trống**
  (tin gần nhất của bạn trong cuộc trò chuyện hoặc thread đang mở). Bong bóng thành ô sửa tại chỗ: nền
  `--color-raised`, viền trong 2px `--color-accent`, radius 12, rộng tối đa min(560, 85%); dưới chữ: "Esc huỷ ·
  Enter lưu" 12px mờ + Huỷ (ghost sm) + Lưu (primary sm). Enter lưu, Shift+Enter xuống dòng, Esc huỷ (trả nguyên
  bản). Trích dẫn và đính kèm giữ nguyên. Lưu xong cuối bong bóng có nhãn **"đã sửa"** 11px mờ (cả người khác
  thấy). Lưu chữ rỗng = hỏi xoá tin. Người khác thấy chữ đổi tại chỗ (fade 160ms), không tính là tin mới, không
  thông báo. Thẻ (bình chọn, công việc…) không sửa được.
- **Xoá tin:** luôn qua hộp xác nhận (`alertdialog`, rộng 420): "Xoá tin nhắn?" + câu nói rõ hậu quả ("Tin sẽ bị
  xoá với mọi người trong cuộc trò chuyện và không khôi phục được."; tin người khác: nêu tên người gửi và "Nhật ký
  nhóm ghi lại bạn đã xoá tin này"; tin có thread: "Thread của tin vẫn giữ lại.") + Huỷ (focus mặc định) / **Xoá**
  (Button `danger`). Xoá xong: tin **không có thread** thu lại và biến mất ở mọi người (fade + co 220ms; tin sau
  cùng người gửi lên làm đầu cụm); tin **có thread** còn khung **"Tin nhắn đã bị xoá"** (chữ nghiêng mờ, không nền,
  viền trong 1px `--color-line`), mất đính kèm và cảm xúc, giữ hàng tóm tắt thread.
- **Panel thread (ThreadPanel):** detail panel (dưới 1024 là sheet toàn màn). Đầu: "Thread" `text-lg` + tên nhóm
  hoặc người (DM) mờ · nút **Theo dõi / Đang theo dõi** (toggle `aria-pressed`, Button sm: tắt = `secondary` +
  icon chuông, bật = nền `--color-accent-wash`, icon chuông reo `--color-accent`; nhãn đổi theo trạng thái, đổi
  xong báo `aria-live` "Đã theo dõi thread" / "Đã bỏ theo dõi thread", không toast) · IconButton Đóng (Esc). Thân:
  tin gốc đầy đủ (đính kèm, cảm xúc), vạch "N trả lời", các trả lời theo thời gian có gom tin, vạch **"Tin mới"**
  trước trả lời chưa đọc đầu tiên (giữ tới khi đóng panel). Mở panel = đánh dấu thread đã đọc: pill "M mới" trên
  hàng tóm tắt và trong panel Thread tắt (fade 160ms). Đáy: ô soạn "Trả lời trong thread" đủ nút như ô soạn chính;
  kéo thả tệp vào panel thì tệp vào ô soạn của panel.
- **Panel Thread của nhóm (ThreadsPanel):** mở bằng IconButton "Thread" (icon hai bong bóng) ở header của nhóm
  và DM; badge pill accent 16 = số thread đang theo dõi có tin mới (ẩn khi 0); `aria-pressed` khi panel mở. Đầu:
  "Thread" + tên nhóm mờ; dưới là 3 chip lọc (radio, kiểu `fchip`): **Tất cả · Đang theo dõi · Chưa đọc** (Chưa
  đọc = thread đang theo dõi có trả lời chưa đọc). Danh sách xếp theo trả lời cuối, mới nhất trên. Mỗi mục là
  `button` radius 10, padding 12, hover `--color-hover`: Avatar 24 + tên người viết tin gốc 600 + giờ tin gốc +
  icon chuông 14 nếu đang theo dõi (chữ ẩn "Đang theo dõi") · tin gốc rút gọn 2 dòng (văn bản thuần; có đính kèm
  thì thêm "icon + 3 ảnh" / "icon + 2 tệp") · hàng meta: cụm Avatar 20 (≤ 3) + "N trả lời" + "Lần cuối 09:40" +
  pill "M mới" cuối hàng. Bấm mục: panel chuyển sang thread đó (fade 160ms) với IconButton Quay lại về danh sách
  (giữ bộ lọc và vị trí cuộn); luồng chính cuộn tới tin gốc. Trạng thái: Skeleton 4 mục; Rỗng theo bộ lọc, Tất
  cả: "Chưa có thread nào. Trả lời một tin để bắt đầu thread."; Đang theo dõi: "Bạn chưa theo dõi thread nào." +
  "Xem tất cả"; Chưa đọc: "Bạn đã đọc hết các thread đang theo dõi." + "Xem tất cả"; Lỗi: "Không tải được danh
  sách thread." + "Thử lại".
- **Trạng thái luồng:** đang tải: Skeleton 6 hàng tin (tròn 32 + hai thanh); tải thêm khi cuộn tới đỉnh: Skeleton
  2 hàng ở đỉnh, giữ nguyên vị trí đọc; rỗng (nhóm): "Chưa có tin nhắn nào. Gửi tin đầu tiên cho nhóm." (DM: "Bắt
  đầu trò chuyện với Nguyễn Thu Lan."); lỗi: "Không tải được tin nhắn." + "Thử lại".
- **Ô soạn (Composer):** `--color-raised` (trong panel: `--color-base`), radius 12, cách mép 16, padding 10 10 10
  14. Từ trên xuống: thanh định dạng (khi bật) → hàng chờ tệp (khi có) → ô nhập 16px (cao tối đa 40% luồng rồi
  cuộn) → thanh công cụ: **Đính kèm** · **"A" Định dạng** · Cảm xúc · gợi ý "Enter để gửi · Shift+Enter xuống
  dòng" (ẩn dưới 768) · **Gửi** (primary sm). Placeholder: "Tin nhắn cho Đối soát giao dịch" / "Trả lời trong
  thread". Gửi disabled khi không có chữ lẫn tệp, hoặc còn tệp đang tải hay bị lỗi; lý do hiện ngay trước nút
  ("Đang tải 1 tệp…", "Thử lại hoặc bỏ tệp lỗi để gửi").
- **Đính kèm:** IconButton kẹp giấy "Đính kèm" mở menu (popover mở lên): **Tải lên từ máy** (hộp chọn tệp của hệ
  thống, chọn nhiều) · **Chọn từ Tài liệu** (Picker cây thư mục §6; không bao giờ nhập đường dẫn hay mã). Dán ảnh
  (Ctrl/Cmd+V) và kéo thả cũng thêm vào hàng chờ.
- **Vùng thả tệp:** chỉ khi đang kéo **tệp** (không phải chữ) vào khung hội thoại (luồng + ô soạn) hoặc panel
  thread: lớp phủ kín vùng đó, cách mép 8, radius 12, nền `--color-accent-wash`, viền nét đứt 2px `--color-accent`;
  giữa là icon tải lên 24 `--color-accent`, "Thả tệp để đính kèm" (`text-lg`) và "Tối đa 20 tệp mỗi tin" mờ. Ra
  khỏi vùng hoặc Esc → ẩn; thả → tệp vào hàng chờ của ô soạn tương ứng.
- **Hàng chờ tệp (AttachmentTray):** trên ô nhập, các mục xếp ngang và xuống dòng, gap 8; quá 2 hàng thì cuộn
  trong hàng chờ. **Ảnh:** thumbnail 64 × 64, radius 8, `object-fit: cover`. **Tệp khác:** chip cao 56, rộng tối
  đa 240, nền `--color-base`, ô icon 32 theo loại + tên (cắt …) + dòng phụ "Bảng tính · 412 KB". Mỗi mục có
  IconButton tròn 24 "Bỏ <tên tệp>" ở góc trên phải (vùng chạm 44; bỏ tệp đang tải là huỷ tải). **Đang tải:**
  thanh 3px ở đáy mục (`--color-line` / `--color-accent`), dòng phụ "Đang tải 64%", ảnh mờ 60%. **Lỗi:** ô icon
  `--color-danger-wash` + icon cảnh báo `--color-danger`, dòng phụ nói vì sao ("Vượt 25 MB của drive nhóm",
  "Mất kết nối") màu `--color-danger`, IconButton **Thử lại**. **Tối đa 20 tệp:** thêm quá thì giữ 20 tệp đầu và
  báo dưới hàng chờ "Mỗi tin tối đa 20 tệp. Đã bỏ qua 3 tệp." Gửi: chữ + mọi tệp thành **một** tin, theo thứ tự
  hàng chờ.
- **Thanh định dạng (FormattingToolbar):** nút **"A"** (IconButton, chữ A gạch chân, `aria-pressed`,
  `aria-controls`) bật/tắt một hàng `role="toolbar"` ngay trên ô nhập, ngăn với ô nhập bằng đường `--color-line`.
  IconButton 32 chia nhóm bằng vạch 1px × 20: **Đậm · Nghiêng · Gạch chân · Gạch ngang** | **Danh sách chấm ·
  Danh sách số** | **Link** | **Code · Khối code · Trích dẫn**. Nút phản ánh vùng chọn: đang áp dụng = nền
  `--color-accent-wash`, icon `--color-accent`, `aria-pressed="true"`. Tooltip có phím tắt ("Đậm (Ctrl+B)", macOS
  hiện ⌘). ←/→ di chuyển trong thanh (roving tabindex). Trạng thái bật/tắt nhớ theo người dùng. Dưới 768: thanh
  cuộn ngang trong ô soạn, không xuống dòng. Phím tắt và cú pháp gõ nhanh theo phase 07 của plan chat.
- **Hiển thị định dạng trong tin:** đậm 600; code inline `--font-mono` 13px trên `--color-sunk`, radius 4, padding
  1 × 4; khối code `--color-sunk`, radius 8, padding 10 × 12, cuộn ngang; trích dẫn: vạch trái 3px `--color-line`,
  chữ `--color-ink-muted`; link `--color-accent` gạch chân, mở tab mới. Không màu chữ hay font tuỳ ý.
- **Hộp chèn link (LinkDialog):** popover neo vào nút Link (hoặc Ctrl/Cmd+K), rộng 320, `--color-overlay`, radius
  12, padding 16: Input có label **Văn bản** (điền sẵn vùng chọn) và **Đường liên kết** (`type="url"`; focus vào
  đây nếu đã có văn bản). Huỷ (ghost) · **Chèn** (primary sm, disabled tới khi URL hợp lệ). Lỗi dưới ô khi blur:
  "Chỉ dùng liên kết bắt đầu bằng https://, http:// hoặc mailto:". Enter = Chèn, Esc = đóng, trả focus về ô
  nhập. Sửa link có sẵn: hộp mở với giá trị cũ và thêm nút "Bỏ liên kết".
- **Đính kèm trong tin:** sau văn bản, gap 8: ảnh trước, tệp sau. **1 ảnh:** giữ tỉ lệ, tối đa 360 × 320, radius
  10. **2 ảnh:** 2 ô vuông. **3 ảnh:** ô trái cao hai hàng + 2 ô phải. **4 ảnh:** lưới 2 × 2. **> 4:** lưới 2 × 2,
  ô thứ tư có pill "+N" `--color-overlay` ở góc dưới phải. Lưới rộng tối đa 360, gap 4, mỗi ô radius 8. Mỗi ảnh là
  `button` "Mở ảnh <tên tệp>", alt lấy từ tên tệp; khi đang tải: Skeleton đúng tỉ lệ từ kích thước đã lưu, không
  nhảy layout. **Thẻ tệp:** `--color-raised` (trong panel `--color-base`), radius 10, padding 10 14 10 10, rộng tối
  đa 340: ô icon 36 theo loại (bảng tính `success`, PDF `danger`, văn bản `info`, khác `--color-sunk`) + tên 600
  (cắt …) + "PDF · 1,2 MB" 12px mờ tabular; hover hiện IconButton Tải xuống; bấm mở xem trước trong Tài liệu.
- **Trình xem ảnh (ImageViewer):** modal toàn màn (`--z-modal`, focus trap), nền `--color-sunk`. Thanh trên nền
  `--color-base`, cao 56: PersonChip người gửi + giờ · tên tệp · dung lượng; phải: **Tải xuống**, **Đóng**. Ảnh
  giữa, `object-fit: contain`, đệm ngang 72 cho nút **Ảnh trước / Ảnh sau** (IconButton tròn 44 `--color-overlay`
  + shadow, giữa mép trái/phải; ẩn ở ảnh đầu/cuối, không quay vòng). Dưới ảnh: "2 / 4" tabular. ←/→ chuyển ảnh, Esc
  đóng và trả focus về ảnh đã bấm. Phạm vi: các ảnh của **cùng một tin**. Màn chạm: vuốt ngang chuyển ảnh, nút
  trước/sau nằm sát mép.

### Thông báo đẩy, cài app, mất kết nối (mockup `design/mockups/push.html`, mobile trước)

- **Thông báo đẩy (OneSignal):** năm loại, mỗi loại tắt được: tin nhắn trực tiếp, được nhắc tên, trả lời trong thread
  đang theo dõi, công việc giao cho bạn, đề nghị cần bạn duyệt (chỉ workspace tổ chức). Dòng nguồn "Nexus · tên
  nhóm/khu vực", tiêu đề là người gửi hoặc câu "X cần bạn duyệt", thân là đoạn đầu văn bản thuần (2 dòng) khi bật
  "Xem trước nội dung", nếu không thì "Tin nhắn mới trong …". Không ID. Gộp tin cùng cuộc trò chuyện trong 30s thành
  "N tin mới". Không gửi khi đang mở đúng cuộc trò chuyện, trong giờ yên lặng, hoặc khi người nhận không còn quyền đọc.
  Bấm: mở/focus app tới đúng cuộc trò chuyện hoặc thread, tin được nhắc có viền accent loé 2,4s.
- **Hỏi bật thông báo:** thẻ trong app (Card `--color-raised`, viền trong `--color-line`, radius 14, ô icon chuông
  40 nền accent wash, "Nhận thông báo khi có tin mới?" + một câu lợi ích + Để sau / **Bật thông báo**) hiện sau tin đầu
  tiên bạn gửi hoặc khi mở Thông báo; chỉ khi bấm Bật mới gọi hộp quyền của trình duyệt. "Để sau" ẩn 7 ngày, ba lần thì
  thôi. Cho phép → toast success; chặn → toast + hướng dẫn trong Cài đặt. iPhone chưa thêm vào màn hình chính: thẻ đổi
  thành lời mời thêm vào màn hình chính.
- **Cài đặt → Thông báo:** nhóm "Thiết bị này" (Switch thông báo đẩy, khoá + banner `danger-wash` có 3 bước mở lại khi
  trình duyệt chặn; banner `info-wash` cho iPhone chưa cài), "Báo cho tôi khi" (5 Switch theo loại, icon 32 radius 9 nền
  sunk), "Nội dung và giờ" (Xem trước nội dung, Giờ yên lặng + hai select từ/đến). Lưu ngay, không hộp xác nhận; tắt
  thiết bị thì các mục dưới mờ 50% nhưng giữ giá trị. Hàng ≥ 56, Switch 44 × 26.
- **Cài app:** Android — thanh nổi trên thanh đáy (icon app 44, "Cài Nexus vào màn hình chính" + một dòng mờ, Để sau /
  Cài đặt) từ lần mở thứ hai, chỉ ở danh sách, không trong cuộc trò chuyện; iPhone — sheet một lần với 3 bước có số
  (Chia sẻ → Thêm vào MH chính → Thêm). Tên "Nexus", icon chữ N trên nền accent.
- **Mất kết nối:** thanh `--color-warning-wash` dưới thanh trên (icon wifi gạch, "Mất kết nối. Đang thử kết nối lại…",
  Thử lại) sau 2s mất WebSocket hoặc offline; có lại → "Đã kết nối lại" success 2s rồi ẩn. Dữ liệu đang hiện giữ nguyên,
  ô soạn khoá; không đọc offline. **Bản mới:** thanh nổi "Có bản mới của Nexus" + Để sau / Tải lại; đang soạn dở thì
  "Tải lại sau khi gửi".

### Chat: tạo trong chat — bình chọn, công việc, tài liệu, phê duyệt (mockup `spaces.html` §6)

- **Nút +** (IconButton 32, icon cộng) đứng đầu hàng nút của ô soạn, trước nút đính kèm; có ở ô soạn chính và ô
  "Trả lời trong thread". Menu (popover lên trên): **Bình chọn · Công việc · Tài liệu mới · Đề nghị phê duyệt**.
  Workspace cá nhân không có Đề nghị phê duyệt. Mỗi mục mở **hộp tạo** (Modal 520, có seg đổi loại Bình chọn /
  Công việc / Tài liệu); gửi xong thành một tin chứa **thẻ**.
- **Thẻ (ChatCard):** nằm ngoài bong bóng, bám bên của tin, rộng tối đa 420; `--color-raised` (trong panel
  `--color-base`), viền trong 1px `--color-line`, radius 12, padding 12 × 14. Đầu thẻ: ô icon 32 radius 8 (Bình
  chọn accent, Công việc success, Tài liệu info, Phê duyệt warning, đều dạng wash) + tiêu đề 600 + dòng phụ 12px
  mờ "Loại · chi tiết". Chân thẻ: trạng thái + hành động phụ (Button ghost sm) căn phải. Không ID nào trên thẻ.
- **Bình chọn:** câu hỏi ≤ 200 ký tự, 2–10 lựa chọn, tuỳ chọn "Cho chọn nhiều đáp án", "Ẩn danh", "Đóng bình
  chọn" (Không tự đóng / Sau 1 giờ / Cuối ngày / Chọn ngày giờ). Mỗi lựa chọn là `button role=radio` (chọn nhiều:
  `checkbox`, ô vuông) cao ≥ 40, nền `--color-sunk`. **Chưa bỏ phiếu: không thấy kết quả**, chỉ thấy "N người đã
  bình chọn" (tránh theo số đông). Bỏ phiếu xong / đã đóng: thanh `--color-accent-wash` theo phần trăm (trượt
  280ms `--ease-out-expo`), số phiếu tabular, tối đa 3 avatar 20 người chọn (ẩn danh thì không). Đổi ý bằng bấm
  lựa chọn khác, "Rút phiếu" ở chân. Người tạo và quản lý nhóm đóng sớm qua menu Thêm; đóng rồi lựa chọn cao nhất
  có pill `ok` "Nhiều nhất", dòng phụ "Đã đóng lúc…". Phiếu realtime của người khác: thanh trượt tới giá trị
  mới, hàng tin phủ màu người bỏ phiếu (§7).
- **Công việc:** tên việc ≤ 200, Giao cho (Picker thành viên nhóm), Hạn (date). Thẻ: ô đánh dấu 20 radius 6
  (`role=checkbox`; xong = nền `--color-success`, tiêu đề gạch ngang mờ) cho người được giao và người tạo;
  PersonChip người làm; StatusPill Chưa làm / Đang làm / Đã xong; "Mở trong Công việc" sang tab Công việc của nhóm.
- **Tài liệu mới:** tên, loại (Văn bản / Bảng tính), "Lưu vào" mặc định thư mục của nhóm (Picker thư mục để đổi).
  Thẻ: vị trí thư mục, giờ sửa cuối, ai đang sửa (avatar), Button "Mở". Chọn tài liệu có sẵn vẫn qua nút đính kèm.
- **Đề nghị phê duyệt:** mở form đề nghị của Phê duyệt (detail panel) với "Gửi vào: tên nhóm". Thẻ: tiêu đề, loại
  và số tiền, **chuỗi duyệt** (avatar 20 + tên + trạng thái từng bước: Đã gửi / Đã duyệt giờ `success`, Đang chờ
  `warning`, Tiếp theo mờ; ngăn bằng "›"), StatusPill tổng ("Chờ … duyệt" `warn`, "Đã duyệt" `ok`, "Từ chối"
  `bad`), "Xem đề nghị". Người đang phải duyệt thấy thêm Duyệt / Từ chối ngay trên thẻ. Người không có quyền
  đọc đề nghị chỉ thấy tiêu đề và "Bạn không có quyền xem đề nghị này".
## 7. Motion

Mọi duration/easing là token. Chỉ animate `transform` và `opacity` (cộng `background-color` cho
lớp phủ realtime). Không `transition: all`. Đóng chạy bằng ~75% thời gian mở.

| Token | Giá trị | Dùng cho |
|---|---|---|
| `--duration-press` | 100ms | nhấn nút, toggle |
| `--duration-quick` | 160ms | hover, focus, menu, tooltip |
| `--duration-base` | 220ms | mở popover, toast, chèn hàng |
| `--duration-layout` | 280ms | panel, modal, sheet, sắp lại danh sách |
| `--ease-out` | `cubic-bezier(0.25, 1, 0.5, 1)` | vào, phản hồi |
| `--ease-out-expo` | `cubic-bezier(0.16, 1, 0.3, 1)` | panel, layout |
| `--ease-in` | `cubic-bezier(0.5, 0, 0.75, 0)` | ra |
| `--ease-spring` | `cubic-bezier(0.175, 0.885, 0.32, 1.275)` | **chỉ** reaction pop |

| Chuyển động | Vào | Ra |
|---|---|---|
| Detail panel | `translateX(16px) scale(.985)` → 0, 280ms expo | 210ms ease-in |
| Modal | scrim fade 220ms + `translateY(8px) scale(.98)` → 0, 280ms expo | 210ms ease-in |
| Popover / menu | `translateY(-4px)` + fade, 160ms | 120ms |
| Toast | `translateY(12px)` + fade, 220ms | 160ms |
| Chèn / xoá hàng | layout animation (FLIP) 220ms | fade + co chiều cao 160ms |
| Chuyển route | content fade 160ms, không trượt | không |
| Reaction | scale 0.6 → 1 spring 280ms | fade 120ms |
| Hover actions trên tin | fade 160ms, không translate | fade 120ms |
| Thanh định dạng | `translateY(4px)` + fade, 160ms | 120ms |
| Hộp chèn link, menu Đính kèm | như Popover / menu | như Popover / menu |
| Vùng thả tệp | fade 160ms | fade 120ms |
| Mục vào hàng chờ tệp | `scale(.96)` + fade, 160ms | `scale(.96)` + fade 120ms, mục sau dời bằng FLIP 220ms |
| Tiến trình tải tệp | thanh `scaleX` theo tiến độ, 160ms ease-out mỗi bước | xong: thanh fade 160ms |
| Nút nổi "Tin mới nhất" / "Nhảy tới tin chưa đọc" | `translateY(8px)` + fade, 220ms; đổi nhãn: fade 160ms | 160ms |
| Cuộn tới vạch "Tin mới" / cuối luồng | cuộn mượt của trình duyệt | không |
| Vạch "Tin mới" | tĩnh khi mở | fade 220ms (khi bạn gửi tin) |
| Số "N trả lời", pill "M mới" | đổi số fade 160ms; pill vào `scale(.6)` → 1 + fade 160ms ease-out | fade 160ms khi đã đọc |
| Panel thread ↔ panel Thread (cùng khe) | đổi nội dung fade 160ms, không trượt | không |
| Trình xem ảnh | scrim fade 220ms + ảnh `scale(.98)` → 1, 280ms expo | 210ms ease-in |
| Đổi ảnh trong trình xem | `translateX(±16px)` + fade theo hướng, 160ms | không |

**Realtime (bản sắc của hệ):**
- Thay đổi do **người khác** gây ra: avatar tác giả loé vòng `--color-person-N-wash` 900ms; hàng
  được phủ `--color-person-N-wash`, giữ 40% thời gian rồi tan, tổng **2400ms**; nhãn nhỏ "vừa
  gửi / vừa đổi tên / vừa duyệt" màu `--color-person-N` biến mất cùng lớp phủ.
- Thay đổi của **chính mình**: không phủ màu, chỉ animation chèn hàng.
- **Gộp:** ≥ 3 thay đổi trong cửa sổ 2000ms ở cùng một danh sách → một dòng "Vinh và 2 người khác
  vừa cập nhật", tối đa một lớp phủ mỗi hàng.
- **Chat, tin mới ở luồng chính** do người khác: chèn hàng + lớp phủ như trên, nhãn "vừa gửi". Bạn đang ở cuối
  luồng thì tự cuộn; đang cuộn lên thì không cuộn, nút "Tin mới nhất" hiện kèm số.
- **Chat, trả lời trong thread bạn đang theo dõi** do người khác: hàng tóm tắt thread phủ
  `--color-person-N-wash` của người trả lời (2400ms, như trên), avatar của họ vào đầu cụm và loé vòng, số trả lời
  và giờ cuối đổi bằng fade, pill "M mới" vào hoặc tăng, nhãn "vừa trả lời" màu người đó. Mục tương ứng trong panel
  Thread trượt lên đầu (FLIP 280ms) và mang cùng lớp phủ. Thread đang mở ở panel: chèn trả lời + lớp phủ, không
  "mới" (đã đọc).
- **Chat, trả lời trong thread không theo dõi:** chỉ cập nhật số trả lời, cụm avatar và giờ cuối (fade 160ms);
  **không** lớp phủ, không "mới". Theo dõi là cách người dùng chọn mức chú ý; luồng đông không nhấp nháy vì những
  thread họ không theo.
- **Presence:** cụm avatar người đang xem ở header (tối đa 3 + "+N"), vào/ra fade 220ms.
- **Kết nối:** bình thường không hiện gì. Mất kết nối > 3s → dải `warning-wash` dưới header
  "Đang kết nối lại…"; nối lại → "Đã cập nhật" 2s rồi ẩn.

**Giảm chuyển động** (`prefers-reduced-motion: reduce`): bỏ mọi `transform`; giữ fade ≤ 120ms;
lớp phủ realtime vẫn hiện (là thông tin) nhưng không loé avatar; layout animation tắt; cuộn tới vạch
"Tin mới" hay cuối luồng là nhảy thẳng; trình xem ảnh và đổi ảnh chỉ fade; tiến trình tải đổi bước không
transition.

Thư viện: `motion` (`motion/react`) cho `AnimatePresence` (exit) và `layout`; mọi thứ còn lại là
CSS. Biến thể dùng chung ở `frontend/src/lib/motion.ts`.

## 8. Content & identifiers

- Không UUID, id, khoá ngoại, mã nội bộ nào lên màn hình. Người → PersonChip. Thực thể → tên
  hoặc tiêu đề. Trạng thái → nhãn tiếng Việt.
- Audit / hoạt động luôn: **ai** (PersonChip) + **làm gì** (động từ) + **khi nào** (giờ tương đối,
  tooltip giờ đầy đủ).
- Định dạng qua `frontend/src/lib/format.ts`: tiền `12.450.000 ₫`, ngày `09/10/2026`, giờ `09:14`,
  tương đối "5 phút trước", dung lượng `412 KB`.
- Văn phong: câu ngắn, chủ động, nói điều sẽ xảy ra ("Duyệt", rồi toast "Đã duyệt"). Lỗi nói rõ
  sai gì và cách sửa. Không dùng dấu gạch ngang dài trong giao diện.

## 9. Accessibility

WCAG 2.2 AA. Chữ ≥ 4.5:1, chữ lớn và UI ≥ 3:1 trên nền thật. Mọi phần tử tương tác là
`button`/`a`/control thật, có hover, `:focus-visible`, active, disabled. Touch target ≥ 44px.
Màu không bao giờ là kênh duy nhất (trạng thái có nhãn; lớp phủ realtime có nhãn chữ).

## 10. Enforcement

`frontend/eslint.config.js` khoá ở `error`: cấm hex/rgba, màu palette Tailwind thô, cỡ chữ và
giá trị tuỳ ý (`text-[`, `z-[`, `tracking-[`), `transition-all`, `duration-<số>`, `animate-bounce`,
`<h1-6>`/`<p>` thô ngoài `Heading`/`Text`. Exemption phải có lý do trong comment.
