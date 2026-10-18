# Nexus Design System: Tín hiệu

> **Nguồn thiết kế duy nhất của frontend.** Chọn ngày 2026-10-09 từ
> [brainstorm](plans/261009-0449-project-wide-refactor/reports/brainstorm-261009-0457-new-ui-direction.html)
> (hướng B, mượn bảng kẻ mảnh của hướng A). Mockup đã duyệt nằm trong `design/mockups/`.
> `.stitch/` là lịch sử, không dùng làm nguồn nữa. Code render đúng file này và mockup; không
> thiết kế trong code. Muốn đổi thiết kế: sửa file này trước, trong cùng PR.

## 1. Thesis

Nexus là nơi nhiều người cùng sửa một thứ: tin nhắn, thư mục, đề nghị phê duyệt. Câu hỏi
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

**Màn gọi** (mockup `design/mockups/video-call.html`): luôn tối, **giống nhau ở hai giao diện** — video đọc tốt trên nền
tối và tách rõ "đang trong cuộc gọi" khỏi workspace. Chỉ dùng trong stage của cuộc gọi.

| Token | Giá trị | Dùng cho |
|---|---|---|
| `--color-stage` | `oklch(16% 0.012 200)` | nền stage, xem trước camera |
| `--color-stage-tile` | `oklch(23% 0.016 200)` | ô người không bật camera, sheet trên stage |
| `--color-stage-raised` | `oklch(28% 0.018 200)` | nút điều khiển, chip, popover trên stage |
| `--color-stage-hover` | `oklch(33% 0.018 200)` | hover/nhấn trên stage |
| `--color-on-stage` / `--color-on-stage-muted` | `oklch(95% 0.008 190)` / `oklch(75% 0.014 195)` | chữ, icon trên stage |
| `--color-stage-accent` | `oklch(76% 0.11 178)` | viền người đang nói, sóng micro, nút đang bật, badge |
| `--color-stage-danger` | `oklch(60% 0.19 25)` | Rời / Từ chối |

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
  webfont): chỉ cho code và khối code trong tin nhắn, 14px.
- `font-variant-numeric: tabular-nums` cho giờ, số tiền, dung lượng, bộ đếm, cột bảng.
- Tiếng Việt có dấu chồng (ặ, ẫ, ổ): line-height thân chữ ≥ 1.5, tiêu đề ≥ 1.2, không tracking âm
  quá `-0.01em`.

Scale 1.2, `rem`:

| Token | Size / line | Weight | Dùng cho |
|---|---|---|---|
| `text-xs` | 12 / 16 | 500 | nhãn phụ, giờ, badge, "đã sửa" |
| `text-sm` | 14 / 21 | 400 | mặc định giao diện: ô bảng, mô tả, form; tiêu đề mục chat, hàng tóm tắt thread, trích dẫn, code trong tin |
| `text-md` | 15 / 23 | 400 | **đọc chat trên máy tính**: tin nhắn, tên người gửi (600), hàng cột điều hướng, tên hàng ở feed |
| `text-base` | 16 / 24 | 400 | input và ô soạn (tránh zoom mobile), **tin nhắn và tên hàng danh sách trên điện thoại**, đọc dài |
| `text-lg` | 19 / 26 | 600 display | tiêu đề panel, tên kênh |
| `text-xl` | 23 / 30 | 700 display | tiêu đề trang |
| `text-2xl` | 28 / 34 | 600 display | số tiền, con số trọng tâm |

Chữ chat cố ý to hơn Google Chat (14px) 1px trên máy tính và 2px trên điện thoại (người dùng chốt 2026-10-11:
"chỉ phần chat", tin nhắn điện thoại 16px). Màn dày dữ liệu (bảng, quản trị) và chữ giao diện giữ 14.

Nhãn viết hoa (`label`): 12px, 600, `letter-spacing: 0.06em`, tối đa một nhãn cho mỗi panel.

### Logo và tên (mockup `design/mockups/logo.html`, hướng A "Lối đi", duyệt 2026-10-11)

Tên sản phẩm hiển thị ở mọi nơi là **Nexus** (không "Nexus Hub", không "Nexus Enterprise"). Tên trong
mã backend, email và tài liệu kỹ thuật không đổi.

- **Dấu hiệu:** chữ N là một đường đi qua bốn nút trên lưới 48: nét 6, nút r5, nút cuối (nơi tới
  được) r5.5. Hai màu chỉ trên nền trung tính: nét và ba nút đầu `--color-accent`, nút cuối
  `--color-person-1`. Trên nền accent chỉ một màu `--color-on-accent`. Component: `NexusMark`,
  `NexusLogo` ở `frontend/src/components/brand/`; không vẽ lại dấu ở chỗ khác.
- **Lockup:** dấu, rồi chữ "Nexus" Bricolage Grotesque 700, tracking −0.01em, cao chữ hoa bằng ~62%
  cạnh dấu; dòng phụ "Không gian làm việc chung" (12px, `--color-ink-muted`) chỉ ở brand pane.
  Khoảng cách dấu-chữ 10px.
- **Khoảng trống quanh dấu:** tối thiểu bằng bán kính một nút (5/48 cạnh dấu).
- **Cỡ nhỏ:** từ 24px trở xuống dùng bản nhỏ (nét 8, bỏ ba nút đầu, giữ nút đích) để favicon 16px
  không thành vệt mờ. Từ 25px dùng bản đầy đủ.
- **Text alternative:** đứng một mình có `role="img"` tên "Nexus"; cạnh chữ "Nexus" thì `aria-hidden`.
- **Icon app và favicon** nằm ở `frontend/public/` (`favicon.svg` đổi màu theo giao diện của tab,
  `favicon-32/16.png`, `apple-touch-icon.png`, `icon-192/512.png`, `icon-maskable-192/512.png`): dấu
  một màu trên ô accent; bản maskable tràn viền, dấu trong ô 320/512 nằm gọn vòng an toàn 80%.

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
│ (dải    │ (kênh, cây   │                           │ (có điều     │
│  72px)  │  thư mục…)   │                           │  kiện)       │
└─────────┴──────────────┴───────────────────────────┴──────────────┘
```

- **Sidebar** (`--color-sunk`): workspace switcher (tên + avatar, display font), 6 mục chính
  **Tin nhắn · Lịch · Tài liệu · Phê duyệt · Tài sản · Danh bạ**, đáy: **Thông báo** (mở NotificationPanel,
  §6), **Quản trị** (khi có quyền),
  **Cài đặt**, người dùng hiện tại. Tài sản nằm trong shell này, không có layout riêng.
- **Menu chính: mở rộng hoặc dải icon** (mockup `design/mockups/app-rail.html`). Hai trạng thái, người dùng tự
  chọn, nhớ theo người trên từng máy; **mặc định thu gọn** (người dùng chốt 2026-10-11, cùng phím tắt, mục Thông báo, nền bo nhẹ) ở mọi bề rộng máy tính và một lựa chọn chung cho mọi
  phân hệ. Cả cột là `<nav aria-label="Điều hướng chính">`.
  - *Nút chuyển:* IconButton 32 (icon khung có cột trái) nằm riêng hàng trên cùng ở cả hai trạng thái, tâm cách
    mép trái 36px nên không đổi chỗ. `aria-expanded` (true = mở rộng), `aria-label` "Thu gọn menu chính" /
    "Mở rộng menu chính", phím tắt **Ctrl+\\** (⌘\\ trên macOS, `aria-keyshortcuts`), ghi trong tooltip.
  - *Mở rộng (232px):* như hiện nay: workspace switcher (logo 32 + tên + dòng phụ + chevron), NavRow, đáy
    Thông báo · Quản trị · Cài đặt · hàng người dùng có nút Đăng xuất.
  - *Thu gọn (72px, đệm 12/4):* ô chữ cái workspace 40 (radius 10) trong nút 48; mỗi mục là link 64×56: icon 20
    trong nền 48×28 radius 10 + nhãn 11px/500 bên dưới (đủ tên, không viết tắt). Đang chọn: nền
    `--color-accent-wash`, icon `--color-accent`, nhãn `--color-ink` 600, `aria-current="page"`; hover nền
    `--color-hover`. Số đếm: pill 16 ở góc trên phải icon, viền 2px `--color-sunk`, `99+`, ẩn khi 0; chữ ẩn
    ", N chưa đọc" trong link. Đáy: Thông báo · Quản trị · Cài đặt, rồi avatar 32 (presence) mở menu tài
    khoản: tên, vai trò, trạng thái, Cài đặt tài khoản, Đăng xuất.
  - *Phiên bản (2026-10-18):* để người dùng biết máy mình đã nhận bản mới sau một lần triển khai. Dạng
    `2026.10.18 · 6368e8e` (ngày build giờ Việt Nam · 7 ký tự commit), chạy máy dev ghi `dev`. Một dòng "Phiên bản …"
    `text-xs` `--color-ink-muted`, chọn chép được, ở cuối màn Cài đặt (mọi tab), cuối menu tài khoản (máy tính, sau
    divider) và dòng Giới thiệu của trang Tôi (điện thoại). Thanh "Có bản mới" không đổi.
  - *Đổi workspace:* bấm ô chữ cái (hay tên khi mở rộng) mở cùng một menu; đầu menu ghi đầy đủ tên workspace
    đang mở (xuống dòng, không cắt) + dòng phụ ("Workspace cá nhân" hoặc tên tổ chức) · vai trò; danh sách
    "Chuyển sang" cũng xuống dòng thay vì cắt. Workspace cá nhân không có Tài sản, Phê duyệt, Quản trị; vẫn có Lịch (chỉ lịch cá nhân).
  - *Phòng ban chỉ có ở workspace tổ chức.* Workspace cá nhân không hiện trường, cột, bộ lọc, nhóm trong ô
    chọn người hay dòng phụ nào nói về phòng ban (Hồ sơ, Danh bạ, ô chọn người).
  - *Tooltip* chỉ ở chỗ chữ bị ẩn (ô workspace, avatar, nút chuyển): `--color-overlay`, `--shadow-overlay`,
    radius 8, 12px, cách 8px bên phải, `--z-tooltip`; trỏ chuột 400ms, focus bàn phím hiện ngay, Esc ẩn.
    Tooltip không bao giờ là nguồn tên duy nhất.
  - *Chuyển trạng thái:* nội dung cột fade ra 100ms `--ease-in`, bề rộng cột 280ms `--ease-out-expo`, nội dung
    fade vào 120ms; giảm chuyển động thì đổi ngay. Không còn "xem tạm" khi trỏ chuột vào dải.
  - Dưới 1024 không có menu này: vỏ điện thoại (ô tìm viên thuốc, thanh đáy phân hệ, trang Tôi) như dưới đây.
- **List panel** chỉ ở Tin nhắn, Tài liệu và Danh bạ: rộng 280, nền `--color-base`, hàng 40 chữ 15px như cột điều
  hướng của Chat, mục đang chọn theo NavRow (nền `--color-raised`, chữ 600). **Detail panel** mở bằng chọn mục, đóng bằng Esc.
  - *Danh bạ* (§6 "Danh bạ"): list panel **chỉ có danh mục** (Phòng ban · Vai trò · Nhóm của tôi + lối tắt Phòng của
    tôi; workspace cá nhân: Thành viên · Nhóm của tôi), không liệt kê cây, vai trò, nhóm hay người; nội dung của danh
    mục mở ở cột content. Tablet (768–1023): list panel thành hàng chip danh mục dưới tiêu đề, cùng thứ tự. Điện thoại:
    màn đầu là chồng ô danh mục (tầng 1), rồi danh mục / phòng (tầng 2), rồi hồ sơ toàn màn (tầng 3).
- **Breakpoints:** ≥ 1280 đủ 4 cột · 1024–1279 detail panel thành overlay bên phải ·
  768–1023 sidebar thành rail, list panel thành cột thay thế content (rail chưa làm: dưới 1024 dùng vỏ điện thoại
  bên dưới) · < 768 vỏ điện thoại: ô tìm viên thuốc, thanh đáy phân hệ, mỗi màn một cột, detail panel thành sheet từ dưới lên.
- **Điều hướng di động (dưới 1024; mockup `design/mockups/mobile-shell.html` §1, §1b, §2, §8, duyệt 2026-10-19; thay
  menu ☰ và thanh đáy chỉ-của-Chat của `chat-mobile-home.html`):** Lark cho cấu trúc (thanh đáy chứa phân hệ, người
  dùng tự chọn, Thêm dạng lưới), Google Chat cho hình thức (viên thuốc nổi + nút + tách riêng, ô tìm viên thuốc).
  - *Thanh trên (mọi màn danh sách của mọi phân hệ):* ô tìm dạng viên thuốc cao 52, radius full, `--color-sunk`, cách mép
    12, dưới `env(safe-area-inset-top)`. Trái: **ô chữ cái workspace** 32 (radius 9) trong vùng chạm 44 — nền `--color-accent`
    ở workspace tổ chức, hue của chính người dùng ở workspace cá nhân; chấm 9px `--color-accent` viền 2px `--color-sunk`
    khi workspace khác có tin chưa đọc; `aria-label` "Đổi workspace, đang ở <tên>"; chạm mở sheet chọn workspace. Giữa:
    nút chữ "Tìm …" của phân hệ (16px mờ) mở ô tìm của §6 "Tìm cho mọi tác vụ". Phải: avatar 32 trong vùng chạm 44 mở
    trang **Tôi** (§6 "Cài đặt"). Không có ☰. Màn chi tiết (một cuộc trò chuyện, một tài liệu, một đề nghị, trang cài
    đặt) dùng thanh có nút quay lại, không có ô tìm này.
  - *Hàng tiêu đề của phân hệ:* tên font display 700 26px; tối đa một nút phụ (Thread, đổi kiểu xem) rồi **⋯**; không dòng
    phụ đề (tên workspace đã ở ô chữ cái). ⋯ mở menu nổi rộng 240, hàng 48, dòng đầu luôn là **Cài đặt <phân hệ>** (trừ
    Danh bạ), sau đó lối tắt riêng (Thùng rác, Đã gắn sao, Mục mới…). Hàng tab/chip cuộn ngang với `mask-image` mờ 15% mép
    phải khi còn tab bị che; tab đang chọn luôn cuộn vào khung; không bao giờ cắt chữ giữa từ. Trên điện thoại không có ô
    tìm thứ hai trong trang.
  - *Thanh đáy (mọi phân hệ):* viên thuốc nổi cao 64, cách trái 12, đáy 22 + safe-area, phải chừa nút +; nền
    `--color-overlay` 90% + blur 16, `--shadow-overlay` + viền trong `--color-line`, `--z-sticky`, `nav aria-label="Phân hệ"`.
    5 ô đều: **4 phân hệ người dùng chọn + Thêm**; mỗi ô icon lucide 24 trong pill 48×30 + nhãn 11px (luôn có chữ). Đang ở:
    pill `--color-accent-wash`, icon accent, nhãn 600, `aria-current="page"`. Số chưa đọc: pill góc trên phải icon, `99+`,
    ẩn khi 0, đọc trong `aria-label`; Thêm có chấm khi phân hệ trong đó có việc mới. Mặc định **Chat · Lịch · Tài liệu · Phê
    duyệt** ở workspace tổ chức, **Chat · Lịch · Tài liệu · Danh bạ** ở workspace cá nhân; lựa chọn lưu theo người, riêng
    từng workspace. Có ở mọi màn danh sách; ẩn ở màn chi tiết, trang cài đặt và khi bàn phím mở.
  - *Nút + của phân hệ:* 64×64, radius 20, `--color-accent-wash`, icon + 28 accent, cạnh phải thanh đáy, cùng đáy. Là hành
    động tạo của phân hệ: Trò chuyện mới (sheet Nhắn tin trực tiếp · Tạo nhóm · Tạo mục) · Tạo/Tải lên · Sự kiện mới · Tạo đề
    nghị · Thêm tài sản; Danh bạ không có +, ô đó để trống.
  - *Cuộn:* cuộn xuống quá 24px thì nhãn gập và thanh cao 52 (240ms `--ease-out-expo`), tiêu đề và chip cuộn đi, viên thuốc
    tìm dính trên; cuộn lên một chút là mở lại. Chạm ô đang ở: về đầu danh sách (rồi về màn gốc của phân hệ; ở Chat, chạm
    lần nữa bỏ lọc về Tất cả).
  - *Đi nhanh trong Chat:* nhấn giữ ô Chat 400ms mở menu nổi rộng 230 trên thanh đáy: Trực tiếp · Được nhắc · Thread đang theo
    dõi · Đã gắn sao, mỗi dòng 48 có pill số.
  - *Thêm (sheet):* khối workspace (ô chữ cái 40, tên, dòng mờ "Workspace tổ chức · N thành viên" / "Workspace cá nhân · chỉ
    bạn", chevron mở danh sách workspace: ✓ ở workspace đang mở, số chưa đọc của workspace khác, "Tạo workspace" cuối) ·
    "Tất cả phân hệ" + link "Sửa thanh đáy" · lưới 4 cột ô 52 radius 16 (viền trong 2px accent = đang ở thanh đáy): Chat ·
    Lịch · Tài liệu · Phê duyệt · Tài sản · Danh bạ · Thông báo. Workspace cá nhân: không Phê duyệt, Tài sản, Quản
    trị; Quản trị nằm trong Cài đặt workspace (nút ⚙ trên khối workspace).
  - *Sửa thanh đáy (trang):* "Ở thanh đáy (4/4)" — 4 hàng kéo được (nút ↑/↓ cho bàn phím) — rồi "Trong Thêm" với "Đưa lên";
    "Về mặc định"; Xong ở thanh trên. Mở từ Thêm, từ Tôi → Thanh đáy, nhấn giữ ô Thêm, hoặc nhấn giữ vùng trống của thanh. Quản trị không đưa
    lên được.
  - *Hành vi sheet và trang:* modal đầy đủ (focus trap, focus vào hàng đầu khi mở, trả focus về nút đã mở, nền sau inert).
    Đóng bằng Esc, chạm scrim, vuốt mép trái (trang) hoặc kéo sheet xuống quá 25% / vuốt nhanh. Chuyển động §7: scrim fade
    220ms; sheet `translateY(100%)` → 0 và trang `translateX(100%)` → 0 trong 280ms expo, ra 210ms ease-in. Giảm chuyển
    động: chỉ fade ≤ 120ms.
  - Danh sách phía trên thanh đáy đệm đáy 108 + safe-area để hàng cuối không bị che. Toast nổi trên thanh đáy (cách
    16), ở màn không có thanh đáy thì cách đáy 16 + safe-area.
  - *Đổi workspace* đổi luôn thanh đáy, Thêm, Tôi và phạm vi tìm theo loại workspace mới. **Loại workspace cố định khi
    tạo**: không có chuyển cá nhân ↔ tổ chức theo chiều nào; làm việc nhóm thì tạo một workspace tổ chức mới.

## 6. Components

Primitive sống ở `frontend/src/components/primitives/`; không component nào tự vẽ lại chúng.

**Nguyên tắc tinh gọn:** thao tác nằm trên chính đối tượng. Muốn đổi ảnh thì chạm vào ảnh; muốn đổi tên thì chạm
vào tên. Không thêm hàng nút "Đổi…" / "Bỏ…" cạnh đối tượng, không thêm dòng hướng dẫn chiếm chỗ (giới hạn tệp
nằm ở `accept` và câu báo lỗi). Máy không có chuột không có hover, nên dấu hiệu sửa phải luôn hiện ở đó (`pointer: coarse`).

- **Button:** `primary` (accent), `secondary` (`--color-hover` nền), `ghost`, `danger`. Cao 36
  (`sm` 32), padding ngang 14, radius 8, 14px/600. Hover: nền sáng/tối một bậc; press
  `scale(0.98)` 100ms; focus ring 2px `--color-focus` offset 2px. Loading: spinner thay icon,
  giữ nguyên chiều rộng. Một nút primary mỗi view.
- **IconButton:** 32×32 hit area ≥ 44 trên touch (`::before` inset −6px), `aria-label` bắt buộc.
- **Input / Select / Textarea:** cao 40, 16px, nền `--color-raised`, không viền; focus = ring.
  Label luôn có (không dùng placeholder làm label). Lỗi dưới ô, `aria-describedby`, validate khi blur.
- **Avatar:** tròn, 20/24/32/40 (64 chỉ cho thẻ hồ sơ trong Danh bạ và Cài đặt), nền `--color-person-N`, 2 chữ cái đầu của **tên hiển thị**
  (không bao giờ của id). Có ảnh thì dùng ảnh. Presence: chấm 8px `success` ở góc dưới phải.
- **AvatarEdit** (`patterns/AvatarEdit`, mockup `design/mockups/avatar-edit.html` §1, §2, §5): "chạm ảnh để đổi" cho
  ảnh đại diện (`circle`) và ảnh nhóm (`square`, bo = 1/4 cạnh, 16 ở cỡ 64). Một `button` duy nhất bọc Avatar (44/52/64),
  `aria-label` do nơi dùng đặt ("Đổi ảnh đại diện", "Đổi ảnh nhóm"), `aria-haspopup="menu"`, `aria-expanded`. Không có quyền
  sửa thì chỉ là Avatar, không nút, không lớp phủ. Trạng thái: **nghỉ** không thêm gì; **hover / focus-visible / menu mở**
  lớp phủ `--color-scrim` + icon máy ảnh 20, 160ms (`--duration-quick`); **cảm ứng** (`pointer: coarse`) huy hiệu máy ảnh 24
  `--color-overlay` + `--shadow-overlay` ở góc dưới phải, luôn hiện; **đang tải** ảnh mờ 60%, vòng 3px `--color-line` với cung
  `--color-accent` theo tiến độ (không có số thì nhấp nháy), tên nút thành "Đang tải ảnh, 64%", `aria-busy`, không mở được;
  **lỗi** huy hiệu cảnh báo `--color-danger` và dòng chữ 12px `--color-danger` dưới ảnh, `role="alert"`, nối `aria-describedby`,
  dùng đúng câu của `avatarErrorMessage` (không hiện chữ của máy chủ). Chọn xong tải lên ngay, không đợi "Lưu thay đổi".
  Từ 768 trở lên là Popover `role="menu"`: "Đổi ảnh" (icon tải lên) và "Bỏ ảnh" (`danger`, chỉ khi đã có ảnh); chưa có ảnh thì
  menu chỉ còn "Tải ảnh lên". Dưới 768 là bottom sheet (NavSheet) hàng 52: Chụp ảnh (ô nhập tệp `capture="user"`) · Chọn ảnh ·
  Bỏ ảnh (chỉ khi có ảnh) · vạch · Huỷ. Bàn phím: Enter / Space mở, ↑↓ Home End đi trong menu, Esc đóng và trả focus về ảnh.
  Tệp được kiểm (JPG/PNG, ≤ 5 MB) trước khi gửi.
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
  không scrim, Esc / bấm ngoài / Đóng thì đóng, trả focus về NavRow. Dưới 1024 là màn toàn màn mở từ
  Thêm → Thông báo hoặc ô Thông báo ở thanh đáy (§5), có Quay lại. Đầu: tiêu đề `text-lg` + "N chưa đọc" mờ + Button `ghost sm` "Đánh dấu tất cả đã đọc"
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

Thứ tự trong cột: ô tìm (xem "Gợi ý trong ô tìm") · nút "Trò chuyện mới" · lối tắt (Trang chủ, Được nhắc tới, Đã gắn sao) · **mục của bạn** ·
Tin nhắn trực tiếp · Nhóm · "Duyệt nhóm". ("Tạo mục" nằm trong menu "Trò chuyện mới", không có nút riêng cuối cột —
người dùng chốt 2026-10-11.)

- **Mục (ChatSection):** người dùng tự tạo để gom nhóm và tin nhắn trực tiếp, như Google Chat. Mục là **của
  riêng từng người** trong từng workspace; người khác không thấy và không bị ảnh hưởng. Tên 1–40 ký tự, không
  trùng (không phân biệt hoa thường) trong các mục của bạn. Một cuộc trò chuyện nằm trong **tối đa một** mục;
  đã vào mục thì không lặp lại ở Tin nhắn trực tiếp hay Nhóm. Mục chỉ là cách sắp xếp, không đổi quyền hay
  thành viên.
- **Tiêu đề mục:** `button` có `aria-expanded`, chevron 16 xoay −90° khi thu gọn (160ms), tên 14px/600
  `--color-ink` (mục mặc định `--color-ink-muted`). Thu gọn thì cuối tiêu đề hiện pill đếm = tổng chưa đọc
  trong mục. Mục của bạn có IconButton 28 "⋯" (hiện khi hover hoặc focus) mở menu: **Đổi tên mục · Chuyển
  lên · Chuyển xuống** (mục đầu/cuối thì mục tương ứng disabled) · divider · **Xoá mục** (`danger`). Mục mặc
  định có nút "+" (Nhắn tin trực tiếp / Tạo nhóm) như cũ, không đổi tên hay xoá được. Trạng thái thu gọn
  nhớ theo thiết bị.
- **Tạo / đổi tên:** popover nổi trong cột (không modal): ô "Tên mục" + Huỷ / Tạo (Lưu); Enter xác nhận,
  Esc đóng, lỗi nói rõ ("Nhập tên mục", "Bạn đã có mục tên này"). Mở từ "Tạo mục" trong menu "Trò
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
- **Mobile:** chip Mục dùng cùng thứ tự và cùng mục; menu ⋯ thành sheet đáy (xem "Chat trên điện thoại").
- **Lối tắt Được nhắc tới / Đã gắn sao** (mockup `design/mockups/shortcuts.html`): "Được nhắc tới" có pill đếm = số tin nhắc tên bạn chưa đọc (99+); là tập con của Trang chủ, không cộng vào số trên thanh đáy. Màn Được nhắc tới: mọi tin (tin chính và trả lời trong thread) nhắc tên bạn ở các cuộc trò chuyện bạn đọc được, mới nhất trên, chia theo vạch ngày; mỗi mục là liên kết: icon cuộc trò chuyện 24 + tên 13px · (trả lời) "Trả lời trong thread của X: “tin gốc…”" 12px mờ · Avatar 32 + tên + giờ · bong bóng rút gọn 3 dòng; @tên bạn là chữ accent 600 trên `--color-accent-wash` bo 4 (cả trong luồng). Chưa đọc: chữ `--color-ink`, tên 600, chấm 8 accent + chữ ẩn; đã đọc: `--color-ink-muted`. Xem danh sách không đánh dấu đã đọc. Bấm: mở cuộc trò chuyện, tin vào giữa, bong bóng loé viền accent 2,4s; trả lời thì mở thêm panel thread và loé trong panel. 25 mỗi trang + "Tải thêm"; Skeleton 4; Rỗng "Chưa ai nhắc tên bạn…" + "Về Trang chủ"; Lỗi + "Thử lại"; tin nhắc mới chèn đầu với lớp phủ màu người.
- **Đã gắn sao**: gắn sao là của riêng bạn, áp cho cuộc trò chuyện và từng tin. Cột điều hướng có mục hệ thống **Đã gắn sao** ngay dưới lối tắt (ẩn khi trống, thu gọn được, không ⋯/+); cuộc trò chuyện đã gắn sao chỉ hiện ở đó (người dùng chốt 2026-10-11), vẫn nhớ mục cũ và về lại khi bỏ sao; menu ⋯ của nó chỉ có Bỏ gắn sao · Tắt thông báo. Gắn/bỏ sao cuộc trò chuyện: menu ⋯, menu tên ở đầu cuộc trò chuyện (đã gắn thì sao 16 accent sau tên), sheet nhấn giữ. Gắn sao tin: menu Thêm (sau Theo dõi thread) và sheet nhấn giữ; tin đã gắn có sao 12 accent sau giờ. Màn Đã gắn sao: phần Cuộc trò chuyện (hàng như cột điều hướng + IconButton sao `aria-pressed`) và phần Tin nhắn (thẻ như Được nhắc tới, xếp theo lúc gắn sao), mỗi phần có trạng thái rỗng riêng. Mọi thao tác lạc quan; toast "Đã gắn sao “…”"/"Đã bỏ gắn sao …" + Hoàn tác, gắn sao tin + "Xem"; lỗi hoàn lại + toast lỗi + Thử lại.
- **Điện thoại**: hai lối tắt mở từ Đi nhanh (nhấn giữ ô Chat, §5; Được nhắc có pill đếm) và từ Cài đặt Chat (§6 "Cài đặt"); hai màn mở toàn màn có Quay lại.

### Chat: luồng in-line, thread, ô soạn (mockup `design/mockups/spaces.html` §2–§5; §2 là tin nhắn trực tiếp với tin hai bên)

Nhóm và tin nhắn trực tiếp dùng **chung một mô hình in-line threading** như Google Chat. Không có thẻ chủ đề:
mỗi tin là một hàng trong luồng theo thời gian; tin nào có trả lời thì có hàng tóm tắt thread ngay dưới. Enter
trong ô soạn chính gửi một tin vào luồng, không mở gì thêm; trả lời đi vào thread qua hover action hoặc hàng
tóm tắt. Panel thread, panel Thread của nhóm và panel Thành viên dùng chung khe detail panel 360px (§5): mở cái
này thì cái kia đóng.

- **Rời nhóm** (mockup `design/mockups/leave-group.html`, 2026-10-18): thành viên tự rời một nhóm. Không có ở tin
  trực tiếp và #general (người của #general là người của workspace). Ba lối vào, cùng một hộp xác nhận:
  menu tên ở đầu cuộc trò chuyện (máy tính) có MenuSeparator rồi MenuItem "Rời nhóm" (icon LogOut, chữ
  `--color-danger`) sau "Thông tin nhóm"; đáy panel Thành viên (máy tính và Cài đặt nhóm trên điện thoại)
  có hàng "Rời nhóm" (icon LogOut, `--color-danger`, cao 44), tách khỏi danh sách bằng đường kẻ `--color-line`;
  sheet nhấn giữ hàng trong cột điều hướng trên điện thoại có "Rời nhóm" cuối, chữ danger.
  Hộp xác nhận (ConfirmDialog; điện thoại hiện dạng sheet đáy như mọi ConfirmDialog): tiêu đề "Rời “{tên nhóm}”?",
  thân "Bạn sẽ không còn thấy tin nhắn và tệp của nhóm này. Một thành viên trong nhóm có thể thêm bạn lại.";
  khi bạn là người cuối cùng: thêm dòng `--color-danger` "Bạn là người cuối cùng trong nhóm. Rời đi thì không còn
  ai đọc được nhóm này." Nút Huỷ · "Rời nhóm" (danger, loading "Đang rời…"). Không lạc quan: chờ máy chủ xong mới
  bỏ nhóm khỏi cột điều hướng, đóng panel, về màn Chat (cuộc trò chuyện trước đó hoặc trạng thái trống) và toast
  "Đã rời “{tên nhóm}”" (không Hoàn tác: quay lại cần người trong nhóm thêm). Lỗi: hộp vẫn mở, dòng lỗi
  `role="alert"` "Không rời được nhóm. Thử lại." dưới thân, nút giữ nguyên. Đang trong cuộc gọi của nhóm: rời nhóm
  cũng ngắt cuộc gọi như khi bị gỡ khỏi nhóm (§Gọi).

- **Luồng (MessageStream):** nền `--color-base`, cũ trên mới dưới, tin xếp **hai bên**. Tin của người khác
  bên trái: Avatar 32 + tên hiển thị 600 + giờ `text-xs` mờ tabular phía trên bong bóng. **Tin của bạn bên
  phải**: không avatar, không tên (chữ ẩn "Bạn" cho trình đọc màn hình), chỉ giờ ở trên, căn phải. Bong bóng:
  `text-md` (điện thoại `text-base`), padding 7 × 12, radius 16, rộng tối đa min(70ch, 85%) cột tin, co theo nội
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
  với bạn — cột phải này chỉ có từ 768 trở lên; 11px mờ, tabular) khi hover hoặc focus. Ngắt nhóm khi:
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
  bên trái, viền 2px màu nền) · **"N trả lời"** 14px/600 `--color-accent` · "Lần cuối 09:20" mờ (giờ theo §8; hôm
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
  bản). Trích dẫn và đính kèm giữ nguyên. Lưu xong cuối bong bóng có nhãn **"đã sửa"** 12px mờ (cả người khác
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
- **Hiển thị định dạng trong tin:** đậm 600; code inline `--font-mono` 14px trên `--color-sunk`, radius 4, padding
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
  (cắt …) + "PDF · 1,2 MB" 12px mờ tabular; hover hiện IconButton Tải xuống; bấm thẻ mở **Trình xem tệp**.
- **Trình xem tệp (FileViewer)** (mockup `design/mockups/file-viewer.html`), một trình xem cho mọi tệp mở từ tin, thread, tab Tệp và Tài liệu.
  - **Hộp:** ≥ 768 là hộp thoại phủ (`--z-modal`, cách mép 32, radius 12, `--shadow-overlay`, nền `--color-scrim`). Dưới 768 toàn màn.
  - **Thanh trên** (`--color-base`, 56): ô loại 36 theo `file-kind`, tên cắt giữa (giữ đuôi), "Loại · dung lượng · PersonChip 20 + gửi lúc/tải lên + giờ". Phải: Mở trong Tài liệu (ghost; chỉ khi tệp nằm trong thư mục người xem mở được, và không hiện khi mở từ Tài liệu), Tải xuống, Đóng. Dưới 1024 các nút chỉ còn icon.
  - **Sân khấu** (`--color-sunk`): nút trước/sau tròn 44 ở mép, "n / N" dưới, chỉ trong cùng tin hoặc cùng danh sách, không quay vòng.
  - **Thân theo loại:**
    - Ảnh: vừa khung / 100% + kéo.
    - PDF và Office (đổi trên máy chủ): trang dọc rộng tối đa 816 + viên "Trang [n] / N · − % +". Office thêm dòng mờ "Bản xem trước từ tệp Word, có thể khác bản gốc đôi chút."
    - Video, âm thanh: `controls` gốc.
    - Bảng tính (xlsx, xls, ods; người dùng chốt 2026-10-11): bảng như CSV, mỗi sheet một tab, 500 dòng đầu, công thức hiện giá trị — không đổi sang PDF.
    - Văn bản, mã, CSV: mono 13; mã có số dòng; CSV là bảng tiêu đề dính, hàng 36. Tối đa 500 dòng / 1 MB; mã chưa tô màu cú pháp, dải đáy "Đang xem N dòng đầu trên M dòng." + Tải xuống để xem đủ.
    - Loại khác: icon 64 + "Không xem trước được loại tệp này" + Tải xuống (primary).
  - **Trạng thái:**
    - Skeleton (sau 300ms).
    - "Đang chuẩn bị bản xem trước…" (Office lần đầu, thanh tiến độ không giá trị, Tải xuống thay vì chờ, aria-live khi xong).
    - Lỗi chuyển đổi và quá lớn (nói lý do + Tải xuống).
    - 403 "Bạn không có quyền xem tệp này": không tên, loại, dung lượng, người gửi; không có thao tác.
    - 404 "Tệp này đã bị xoá": tên mờ, không có thao tác.
  - **Điện thoại:** Đóng trái, ⋯ phải (sheet: tên đầy đủ, người gửi, Tải xuống, Mở trong Tài liệu). Thanh đáy ‹ n / N ›, nút tắt ở hai đầu. Phóng bằng chụm hai ngón.
  - **Bàn phím:** Esc (sheet trước), ←/→ (trừ khi đang ở media/ô số trang), + − 0. Focus trap, trang sau `inert`, mở thì focus vào Đóng, đóng thì trả focus về chỗ đã bấm.
  - **Giới hạn (người dùng chốt 2026-10-11):** đổi Office tối đa 50 MB nguồn, 45 giây, PDF ≤ 80 MB; trong lúc máy chủ đổi, request chờ và thân hiện "Đang chuẩn bị bản xem trước…". 403 giữ ẩn tên tệp, chưa có "Xin quyền xem".

### Chat trên điện thoại (mockup `design/mockups/mobile-shell.html` §1, §1b, §7, §9; hàng danh sách và luồng theo `chat-mobile-home.html`, `chat-mobile.html`)

Bản 390px của Chat; hành vi theo các mục "Chat: …" ở trên, chỉ khác cách chạm. Vỏ (ô tìm, thanh đáy phân hệ, nút +)
theo §5 "Điều hướng di động". Tiêu đề "Chat" + nút **Thread** + ⋯ (Cài đặt Chat · Mục mới). Dưới tiêu đề là hàng chip lọc
cuộn ngang (cao 36): **Tất cả · Mục · Chưa đọc · Được nhắc · Trực tiếp · Nhóm** (thay bốn tab Trang chủ · Trực tiếp · Mục ·
Thêm của bản trước). Chip đang chọn nhớ trên thiết bị; Cài đặt Chat chọn được màn mở đầu (Tất cả hoặc Mục). Trên máy tính các
màn Được nhắc và Đã gắn sao giữ nguyên route của chúng.
- **Tất cả** (trước là Trang chủ): mọi cuộc trò chuyện (nhóm và trực tiếp) xếp theo hoạt động mới nhất. Nút **Thread** nằm
  ở hàng tiêu đề (pill số thread đang theo dõi có tin mới; mở danh sách Thread); lọc chưa đọc là chip **Chưa đọc** (bật mà
  hết thì "Bạn đã đọc hết"). Hàng: avatar
  44 (nhóm vuông bo 12, người tròn + chấm đang hoạt động) · tên 16px + giờ tương đối (1 phút, 09:02, Hôm qua) · dòng
  "Người gửi: đoạn đầu" của tin mới nhất ở luồng chính ("Bạn:" cho tin của mình; 2 dòng nếu không có dòng thread) ·
  nếu trả lời thread mới hơn tin luồng chính: dòng thụt có nét cong `--color-line` 12×12 bo 8, "Người trả lời: đoạn
  đầu" tối đa 2 dòng, và huy hiệu thread 22 (`--color-accent-wash`, icon accent, viền 2px `--color-base`) góc dưới
  phải avatar. Tên được nhắc trong đoạn xem trước là chip `--color-accent-wash` (@ màu accent). Tin có tệp: "📎 tên
  tệp"; thẻ: "Bình chọn: câu hỏi", "Công việc: tiêu đề". Chưa đọc: tên và giờ 700, đoạn chưa đọc màu `--color-ink`
  600, pill số (hoặc "@" khi được nhắc trong cuộc đã tắt thông báo) cuối dòng. Tắt thông báo: hàng mờ 66%. Chạm hàng mở
  cuộc trò chuyện; chạm dòng thread mở thẳng thread đó. Nhấn giữ hàng: sheet Chuyển tới mục / Mục mới… / Bỏ khỏi mục /
  Gắn sao / Tắt thông báo / Đánh dấu đã đọc. Kéo xuống để tải lại.
- **Trực tiếp / Nhóm / Chưa đọc / Được nhắc:** như Tất cả nhưng chỉ phần đó; Được nhắc liệt kê các lần bạn được nhắc tên.
- **Mục** (section kiểu Google Chat, giữ nguyên cơ chế): cùng thứ tự cột điều hướng máy tính (Đã gắn sao, mục của bạn, Tin nhắn trực tiếp, Nhóm).
  Tiêu đề mục: nút gập/mở (chevron + tên 16/600, cao 44; gập thì pill tổng chưa đọc) + nút chỉ chưa đọc của riêng mục
  (pill 52×36). Hàng 56, avatar 36 (nhóm vuông bo 10), tên 16px (700 khi chưa đọc), phải: pill số, "@", icon tắt thông
  báo. Nhấn giữ hàng như Tất cả; nhấn giữ tiêu đề mục của bạn: Đổi tên, Lên/Xuống, Xoá mục. "Mục mới" (accent) cuối
  danh sách. Gập/mở dùng chung trạng thái với cột điều hướng máy tính.
- **Đã gắn sao, Thread đang theo dõi:** qua "Đi nhanh" (nhấn giữ ô Chat, §5) và mục Đã gắn sao; thông báo của Chat ở Cài
  đặt Chat (§6 "Cài đặt").
- **Đưa vào mục:** nhấn giữ hàng, vuốt trái → Thêm, hoặc dòng Mục trong Cài đặt nhóm/trò chuyện — cùng một sheet chọn mục
  (radio các mục + "Mục mới…").
- **Tìm trong chat:** chạm viên thuốc thì nó đứng yên và thành ô tìm (ô chữ cái xoay thành ←, viền accent 2px, danh sách mờ đi,
  panel gợi ý dâng lên); chưa gõ: Tìm gần đây (xoá từng mục hoặc Xoá tất cả + Hoàn tác) + Trò chuyện gần đây; gõ: gợi ý Người,
  Nhóm và trò chuyện, Tin nhắn + "Xem tất cả kết quả"; Enter: kết quả tin nhắn với chip lọc cuộn ngang (cao 36) Người gửi ·
  Trong · Ngày · Có tệp · Có liên kết · Nhắc đến tôi, chip mở danh sách thành bottom sheet. Chỉ trả tin trong cuộc trò
  chuyện còn quyền đọc. Chi tiết: "Gợi ý trong ô tìm" (§6, `search-gchat.html` §5, duyệt 2026-10-18).
- **Luồng:** thanh trên có quay lại, icon nhóm 28, tên + "N thành viên" (chạm tên mở Cài đặt nhóm/trò chuyện, §6), nút Gọi và Thread (badge); bong bóng tối đa
  88%, chữ 16px; hàng tóm tắt thread cao 36. Không hover: chạm tin hiện giờ trong cụm. Tin của bạn không chừa
  cột giờ bên phải: bong bóng cách mép phải bằng khoảng avatar cách mép trái (14); chạm dòng gom cụm thì giờ hiện
  ngay trên bong bóng, căn phải, chạm lần nữa thì ẩn.
- **Thread và danh sách Thread:** màn toàn màn trượt lên (280ms), quay lại giữ vị trí cuộn của luồng; nút Theo dõi /
  Đang theo dõi (pill 36) trên thanh trên; danh sách Thread có chip lọc cuộn ngang (cao 36).
- **Nhấn giữ tin (400ms):** sheet đáy có hàng 6 cảm xúc nhanh (48) + thao tác (hàng 52): Trả lời trong thread, Theo dõi /
  Bỏ theo dõi thread, Sao chép, Trích dẫn, Ghim, Sửa tin (của bạn) hoặc Đánh dấu chưa đọc (người khác), divider, Xoá tin.
  Sửa trên điện thoại dùng ô soạn ở chế độ sửa (thanh "Đang sửa tin" + Huỷ).
- **Ô soạn:** bo 22, nút + (sheet: hàng đính kèm Máy ảnh / Ảnh / Tệp / Tài liệu, rồi Bình chọn / Công việc / Tài liệu mới /
  Đề nghị phê duyệt), chữ 16px (tránh iOS phóng to), nút định dạng, nút gửi tròn 40 accent khi có nội dung; dính đáy và
  đi lên cùng bàn phím ảo.
  - **Bàn phím đóng:** ô bo 22 như trên, cách đáy `max(12px, safe-area-inset-bottom)` để không đè vạch home.
  - **Bàn phím mở** (vùng nhìn thấy thấp hơn cửa sổ quá 120 px, chỉ dưới 768; mockup `design/mockups/avatar-edit.html` §6,
    duyệt 2026-10-16): như Google Chat, ô soạn thành **thanh tràn ngang sát mép bàn phím** — không bo, không margin, nền
    `--color-base`, viền trên 1px `--color-line`, padding 6 × 10; ô nhập dạng viên thuốc cao 38 (`--color-raised`, viền
    trong 1px `--color-line`), nút + và Gửi tròn hai bên. Không hiệu ứng chuyển: thanh đi cùng bàn phím. Thanh "‹ › Xong"
    phía trên bàn phím là của iOS cho mọi ô soạn trên web, app không tắt được.

### Thông báo đẩy, cài app, mất kết nối (mockup `design/mockups/push.html`, mobile trước)

- **Thông báo đẩy (OneSignal):** năm loại, mỗi loại tắt được: tin nhắn trực tiếp, được nhắc tên, trả lời trong thread
  đang theo dõi, công việc giao cho bạn, đề nghị cần bạn duyệt (chỉ workspace tổ chức). Dòng nguồn "Nexus · tên
  nhóm/khu vực", tiêu đề là người gửi hoặc câu "X cần bạn duyệt", thân là đoạn đầu văn bản thuần (2 dòng) khi bật
  "Xem trước nội dung", nếu không thì "Tin nhắn mới trong …". Không ID. Gộp tin cùng cuộc trò chuyện trong 30s thành
  "N tin mới". Không gửi khi đang mở đúng cuộc trò chuyện, hoặc khi người nhận không còn quyền đọc. Trong giờ yên lặng push bị chặn, trừ khi người nhận được nhắc tên (kể cả @Tất cả).
  Bấm: mở/focus app tới đúng cuộc trò chuyện hoặc thread, tin được nhắc có viền accent loé 2,4s.
- **Hỏi bật thông báo:** thẻ trong app (Card `--color-raised`, viền trong `--color-line`, radius 14, ô icon chuông
  40 nền accent wash, "Nhận thông báo khi có tin mới?" + một câu lợi ích + Để sau / **Bật thông báo**) hiện sau tin đầu
  tiên bạn gửi hoặc khi mở Thông báo; chỉ khi bấm Bật mới gọi hộp quyền của trình duyệt. "Để sau" ẩn 7 ngày, ba lần thì
  thôi. Cho phép → toast success; chặn → toast + hướng dẫn trong Cài đặt. iPhone chưa thêm vào màn hình chính: thẻ đổi
  thành lời mời thêm vào màn hình chính.
- **Cài đặt → Thông báo (máy tính):** là bản máy tính của các tuỳ chọn thông báo theo phân hệ ở §6 "Cài đặt" (cùng một nơi lưu, hai lối vào); điện thoại dùng Tôi → Thông báo. Nhóm "Thiết bị này" (Switch thông báo đẩy, khoá + banner `danger-wash` có 3 bước mở lại khi
  trình duyệt chặn; banner `info-wash` cho iPhone chưa cài), "Báo cho tôi khi" (5 Switch theo loại, icon 32 radius 9 nền
  sunk), "Nội dung và giờ" (Xem trước nội dung, Giờ yên lặng + hai select từ/đến; giờ yên lặng chặn push trừ khi bạn được nhắc tên, kể cả @Tất cả). Lưu ngay, không hộp xác nhận; tắt
  thiết bị thì các mục dưới mờ 50% nhưng giữ giá trị. Hàng ≥ 56, Switch 44 × 26.
- **Cài app:** Android — thanh nổi trên thanh đáy (icon app 44, "Cài Nexus vào màn hình chính" + một dòng mờ, Để sau /
  Cài đặt) từ lần mở thứ hai, chỉ ở danh sách, không trong cuộc trò chuyện; iPhone — sheet một lần với 3 bước có số
  (Chia sẻ → Thêm vào MH chính → Thêm). Tên "Nexus", icon chữ N trên nền accent.
- **Mất kết nối:** thanh `--color-warning-wash` dưới thanh trên (icon wifi gạch, "Mất kết nối. Đang thử kết nối lại…",
  Thử lại) sau 2s mất WebSocket hoặc offline; có lại → "Đã kết nối lại" success 2s rồi ẩn. Dữ liệu đang hiện giữ nguyên,
  ô soạn khoá; không đọc offline. **Bản mới:** thanh nổi "Có bản mới của Nexus" + Để sau / Tải lại; đang soạn dở thì
  "Tải lại sau khi gửi".
- **Đang cập nhật (mở app từ dữ liệu lần trước):** khi app mở bằng bản lưu trên máy, Trang chủ và danh sách hiện ngay
  từ bản đó, và một dải nhỏ báo rằng đang hiện dữ liệu cũ trong lúc tải bản mới. Vị trí: ngay dưới thanh trên, trong
  cùng chỗ với thanh mất kết nối (chiếm chỗ của nó, không chồng lên). Dải mảnh, cao 28 (không phải thanh 44 của mất
  kết nối), nền `--color-sunk`, chữ `text-xs` 500 `--color-ink-muted` "Đang cập nhật…" đứng sau icon `RefreshCw` 14
  `--color-ink-muted` xoay chậm (1,2s tuyến tính; bỏ xoay khi giảm chuyển động). Không nút, không toast, không màu
  cảnh báo: đây là chờ bình thường, không phải lỗi. Vùng `role="status"` `aria-live="polite"` luôn có mặt, chỉ nội dung
  đổi. Biến mất ngay khi bản mới về, hoặc khi tải bản mới thất bại (lúc đó thanh
  mất kết nối hoặc lỗi từng khối nói thay); khi thanh mất kết nối đang hiện thì dải này không hiện. Không có bản lưu
  (lần đầu, sau đăng xuất, đổi workspace) thì không bao giờ hiện: màn hình dùng Skeleton như thường.

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
### Cuộc gọi video và thoại (mockup `design/mockups/video-call.html`, duyệt 2026-10-13)

Cuộc gọi luôn thuộc một cuộc trò chuyện. Tin nhắn trực tiếp: gọi thì đổ chuông. Nhóm (tối đa 20 người): bắt đầu thì
ai trong nhóm cũng tham gia được, không đổ chuông cả nhóm. Đợt đầu: chia sẻ màn hình, chat trong cuộc gọi, giơ tay,
thả cảm xúc; không ghi hình, không quyền chủ phòng.
- **Điểm bắt đầu:** thanh đầu tin nhắn trực tiếp có IconButton **Gọi thoại** và **Gọi video** (trước Thông tin). Nhóm có
  nút pill 36 "Bắt đầu cuộc gọi" (`--color-accent-wash`, icon video accent); khi đang có cuộc gọi đổi thành **Tham gia**
  (`--color-success`, chữ `--color-on-accent`) + pill `--color-success-wash` "Đang gọi · mm:ss" có chấm success nhấp nháy
  (giảm chuyển động: tĩnh). Không có quyền gửi tin thì không có nút.
- **Thẻ cuộc gọi trong luồng** (tin hệ thống của người bắt đầu, luồng chính, không có thread): Card `--color-raised`,
  viền trong `--color-line`, radius 14, rộng tối đa 420; ô icon 44 radius 12 — đang diễn ra `--color-success-wash`/success,
  kết thúc `--color-accent-wash`/accent, nhỡ `--color-danger-wash`/danger. Đang diễn ra: avatar 20 tối đa 3 + "và N
  người" + đồng hồ + Button sm **Tham gia** (success); bạn đang ở trong: "Mở lại cuộc gọi". Kết thúc: số người · thời
  lượng · giờ bắt đầu–kết thúc. Nhỡ: "Phạm Hải Yến gọi lúc 08:33" + Button sm primary "Gọi lại". Xem trước/Trang chủ:
  "📹 Cuộc gọi video · 32 phút", "📹 Cuộc gọi nhỡ".
- **Kiểm tra trước khi vào** (nhóm; DM gọi đi và trả lời vào thẳng): xem trước 16:10 nền `--color-stage` radius 18, gương
  lật; camera tắt → avatar 88 + "Camera đang tắt"; đo micro 5 vạch `--color-stage-accent`; nút tròn micro/camera trên
  xem trước. Bên phải: "Sẵn sàng tham gia?", ai đang trong cuộc gọi, Select Micro / Camera / Loa (tên thiết bị do hệ
  điều hành đặt, không ID; "Thử loa"), banner `danger-wash` khi trình duyệt chặn (cách mở lại; vẫn vào để nghe được),
  Button lg primary "Tham gia ngay" + Huỷ. Đủ 20 người: "Cuộc gọi đã đầy (20/20)" disabled. Nhớ lựa chọn theo người trên
  thiết bị.
- **Cuộc gọi đến:** máy tính — thẻ nổi `--color-overlay` radius 16 rộng 360 góc trên phải (cách 16) ở mọi màn: avatar 56
  có vòng success lan ra, tên, "đang gọi video cho bạn…", ba nút Từ chối (danger) · Thoại · Video (success); chuông tới
  45 giây; Esc tắt chuông. Điện thoại app mở — màn đổ chuông toàn màn trên `--color-stage`: avatar 112, tên 26/700,
  ba nút tròn 68 (Từ chối `--color-stage-danger`, Thoại, Video success). App đóng — thông báo đẩy loại "Cuộc gọi đến"
  (Trả lời mở thẳng cuộc gọi; hết 45 giây thành "Cuộc gọi nhỡ"). Trả lời ở một thiết bị thì thiết bị khác ngừng đổ
  chuông. Đang trong cuộc gọi khác: "Kết thúc và trả lời".
- **Stage (máy tính):** nền `--color-stage`; thanh trên 52 (tên cuộc trò chuyện display 700, đồng hồ, chip số người,
  chip "Chỉ thành viên nhóm", Bố cục); lưới ô 16:9 radius 14, khe 10: 1 người khác → người đó lớn + ô bạn nhỏ góc;
  2–4 → 2 cột; 5–9 → 3 cột; 10–20 → 4 cột, phân trang. Ô: video hoặc avatar 88 trên `--color-stage-tile`; nhãn tên tối
  (đen 62%) góc trái dưới; micro góc phải trên (sóng accent khi nói, gạch đỏ khi tắt); chip "Mạng yếu"; huy hiệu vàng
  "✋ thứ tự" khi giơ tay; người nói viền trong 3px `--color-stage-accent`. Thanh dưới 84: nút tròn 52
  `--color-stage-raised` — Micro (Ctrl+D) · Camera (Ctrl+E) · Trình bày · Giơ tay (Ctrl+Alt+H) · Cảm xúc · **Rời**
  72×52 `--color-stage-danger`; phải: Chat · Người tham gia (badge `--color-stage-accent`) · Thêm (vuông 44 radius 12);
  trái: giờ · tên. Tắt micro/camera: nền đỏ nhạt; đang bật trình bày/giơ tay: `--color-stage-accent`. Space giữ để nói.
- **Panel trong cuộc gọi** (rộng 340, `--color-raised`, radius 16, nằm trên stage): tab Chat / Người tham gia. Chat là
  luồng chính của chính cuộc trò chuyện (rút gọn, ô soạn cuối). Người tham gia: Đang giơ tay (thứ tự) · Trong cuộc gọi ·
  Trong nhóm chưa vào (+ "Mời" gửi thông báo).
- **Cảm xúc:** 6 biểu tượng trong popover viên thuốc; bay lên 3 giây từ góc trái dưới kèm tên (giảm chuyển động: tĩnh
  3 giây); không lưu. Giơ tay: tiếng nhẹ cho người khác; tự hạ 2 giây sau khi người đó nói xong.
- **Trình bày:** bản trình bày lớn + dải 200px bên phải (tối đa 4 ô + "N người khác"); nhãn "X đang trình bày"; người
  trình bày thấy viên thuốc `--color-stage-accent` "Bạn đang trình bày …" + "Dừng trình bày" ở giữa thanh trên. Một
  người trình bày mỗi lúc (người sau được hỏi "Thay người đang trình bày?"). Điện thoại không trình bày (ẩn nút).
- **Điện thoại:** toàn màn trên `--color-stage`; thanh trên: Thu nhỏ (chevron xuống) · tên + đồng hồ · Loa ngoài. 1–1:
  người kia toàn màn radius 20 + ô của bạn 104×148 kéo được vào 4 góc. Nhóm: lưới 2 cột ô 3:4, vuốt ngang qua trang
  (chấm trang). Dưới: nút tròn 56 Micro · Camera · Đổi camera · Thêm · Rời. Thêm = sheet `--color-stage-tile`: hàng 6
  cảm xúc 48, Giơ tay, Chat trong cuộc gọi, Người tham gia, Micro/camera/loa. Chạm màn ẩn/hiện điều khiển (tự ẩn 4 giây,
  trừ khi sheet mở hoặc dùng trình đọc màn hình). Thu nhỏ/vuốt xuống → ô nổi 132 rộng (người đang nói + đồng hồ + Rời)
  trên mọi màn, kéo về hai góc; máy tính dùng Picture-in-Picture khi rời tab. iPhone PWA: "Chạm để bật âm thanh" khi cần.
- **Trạng thái:** đang kết nối lại (lớp phủ, giữ chỗ 30 giây) → không kết nối được (gợi ý mạng công ty) + Thử lại; mạng
  yếu → hạ chất lượng trước, chip trên thanh trên; kết thúc → tổng kết (thời lượng, số người, số tin) + Về cuộc trò
  chuyện / Vào lại; đang ở cuộc gọi khác → "Rời và vào"; bị gỡ khỏi nhóm → ngắt ngay + banner danger.

### Tài liệu: lưới, danh sách, panel xem trước (mockup `design/mockups/drive-gallery.html`, duyệt 2026-10-16)

Tài liệu mở mặc định ở **Lưới**; **Danh sách** là bảng mảnh hiện có. Lựa chọn nhớ theo người dùng, riêng cho Tài liệu và
cho tab Tệp của nhóm.
- **Thanh công cụ** (một hàng): ô tìm viên thuốc 240 "Tìm trong <thư mục>" · chip loại (radio, kiểu `fchip`): Tất cả ·
  Tài liệu · Bảng tính · Ảnh & video · PDF · khoảng trống · sắp xếp (ghost sm, "Sửa gần nhất" / Tên / Dung lượng) ·
  nút đổi Lưới/Danh sách (seg 2 nút icon, `aria-pressed`) · **Tải lên** (primary sm; menu có Thư mục mới).
- **Lưới:** nhãn nhóm "Thư mục" rồi "Tệp" (`k` 11px uppercase mờ). **Ô thư mục:** cao 48, `--color-raised`, bo 12, icon
  thư mục mờ + tên 600 + số tệp mờ bên phải; là link thật vào thư mục. **Thẻ tệp:** `--color-raised`, bo 12, padding
  6 6 10; thumbnail 16:10 bo 8 (ảnh do máy chủ sinh: ảnh, trang 1 PDF/Office, khung video có nút phát + thời lượng);
  hàng tên: ô loại 20 theo `file-kind` + tên 600 cắt cuối; dòng "Avatar 20 + tên ngắn · thời gian" 12px mờ. Hover:
  `--color-overlay` + `--shadow-overlay`, hiện ⋯ (28, góc trên phải). Chọn: nền `--color-accent-wash`, viền trong 2px
  accent. Cột: lưới auto-fill tối thiểu 200, gap 12 (ô thư mục gap 10).
- **Thumbnail:** khung 16:10 luôn giữ chỗ; đang tải = Skeleton chạy sáng (dừng khi giảm chuyển động); máy chủ không
  sinh được hay đang bận = icon loại 34 `--color-ink-subtle` trên `--color-sunk`, không báo lỗi.
- **Danh sách:** bảng hiện có + thumbnail 36 × 24 bo 4 trước tên.
- **Panel chi tiết** (rộng 320, khi có tệp được chọn): tiêu đề (ô loại + tên + Đóng) · xem trước 4:3 bo 10 (ảnh 640) ·
  **Mở** (primary, giãn) · Tải xuống · Chia sẻ · bảng thông tin (Loại, Dung lượng, Chủ sở hữu PersonChip, Sửa lần cuối,
  Vị trí, Người có quyền cụm Avatar + "+ N") · Hoạt động (Avatar 20 + "**Tên** việc · thời gian"). Không chọn gì: panel ẩn.
- **Bàn phím:** mũi tên đi trong lưới theo số cột; **Space/Enter** mở Trình xem tệp với các tệp của thư mục (đã lọc),
  ←/→ trong đó; Shift+F10 mở ⋯; chuột phải mở cùng menu.
- **Dưới 1024 (điện thoại và máy tính bảng; mockup `design/mockups/mobile-shell.html` §10, duyệt 2026-10-19):** vỏ như
  mọi phân hệ (§5). Không ngăn kéo cây thư mục: dưới tiêu đề là chip lối vào **Gần đây · Thư mục · Gắn sao · Chia sẻ với
  tôi · Bản nháp** (một mục "chia sẻ" duy nhất: những gì người khác chia sẻ với bạn; Bản nháp là văn bản nháp của bạn).
  "Thư mục" mở thư mục gốc của workspace (cá nhân: "Tài liệu của tôi"; tổ chức: tên tổ chức, rồi thư mục phòng ban theo
  quyền). Máy tính bảng: lưới 3 cột, panel chi tiết thành sheet bên phải (rộng 340, cách mép 10, bo 16) phủ lên với scrim.
  - *Gần đây:* "Truy cập nhanh" — tối đa 6 thẻ 148 cuộn ngang (thumbnail 16:10 bo 9 + tên 13/600 + một dòng lý do: "Bạn
    sửa · Hôm qua", "Thu Lan chia sẻ"; lý do chỉ hiện khi người xem còn quyền thấy nguồn của nó) — rồi danh sách nhóm theo
    thời gian (Hôm nay · Tuần này · Tháng này · Cũ hơn).
  - *Danh sách là mặc định trên điện thoại:* hàng 60, ô loại 40 radius 10 tô theo `file-kind` (văn bản `--color-info`,
    bảng tính `--color-success`, PDF `--color-danger`, ảnh `--color-warning`, thư mục `--color-sunk`), tên 15/500 (sao vàng
    nếu đã gắn sao) + "Loại · ai sửa · khi nào" 13px mờ, ⋯ 44. Nút đổi Lưới/Danh sách ở hàng tiêu đề, nhớ theo người. Lưới 2
    cột chỉ dùng thumbnail thật; tệp không có thumbnail hiện như ô danh sách.
  - *Trong thư mục:* thanh trên ← (lên thư mục cha) · tên thư mục · Tìm (ô tìm §6 với phạm vi = thư mục này) · ⋯; dưới là
    đường dẫn cuộn ngang 13px ("NovaPay › Phòng Kế toán › Tài chính", đoạn cuối 600, chạm một đoạn để nhảy), rồi một hàng
    sắp xếp ("Sửa gần nhất ⌄") + đổi kiểu xem; thư mục trước tệp.
  - *Menu tệp* (⋯ hoặc nhấn giữ, bottom sheet): đầu sheet là hàng tệp (ô loại + tên + "Loại · dung lượng · ai sửa"); 4 lối
    tắt ô 16 radius: Chia sẻ · Sao chép liên kết · Gắn sao · Tải xuống; rồi hàng 52: Gửi vào trò chuyện… · Chuyển tới thư
    mục… · Tạo bản sao · Thông tin và hoạt động (người có quyền, lịch sử) · Đổi tên · Chuyển vào thùng rác (danger). Chỉ
    hiện việc người xem được phép. Tạo bản sao trong cùng thư mục thành "Bản sao của <tên>".
  - *Chọn nhiều:* nhấn giữ một hàng vào chế độ chọn; ô chọn 22 radius 7 thay ô loại; chạm hàng khác (hoặc kéo dọc cột ô
    chọn) để chọn tiếp; thanh trên "Đã chọn N" + ✕ + Chọn tất cả; thanh thao tác nền `--color-ink` cao 64 radius 22 thay
    thanh đáy: Chia sẻ · Chuyển · Tải xuống · Gắn sao · Xoá — chỉ những việc mọi mục đã chọn đều cho phép. Esc / ✕ / vuốt mép
    trái thoát chọn.
  - *Nút + (sheet "Thêm vào <thư mục>"):* Tải tệp lên · Chụp ảnh · Thư mục mới · Văn bản mới. Tải lên chạy nền: thẻ tiến độ
    (`--color-overlay`, radius 16, "Đang tải lên N tệp" + %, thanh 4px accent, tên tệp + thời gian còn lại) nổi trên thanh
    đáy, gập thành vòng % khi cuộn, theo sang màn khác và còn sau khi tải lại app; lỗi từng tệp có "Thử lại"; xong thì toast
    "Đã tải lên N tệp vào <thư mục> · Xem".
  - *Vuốt hàng (§6 "Vuốt"):* trái lộ Chia sẻ · Gắn sao · Thêm; không có vuốt xoá.
- **Tab Tệp của nhóm:** cùng thẻ và lưới; chip Tất cả · Ảnh & video · Tài liệu · Khác; nhóm theo thời gian (Hôm nay · Tuần
  này · Tháng này · Cũ hơn, tiêu đề dính khi cuộn trên điện thoại); nhóm toàn ảnh/video thành lưới vuông sát nhau (ô tối
  thiểu 112, gap 4, bo 6; điện thoại 3 cột) như album; "Mở trong Tài liệu" ghost ở cuối thanh công cụ; menu ⋯ có thêm
  "Tới tin nhắn". Trình xem tệp nhận danh sách đã lọc. Rỗng theo chip: "Chưa có ảnh hay video nào trong nhóm." / "Chưa
  có tài liệu nào trong nhóm."; rỗng hẳn giữ câu hiện có + "Đính kèm trong Trò chuyện".

### Lịch (mockup `design/mockups/calendar.html`, duyệt 2026-10-17)

Lịch là mục chính sau Tin nhắn (§5), cả ở workspace cá nhân (chỉ lịch cá nhân). Hành động chính là **Tạo** ở cột trái;
thanh trên không có nút primary.

**Ngoại lệ có chủ đích** (người dùng yêu cầu "giống Google Calendar", 2026-10-17); chỉ áp trong module Lịch:
- Khối sự kiện **tô đặc** màu lịch, không phải wash + vạch trái như phần còn lại của hệ.
- Vạch giờ hiện tại dùng `--color-danger` (đỏ), không phải accent; chấm đầu vạch 12px.
- Nút dạng pill (radius full) cho Hôm nay, nút trên thanh, nút trong popover và trình soạn; riêng **Tạo** radius 16.
- Bộ chọn kiểu xem là nút dropdown (không phải seg).

- **Thanh trên (trải ngang, kiểu Google):** ☰ 40 · logo 38 + chữ "Lịch" · Hôm nay (`gbtn` 36) · ‹ › · tiêu đề tháng
  (body 22/400, "Tuần N" mờ) · khoảng trống · tìm (icon, mở ô viên thuốc "Tìm sự kiện") · nút chọn kiểu xem
  (**Ngày · Tuần · Tháng · Lịch biểu**, mỗi dòng ghi phím tắt, nhớ theo người trên từng máy) · Cài đặt lịch.
- **Cột lịch (256):** nút **Tạo** (radius 16, icon + nhãn; mở trình soạn, loại chọn trong đó) · lịch tháng nhỏ (hôm nay tròn
  accent, ngày đang xem nền wash) · **Tìm người** (Picker, thêm lịch của người/nhóm) · **Lịch của tôi** (lịch cá nhân, lịch
  workspace, lịch nhóm) · **Lịch khác** (đã được chia sẻ hoặc thêm). Mỗi lịch một ô chọn màu lịch (bỏ chọn = ẩn khỏi lưới)
  và menu ⋯ (đổi màu, Chia sẻ, Cài đặt).
- **Lưới giờ:** tuần bắt đầu **Thứ Hai**; 1 giờ = **48px**; 24h `09:00` tabular; cột giờ 56 có nhãn múi giờ (`GMT+7`);
  mở ra cuộn tới 1 giờ trước hiện tại. Số ngày 26/400 trong vòng 46, hôm nay vòng accent; không tô nền cột (kể cả
  hôm nay, cuối tuần). Hàng cả ngày dưới tiêu đề ngày, khối kéo qua nhiều ngày, quá 3 hàng thì "+ N" có chevron.
- **Màu lịch = 8 person hue** (§2). Lịch cá nhân mặc định hue của chủ lịch (lịch người khác hiện chồng bằng hue của họ);
  lịch workspace dùng accent; lịch nhóm chọn từ 8 hue. Mỗi sự kiện kế thừa màu lịch, đổi riêng được.
- **Khối sự kiện** (radius 6, đệm 3/7, viền 1px `--color-base` để tách khối kề nhau; tiêu đề 12/500 + giờ mờ):
  - Mặc định: nền màu lịch, chữ `--color-on-person` (giờ ở alpha 0.88).
  - **Đã qua:** nền `person-N-wash`, chữ `--color-ink`, giờ `--color-ink-muted`.
  - **Chưa trả lời:** nền `--color-raised`, viền trong 1px màu lịch, chữ màu lịch.
  - **Từ chối:** nền raised, chữ `--color-ink-muted` gạch ngang, viền `--color-line`.
  - **Bận** (lịch chỉ xem bận/rảnh): sọc chéo 135° wash, chữ "Bận" mờ, không mở chi tiết.
  - Đang chọn: `--shadow-overlay`. Khối tạm khi tạo/kéo: nền accent, `--color-on-accent`, tay nắm ở mép dưới.
  - Realtime từ người khác: vòng 3px `person-N-wash` + 2px `person-N` + nhãn (§7). ≤ 30 phút: một dòng "tiêu đề, 09:00".
    Trùng giờ chia cột theo giờ bắt đầu, khối sau chồng 50% sang phải. Không chỉ dựa vào màu: tên truy cập đọc đủ
    "Phỏng vấn kế toán viên, 15:00 đến 16:00, chưa trả lời".
- **Kéo thả:** kéo khối để dời (bước 15 phút, giữ Alt bước 5), kéo mép dưới đổi độ dài, kéo sang ngày khác được; thả là lưu
  + toast "Đã dời sang 10:00 · Hoàn tác". Không có quyền sửa: không kéo được, con trỏ mặc định.
- **Popover chi tiết** (rộng 420, Popover API, Esc đóng và trả focus về khối): công cụ Sửa · Xoá · Sao chép liên kết · ⋯
  (Nhân bản, In, Mở cuộc trò chuyện của sự kiện) · Đóng. Tiêu đề body 22/400, giờ, quy tắc lặp bằng chữ. Cuộc gọi video là
  **primary duy nhất**, chỉ từ 15 phút trước giờ bắt đầu tới lúc kết thúc (ngoài đó là "Sao chép liên kết cuộc gọi").
  Khách: tổng + tóm tắt trả lời, Avatar 32 + huy hiệu (✓ success, × danger, ? mờ). Dải "Bạn sẽ tham gia?" (Có · Không ·
  Có thể, pill 32) chỉ cho khách; người tổ chức thấy "Bạn là người tổ chức".
- **Tạo nhanh:** kéo một khoảng giờ (bước 15 phút) hoặc bấm ô trống → khối tạm + popover tiêu đề (autofocus) · Loại (Sự kiện ·
  Giờ làm việc riêng: người khác thấy Bận · Vắng mặt: cả ngày/nhiều ngày) · Khách · Lưu (mở trình soạn đầy đủ để thêm chi tiết). Lịch mặc định là lịch cá nhân;
  trong tab Lịch của nhóm là lịch nhóm. Esc, hay bấm ra ngoài khi tiêu đề trống, huỷ không hỏi. Lưu: toast "Đã tạo sự kiện ·
  Hoàn tác" (+ "Đã gửi lời mời cho 3 người").
- **Trình soạn đầy đủ:** ngày (lịch nhỏ) · giờ (Select gõ được, bước 15 phút, "đến" ghi kèm độ dài, đổi giờ bắt đầu giữ độ dài) ·
  múi giờ ẩn mặc định · Khách (Picker, PersonChip + trạng thái rảnh/bận; mời được cả nhóm chat; không bao giờ nhập email hay ID) ·
  cuộc gọi video Nexus · địa điểm · mô tả · đính kèm từ Tài liệu (khách chưa có quyền thì hỏi "Chia sẻ quyền xem cho N khách?") ·
  lời nhắc · màu · Hiển thị Bận/Rảnh · Quyền xem Mặc định/Riêng tư. Kiểm khi blur ("Giờ kết thúc phải sau giờ bắt đầu");
  đóng khi chưa lưu thì hỏi "Bỏ thay đổi?"; lưu có khách: "Đã lưu. Đã báo 4 khách."
- **Tìm giờ:** lưới mỗi cột một khách (Avatar 32 + tên + một dòng trạng thái: "Có thể tham gia" · "Trùng 1 sự kiện" · "Chỉ thấy
  bận/rảnh"; cột của bạn đứng đầu; × ẩn khỏi tìm giờ, không bỏ khách). Dải accent kéo được; chạm khối bận thì đầu cột thành pill
  warning "Trùng giờ"; ngoài giờ làm việc từng người = sọc xám. 3 chip giờ gợi ý sớm nhất mọi người cùng rảnh; không có thì
  "Không có giờ nào mọi người cùng rảnh hôm nay · Xem ngày mai". Bận/rảnh chỉ trả khoảng thời gian, không tiêu đề/địa điểm.
- **Lặp lại:** menu Không lặp lại · Hằng ngày · Hằng tuần vào thứ N · Hằng tháng vào thứ N thứ M · Hằng năm vào ngày D tháng M · Tuỳ chỉnh
  (mỗi N tuần, chọn ngày T2…CN, kết thúc Không bao giờ / Vào ngày / Sau N lần; câu mô tả bằng chữ tiếng Việt, không lộ RRULE).
  Sửa/kéo/xoá một lần của chuỗi mở **hộp phạm vi**: Chỉ sự kiện này · Sự kiện này và các sự kiện sau · Tất cả sự kiện (radio +
  Huỷ/OK); "Tất cả" giữ các lần đã sửa riêng và nói rõ trong toast. Không lặp thì xoá không hỏi, toast Hoàn tác 5s. Khách
  (không phải tổ chức) trả lời chọn phạm vi ngay trong dải trả lời.
- **Tháng:** ô cao theo lưới, số dòng tự tính theo chiều cao (tối thiểu 2), "+ N nữa" là button tên đầy đủ ("Thêm 2 sự kiện
  ngày 15 tháng 10"). Sự kiện cả ngày/nhiều ngày là thanh đặc; còn lại chấm màu + giờ + tiêu đề. Bấm số ngày → Ngày; bấm khoảng
  trống → tạo nhanh cả ngày; kéo dòng sang ô khác dời ngày (giữ giờ). Ngày tháng khác mờ.
- **Ngày và lịch biểu:** Ngày là lưới một cột. Lịch biểu: cột ngày 120 (số display 22 + "T4, TH 10"), hàng 36: chấm màu lịch ·
  giờ tabular · tiêu đề 600 · meta phải (icon video, cụm Avatar khách ≤ 3, "chưa trả lời"). Ngày trống không có hàng; vạch "Bây giờ"
  (`--color-danger`) giữa hai sự kiện hôm nay; tháng mới có tiêu đề display 19; cuộn cuối tải thêm 30 ngày.
- **Chia sẻ lịch (Select, 3 mức):** **Xem mọi chi tiết** (tiêu đề, địa điểm, khách; trừ sự kiện riêng tư) · **Sửa sự kiện**
  (tạo, sửa, xoá trên lịch này) · **Sửa và quản lý chia sẻ** (như trên + đổi ai được xem). Hàng cố định, không gỡ được:
  **Mọi người trong workspace · Thấy bận/rảnh** (không có mức riêng). Có hiệu lực ngay trên mọi thiết bị; người được chia sẻ nhận
  thông báo và lịch tự vào "Lịch khác". Lịch workspace: mọi thành viên xem, người quản lý workspace sửa. Lịch nhóm: thành viên
  xem và tạo; chỉ người tạo và người quản lý nhóm sửa sự kiện của người khác.
- **Nhắc giờ và thông báo:** toast trong app + thông báo đẩy ("Tham gia" khi có cuộc gọi · "Nhắc lại sau 5 phút") + hàng trong
  NotificationPanel. Loại: lời mời, đổi giờ/địa điểm, huỷ, khách trả lời (chỉ tổ chức), được chia sẻ, nhắc giờ; mỗi loại có mức
  trong Cài đặt thông báo. Từ chối thì không nhắc; mỗi người nhận một lần dù mở nhiều thiết bị.
- **Lịch của nhóm chat:** tab **Lịch** trong nhóm (lưới tuần/tháng của lịch nhóm). Lịch nhóm tự vào "Lịch của tôi" của thành
  viên; rời nhóm thì biến mất. Tạo sự kiện đăng **thẻ sự kiện** vào luồng (tin hệ thống của người tạo, có thread): đổi giờ thì
  thẻ cập nhật tại chỗ + tin nhỏ "Lê Quang Vinh đã dời sang 14:00"; huỷ thì thẻ mờ, nhãn "Đã huỷ"; đến giờ hiện "Đang diễn
  ra · Tham gia" và nút gọi ở đầu nhóm đổi thành "Tham gia".
- **Máy tính bảng (768–1023):** cột lịch thành ngăn kéo (nút khung bên phải tiêu đề); nút + nổi 56 bo 18 thay Tạo; vùng chạm
  ≥ 44; kéo thả bằng nhấn giữ 400ms; chạm sự kiện mở sheet phải rộng 360 (cách mép 10, bo 16), chạm ô trống mở sheet tạo nhanh.
- **Điện thoại (< 768):** vỏ như mọi phân hệ (§5), thanh trên viên thuốc "Tìm sự kiện", thanh đáy phân hệ, nút + là Sự kiện mới; vuốt ngang dải tuần đổi tuần (§6 "Vuốt"). Dải tuần 7 ô 36 (chấm dưới
  ngày có sự kiện, vuốt ngang đổi tuần); chạm "Tháng 10 ▾" gấp ra lịch tháng (280ms expo). Thẻ lịch biểu cao ≥ 56, nền đặc màu
  lịch (đã qua / chưa trả lời / từ chối theo cùng quy tắc khối), tiêu đề 16/600, giờ + địa điểm 13 mờ. ⋯ cạnh tiêu đề chọn kiểu
  xem: Lịch biểu · Ngày · 3 ngày · Tháng. Chi tiết là bottom sheet (§5); + nổi mở trình soạn toàn màn, Khách và Tìm giờ là màn con.
- **Trạng thái:** mọi truy vấn có Skeleton (cùng hình lưới), Rỗng ("Tuần này bạn chưa có sự kiện nào…" + Tạo sự kiện), Lỗi ("Không
  tải được lịch… Sự kiện đã lưu không bị mất" + Thử lại). Mất quyền giữa chừng: "Bạn không còn quyền xem lịch “…”" + lịch tự gỡ.
  Xung đột lưu: "Phạm Hải Yến vừa sửa sự kiện này" + Xem bản mới. Tìm rỗng: "Không có sự kiện nào khớp “…”".
- **Phím tắt** (giống Google, ghi trong bảng `?`): **D W M A** đổi kiểu xem · **T** hôm nay · **J / K** khoảng sau / trước ·
  **C** tạo · **E** sửa sự kiện chọn · **Delete** xoá · **/** tìm · Tab ↑ ↓ đi giữa sự kiện · Enter mở · Esc đóng · Ctrl S lưu.
- **Quyền hiện ra:** không có quyền thì ẩn hành động, không báo lỗi: không kéo được, popover không Sửa/Xoá, không mục Chia sẻ,
  không dải trả lời; chỉ thấy "Bận" khi không đọc được chi tiết. Quy tắc quyền (NGAC, trên OA của lịch): mục 14 của mockup, sẽ chuyển vào spec của Lịch trong `docs/specs/`.

### Gợi ý trong ô tìm: ô tìm nở ra, Tìm gần đây, gợi ý khi gõ, bộ lọc Chat (mockup `design/mockups/search-gchat.html`, duyệt 2026-10-18)

Thay bản trước (`search-suggestions.html`, duyệt 2026-10-17; giữ file làm lịch sử). Hướng: như Google Chat.
Ô tìm là **một bề mặt**: khi focus nó nở ra từ chính viên thuốc, nổi lên bằng bóng overlay và chứa gợi ý bên trong.
Áp cho ô tìm chính của **Chat, Tài liệu, Danh bạ, Tài sản, Phê duyệt** (bề mặt nở, Tìm gần đây xoá được, chuyển động);
gợi ý khi gõ và **bộ lọc** đợt này chỉ có ở Chat. Mockup §1–§6.

- **Ô tìm của Chat trên máy tính (mới):** đầu cột danh sách, trên "Trò chuyện mới": viên thuốc 256×40 (cột 280 trừ
  12 mỗi bên), cách đỉnh 12, nền `--color-sunk`, icon tìm 16 cách trái 14, chữ 16px, placeholder "Tìm trong chat"
  `--color-ink-muted`, cuối ô gợi ý phím `/` (kbd 11px mono). Phím **/** ở bất cứ đâu trong Chat (trừ khi đang gõ
  trong ô nhập) focus ô và nhớ phần tử trước đó. Có chữ mà ô không focus: `/` thay bằng ✕ (IconButton 32, "Xoá chữ
  đã nhập").
- **Bề mặt khi mở (máy tính, mọi module):** neo đúng mép trái của viên thuốc, đỉnh lệch −4; rộng
  `min(640px, mép phải viewport − mép trái ô − 16px)` cho Chat, `max(bề rộng ô, 440px)` (cùng giới hạn viewport) cho module
  khác; bo 12, `--color-overlay`, `--shadow-overlay`, `--z-dropdown`, không scrim, không đẩy gì. Hàng nhập cao 48 (icon
  và chữ đứng yên đúng chỗ cũ, con trỏ không nhảy); divider 1px `--color-line`; danh sách cao tối đa 448 rồi cuộn; chân
  36 có divider: ↑ ↓ chọn · Enter mở · Delete xoá lần tìm cũ · Esc đóng (11px mono, mờ). Bấm ra ngoài hoặc Esc: thu lại.
- **Chuyển động máy tính** (ngoại lệ có chủ đích của §7, chỉ cho bề mặt này: animate `clip-path`, vì `scale` làm méo chữ
  và bo góc, `width` thì tính lại bố cục; bề mặt dàn sẵn ở kích thước mở, ô nhập là một phần tử nên focus và bộ gõ không
  bị ngắt):
  - *Mở:* `clip-path: inset(4px calc(100% − 256px) calc(100% − 44px) 0 round 20px)` → `inset(-24px -24px -56px -24px round
    12px)` (âm để giữ bóng), 280ms `--ease-out-expo`; lớp nền `--color-sunk` fade ra 100ms `--ease-out` (lộ
    `--color-overlay`); lớp bóng fade vào 220ms `--ease-out`; hàng gợi ý `translateY(-4px)` + fade 160ms `--ease-out`,
    bắt đầu sau 60ms, lệch 20ms mỗi hàng, tối đa 8 hàng lệch.
  - *Đóng:* danh sách và chân fade ra 120ms `--ease-in`, clip về viên thuốc 210ms `--ease-in`, bóng tắt cùng lúc; chữ ở
    lại trong ô.
  - *Đổi nhóm khi gõ:* hàng mới fade 120ms, không trượt, không lệch.
- **Điện thoại (< 1024):** không có popover. Chạm viên thuốc 52 của thanh trên (§5): viên thuốc **đứng yên tại chỗ**, ô
  chữ cái workspace xoay thành ← (← xoay 90° → 0 + fade vào, chữ cái xoay −90° + fade ra) (160ms `--ease-out`), avatar fade ra 160ms, viền trong 2px
  `--color-accent` (lớp riêng) fade vào 160ms, danh sách màn fade ra 120ms `--ease-in`, panel gợi ý `translateY(8px)` +
  fade 220ms `--ease-out` sau 60ms, hàng lệch 20ms. Focus đồng bộ trong cú chạm để bàn phím mở ngay. ← hoặc back hệ điều
  hành: ngược lại 160ms, giữ vị trí cuộn. Hàng 52, icon 36, tên 15/500; ✕ luôn hiện, vùng chạm 44. Cất bàn phím không
  đóng gợi ý.
- **Ô trống: nhóm gợi ý.** "Tìm gần đây" (tối đa 5 máy tính / 8 điện thoại) rồi nhóm của module (tối đa 3 / 5): Chat:
  Trò chuyện gần đây (tin cuối, giờ). Danh bạ: Nhắn gần đây (người, chức danh · phòng ban; không có chính mình). Tài liệu:
  Sửa gần đây trong <thư mục> (không có thư mục) · Được chia sẻ với tôi (chỉ tải khi gợi ý mở lần đầu). Tài sản: Bạn đang
  giữ · Hoạt động gần đây ("**Tên** việc · thời gian"). Phê duyệt: Chờ bạn duyệt (pill Khẩn khi có) · Đề nghị của bạn.
  Workspace cá nhân không có Tài sản và Phê duyệt. Nhãn nhóm 12/600 mờ, cao 32, không divider giữa nhóm. Hàng 44, bo 8,
  icon 32, tên 14/500, dòng phụ 12 mờ; hover `--color-hover`; hàng đang chọn `--color-accent-wash`. Icon: đồng hồ trên
  `--color-sunk` cho lần tìm cũ; ô loại tệp theo `file-kind`; Avatar người; ô nhóm vuông. Dòng phụ luôn là tên người và
  thời gian, không bao giờ là ID.
- **Tìm gần đây:** lưu chuỗi đã gõ (≥ 2 ký tự) khi bấm Enter hoặc mở một kết quả; theo người, theo module, trên máy
  này; trùng khi bỏ hoa thường và dấu thì giữ cái mới; tối đa 8; đăng xuất thì xoá hết.
  - *Xoá một mục:* ✕ (IconButton 32; máy tính hiện khi rê hoặc khi hàng được chọn bằng phím, fade 160ms; điện thoại luôn
    hiện, 44) hoặc Delete khi hàng được chọn và ô trống. Hàng fade + co chiều cao 160ms `--ease-in`, hàng dưới trôi lên;
    lựa chọn sang hàng kế (hết thì hàng trước). Mục cuối: nhãn nhóm co cùng lúc. Không toast.
  - *Xoá tất cả:* nút chữ 12/600 `--color-accent` cao 28 (điện thoại 13px, cao 44) bên phải nhãn "Tìm gần đây", chỉ khi
    có ≥ 2 mục. **Không hỏi xác nhận**: mọi hàng fade + co cùng lúc 160ms, nhóm sau trôi lên, focus về ô; toast "Đã xoá
    N lần tìm gần đây" + **Hoàn tác** (5s; máy tính đáy phải, điện thoại đáy giữa) khôi phục đúng thứ tự, hàng vào như khi
    mở. Cùng lý do với "Xoá mục": không mất dữ liệu, hoàn tác được.
  - *Hết lần tìm cũ:* nhóm biến mất hẳn. Không có gì để gợi ý: máy tính vẫn nở bề mặt và hiện icon 24 + "Tìm tin nhắn,
    người và nhóm."; điện thoại icon 24 + câu đó + dòng mờ "Sau khi tìm, lọc theo người gửi, cuộc trò chuyện hay ngày."
- **Gõ chữ (Chat):** từ 1 ký tự không khoảng trắng, nhóm gợi ý nhường chỗ cho ba nhóm khớp (bỏ hoa thường và dấu; phần
  khớp tô `--color-warning-wash`): **Người** (tối đa 3: Avatar, tên, "chức danh · phòng ban"; không có chính mình; chọn
  = mở trò chuyện trực tiếp, tạo nếu chưa có) · **Nhóm và trò chuyện** (3: ô nhóm/Avatar, tên, "Nhóm · N thành viên" /
  "Trò chuyện trực tiếp"; người đã có ở nhóm Người thì không lặp ở đây; chọn = mở) · **Tin nhắn** (3, từ ≥ 2 ký tự, gọi
  máy chủ sau 250ms ngừng gõ, 2 hàng Skeleton trong lúc chờ: Avatar người gửi, đoạn khớp 1 dòng, "Người gửi · Cuộc trò
  chuyện · giờ"; chọn = mở cuộc trò chuyện, tin vào giữa, loé viền accent 2,4s). Hàng cuối luôn là **Xem tất cả kết quả
  cho "…"** (icon tìm, kbd Enter). Không mục nào chọn sẵn: Enter khi chưa chọn = Xem tất cả. Không khớp gì: chỉ hàng đó +
  dòng mờ "Không có gợi ý. Nhấn Enter để tìm trong mọi tin nhắn." Tin nhắn lỗi: nhóm ẩn, không báo trong gợi ý.
  Module khác: gõ chữ thì danh sách gợi ý thu lại (bề mặt về viên thuốc) và màn lọc tại chỗ như cũ.
- **Trang kết quả (Chat):** Enter thu bề mặt, giữ chữ, mở trang kết quả ở cột nội dung; URL `/channels/search?q=…` mang
  cả bộ lọc (Quay lại về cuộc trò chuyện trước; khoá trong URL không bao giờ lên màn hình). Điện thoại: ngay trong màn tìm
  toàn màn, dưới viên thuốc.
  - *Đầu trang* cao 56: IconButton ← "Đóng tìm kiếm" (về cuộc trò chuyện trước, focus về ô tìm), "Kết quả tìm kiếm"
    display 19/600, phải "N tin nhắn" 14px mờ `tabular-nums` (đổi số fade 160ms; còn trang sau mà chưa biết tổng: "Hơn
    25 tin nhắn"). Điện thoại: chỉ dòng "N tin nhắn" 13px mờ dưới hàng chip.
  - *Hàng chip* (`toolbar` "Bộ lọc", cách 8, đệm ngang 20; điện thoại 12, cuộn ngang với `ScrollFade`): **Người gửi ·
    Trong · Ngày · Có tệp · Có liên kết · Nhắc đến tôi**. Chip cao 32 (điện thoại 36), pill, đệm 0 12, 13/600 (điện thoại
    14). Mặc định `--color-sunk`, hover `--color-hover`; ba chip đầu có chevron 16 mờ (xoay 180° khi mở, 160ms). Bật: lớp
    `--color-accent-wash` + viền trong 1px `--color-accent` fade vào 160ms `--ease-out`. Chip bật/tắt (`aria-pressed`)
    thêm ✓ 16 accent `scale(.6)` → 1 + fade 160ms. Chip giá trị đổi nhãn thành giá trị (Avatar 20 + tên; ô nhóm 20 + tên;
    "Hôm nay", "7 ngày qua", "01/10 – 15/10") + ✕ 28 ("Bỏ lọc Người gửi: …", focus về chip sau khi bỏ). Chip phía sau dời
    bằng FLIP 220ms `--ease-out-expo`. Có ≥ 1 chip bật: nút chữ "Xoá bộ lọc" cuối hàng. Chọn là áp dụng ngay; không nút
    "Tìm", không bảng "Tìm nâng cao". Điện thoại: chip vừa bật cuộn vào tầm nhìn.
  - *Danh sách chọn* (máy tính: Popover 320, cách chip 6, `translateY(-4px)` + fade 160ms, ra 120ms; điện thoại: bottom
    sheet theo §5, scrim 220ms, `translateY(100%)` → 0 280ms expo, ra 210ms, bo 24, tay kéo 32×4, cao tối đa 85%).
    **Người gửi:** ô "Tìm người" (36; sheet: viên thuốc 44, 16px) + "Bạn (tên)" đầu danh sách + người trong workspace
    (Avatar 32/36, tên, chức danh · phòng ban), một người. **Trong:** ô "Tìm cuộc trò chuyện" + các cuộc còn quyền đọc
    (ô nhóm/Avatar, tên, "Nhóm · N thành viên" / "Trò chuyện trực tiếp"), một cuộc. **Ngày:** Bất kỳ lúc nào · Hôm nay ·
    7 ngày qua · 30 ngày qua · divider · Tuỳ chọn khoảng… (mở "Từ ngày" "Đến ngày" 36/44 + Áp dụng). Hàng 44 (sheet 56,
    radio 36/52), mục đang chọn có ✓ accent. Chọn = áp dụng + đóng. Không bao giờ có ô nhập khoá.
  - *Bộ lọc → máy chủ* (backend làm sau, mở rộng `GET /api/messages/search`): người gửi, cuộc trò chuyện, khoảng ngày
    (theo múi giờ người dùng), có tệp, có liên kết, nhắc đến tôi; vẫn chỉ trả tin trong cuộc còn quyền đọc.
  - *Khối "Nhóm và người"* (chỉ máy tính, chỉ khi chưa bật bộ lọc): tối đa 3 hàng như gợi ý, trên nhãn "Tin nhắn".
  - *Thẻ kết quả:* ô nhóm/Avatar 20 + tên cuộc 13/600 mờ; Avatar 32 + tên người gửi 15/600 + giờ 12 mờ ("09:12", "Hôm qua
    16:05", "Thứ Năm 10:41", "02/10"); nội dung 15/23 (điện thoại 16/24) tối đa 3 dòng, từ khớp `--color-warning-wash`,
    @tên theo mẫu nhắc tên; tệp/liên kết: chip 36 `--color-sunk`. Đệm 12 (điện thoại 12/16, tràn mép), bo 10, hover
    `--color-hover`, divider 1px `--color-line`. Bấm = mở cuộc trò chuyện tại tin (loé viền 2,4s). Mới nhất trước, 25 mỗi
    trang, tải thêm khi cuộn tới cuối.
  - *Chuyển động:* lần đầu có kết quả: thẻ `translateY(8px)` + fade 220ms `--ease-out`, lệch 30ms, tối đa 8. Đổi bộ lọc:
    danh sách cũ còn 50% opacity trong 160ms (`aria-busy`), kết quả mới thay bằng fade 160ms, không trượt. Trang sau: như
    lần đầu.
  - *Trạng thái:* tải lần đầu 4 thẻ Skeleton cùng hình. Không kết quả có bộ lọc: icon 24 + "Không có tin nhắn nào khớp
    "…" với các bộ lọc này." + **Xoá bộ lọc**; không bộ lọc: "Không có tin nhắn nào khớp "…". Thử từ khác hoặc viết ngắn
    hơn." Lỗi: icon 24 + "Chưa tìm được. Kiểm tra kết nối rồi thử lại." + **Thử lại**.
- **Giảm chuyển động:** bề mặt đổi ngay rồi fade 120ms; không `transform`, không lệch hàng, không FLIP; xoá hàng chỉ fade
  120ms; ô chữ cái/← đổi ngay; sheet chỉ fade ≤ 120ms; Skeleton không chạy sáng.
- **Bàn phím (máy tính):** ↑/↓ đi qua mọi hàng mọi nhóm (vòng), focus vẫn ở ô; Enter chọn hàng hoặc Xem tất cả; Delete xoá
  lần tìm cũ đang chọn (chỉ khi ô trống); Tab tới "Xoá tất cả" (thứ duy nhất Tab tới được trong bề mặt), Tab lần nữa
  rời và đóng, Esc ở đó về ô; **Esc** lần 1 thu bề mặt giữ chữ, lần 2 xoá chữ, lần 3 rời ô và trả focus về chỗ trước `/`
  (mở bằng chuột thì ở lại ô). Trong danh sách chọn của chip: focus vào ô tìm, ↑↓ Enter, Esc đóng và trả focus về chip.
  Trang kết quả: Esc không đóng trang; ←/→ đi giữa chip.
- **A11y:** máy tính: ô tìm là `combobox` (`aria-expanded`, `aria-controls`, `aria-autocomplete="list"`,
  `aria-activedescendant`); gợi ý là `listbox` chia `group` có nhãn; option có tên đầy đủ ("Tìm lại …", "Trần Lan Anh, Kế
  toán viên", "Tin của … trong …: …"); ✕ máy tính là của chuột (`aria-hidden`, `tabindex=-1`). Một vùng
  `aria-live="polite"`: "N gợi ý", "Đã xoá … khỏi Tìm gần đây", "Đang tìm…", "N tin nhắn" / "Không có kết quả". Chip mở
  danh sách: `aria-haspopup="dialog"` + `aria-expanded`, tên gồm cả giá trị ("Người gửi: Trần Lan Anh"); chip bật/tắt:
  `aria-pressed`. Trang kết quả: `region` "Kết quả tìm kiếm", `aria-busy` khi tải, danh sách link. Điện thoại: ô tìm
  thường; mỗi gợi ý là nút ("Tìm lại …"), ✕ là nút riêng ("Xoá "…" khỏi Tìm gần đây"); sheet là modal đầy đủ.
- **Quyền:** gợi ý chỉ lấy từ dữ liệu đã qua kiểm tra quyền, tính lại mỗi lần mở; không nhớ thực thể theo tên hay ID, nên
  thứ đã mất quyền tự biến mất. Danh sách chọn Trong chỉ có cuộc còn quyền đọc.
- **Ô tìm của Phê duyệt:** `SearchField` 240 đầu hàng dưới tab, `moduleSearch`; lọc danh sách của tab đang mở
  (đã tải) theo tiêu đề, người gửi, tên mẫu; bỏ hoa thường và dấu, mọi từ phải khớp, từ khớp tô `--color-warning-wash`;
  bên cạnh ghi "N trong M đề nghị"; giữ trong URL `?q=`, đổi tab thì xoá.

### Danh bạ (mockup `design/mockups/contacts-lark.html`, duyệt 2026-10-17)

Danh bạ kiểu Lark: list panel chọn danh mục (§5), content duyệt danh mục có đường dẫn, detail panel là thẻ hồ sơ. Mục đích
(Zane, 2026-10-17): một màn không tải và không hiện quá nhiều. Thay bảng phẳng + chip phòng ban của
`contacts-documents-settings.html` §1.
- **Danh mục:** workspace tổ chức: **Phòng ban** (gốc là tên tổ chức, gồm cả người chưa thuộc phòng nào) · **Vai trò**
  (vai trò quản trị viên tạo + "Quản trị viên" đứng đầu, icon khiên; không liệt kê "Thành viên") · **Nhóm của tôi** (nhóm
  chat riêng bạn đang ở; không có nhóm nào thì ẩn mục) + lối tắt **Phòng của tôi** (vào thẳng phòng mình). Workspace cá
  nhân: **Thành viên** (liệt kê thẳng) · **Nhóm của tôi**; không Phòng ban, Vai trò, Phòng của tôi. Không có mục "Tất cả
  thành viên": tìm người thì dùng ô tìm, xem cả công ty thì vào Phòng ban › gốc. "Bên ngoài" và "Danh bạ chia sẻ" là đợt
  2, không hiện mục rỗng. Ô danh mục: hàng 56 (điện thoại 64), ô icon 40 radius 12 trên `--color-person-N-wash` icon
  `--color-person-N`, tên + một dòng mô tả mờ. Số đếm là **chữ mờ** (`--color-ink-muted`, `tabular-nums`), không phải pill
  accent (pill accent nghĩa là chưa đọc).
- **Bấm mới tải:** mỗi danh mục gọi API của nó khi được mở (cây phòng ban, danh sách vai trò, nhóm của bạn; thành viên một
  nhóm khi mở nhóm đó). Đang tải: Skeleton **cùng hình hàng thật** (ô phòng vuông, avatar tròn), không spinner. Danh sách
  người thì app đã tải sẵn.
- **Đường dẫn:** icon toà nhà là gốc (`aria-label` = tên tổ chức), mỗi đoạn là nút lên đúng cấp đó, đoạn cuối là trang hiện
  tại (`aria-current="page"`, chữ 600). Dài thì các đoạn giữa co lại bằng "…", **đoạn cuối không bao giờ bị cắt**. Vai trò và
  nhóm: "Vai trò › X", "Nhóm của tôi › X". Không giới hạn số cấp; một cấp mỗi màn.
- **Hàng phòng** (trước hàng người): ô thư mục 32 radius 8 `--color-accent-wash` icon accent, tên, "N người" mờ (tính cả
  cây con), chevron; còn cấp con thì dòng phụ "N phòng, nhóm bên dưới". Nhãn mục "Khối · N" / "Phòng ban · N".
- **Hàng người:** cao 52, avatar 32 (chữ cái đầu của tên hiển thị trên `--color-person-N`, chấm presence), tên, chức danh,
  **trạng thái bằng chữ** ("Trực tuyến" `--color-success` / "Ngoại tuyến" mờ). Ngoài cây (tìm, vai trò, nhóm) dòng phụ là
  chức danh · đường dẫn phòng đầy đủ, đoạn đầu co bằng "…", đoạn cuối luôn đủ và màu chữ chính (hai phòng trùng tên khác
  nhánh vẫn phân biệt). Hàng đang chọn `--color-accent-wash`. Chính mình có "(bạn)" mờ. **Xếp theo tên (chữ cuối)** rồi cả
  họ tên, so sánh tiếng Việt.
- **Tìm:** luôn tìm trong toàn bộ phạm vi được xem, không chỉ phòng đang mở; placeholder "Tên, chức danh hoặc email",
  bỏ hoa thường và dấu, từ khớp tô `--color-warning-wash`, chỉ tìm người. Đầu cột "Kết quả trong cả <tổ chức>" + "N người
  khớp"; ✕ hoặc Esc trả về đúng chỗ trước đó. Chip "Đang trực tuyến · N" lọc danh sách đang xem. Gợi ý khi trống theo
  "Gợi ý trong ô tìm".
- **Thẻ hồ sơ** (detail panel 360; 1024–1279 overlay phải; tablet sheet phải 380 `--color-overlay` trên `--color-scrim`;
  điện thoại toàn màn, nút quay lại 44 nổi trên bìa): dải bìa cao 88 (điện thoại 112) chuyển từ `--color-person-N-wash` sang
  `--color-accent-wash` (màu của chính người đó, không ảnh); **avatar 64** đè lên bìa, viền 4px màu nền thẻ; tên `text-lg`
  700, chức danh mờ, StatusPill "Trực tuyến" / "Ngoại tuyến". Ba nút cao 64 chia đều: **Nhắn tin** (primary, mở hoặc tạo
  tin nhắn trực tiếp) · **Gọi** (cuộc gọi 1-1) · **Gửi email** (`mailto:`). **Gọi luôn bấm được**, kể cả khi người kia ngoại
  tuyến (đổ chuông thiết bị của họ). Hồ sơ của chính bạn: một nút **"Sửa hồ sơ"** (sang Cài đặt › Hồ sơ) thay cho ba nút. Không
  có số điện thoại.
- **Dòng chi tiết** (hàng 40, icon 16 mờ, nhãn 84, divider `--color-line`; điện thoại nhãn trên giá trị): Email + IconButton
  "Sao chép email" · Phòng ban = **đường dẫn đầy đủ, bấm mở phòng đó** · Vai trò (mỗi vai trò bấm mở danh sách của nó) · Nơi
  làm việc. Trường trống thì bỏ cả dòng. **Cùng phòng:** nhãn 12/600 hoa, chồng avatar 24 (tối đa 4) + "<tên> và N người
  khác", bấm mở phòng. Workspace cá nhân: không Phòng ban, Vai trò, Cùng phòng.
- **Phạm vi hiển thị** (quản trị viên đặt ở Quản trị › Danh bạ, NGAC quyết ở API, giao diện không tự lọc): người bị ẩn
  không có trong cây, kết quả tìm, số đếm, danh sách vai trò. Người bị giới hạn: danh mục Phòng ban ghi **"Chỉ phòng của
  bạn"**, tìm ghi **"Kết quả trong phạm vi bạn được xem"**; không câu nào nói có người bị ẩn. Quản trị viên thấy tất cả; phòng
  bị ẩn có pill `--color-warning-wash` **"Ẩn khỏi danh bạ"** sau tên.
- **Thẻ người bị ẩn:** tên và avatar của họ vẫn hiện ở nơi đã cùng làm việc (tin nhắn, phê duyệt, tài liệu, lịch); bấm tên
  hoặc avatar mở popover nhỏ (`--color-overlay`, `--shadow-overlay`, radius 12, padding 16, tối đa 300): avatar 40 + tên, rồi
  icon cảnh báo mờ + **"Hồ sơ này không còn hiển thị với bạn."** Không chức danh, phòng, email, không Nhắn tin hay Gọi. Trả
  lời trong cuộc trò chuyện đang có vẫn được.
- **Trạng thái content:** Đang tải (Skeleton trên) · Phòng trống ("<Phòng> chưa có ai" + "Về <phòng cha>") · Tìm không ra
  ("Không có ai khớp “…” trong cả <tổ chức>" + "Xoá tìm kiếm") · Lọc trực tuyến không ra ai ("Bạn vẫn nhắn được; họ thấy tin
  khi quay lại." + "Hiện mọi người") · Lỗi ("Không tải được danh bạ" + "Thử lại"). Mỗi nguồn hỏng riêng: cây phòng ban lỗi
  thì báo lỗi tại chỗ dưới danh mục Phòng ban ("Không tải được cây phòng ban. Bạn vẫn tìm được người bằng ô tìm." + Thử lại),
  Vai trò, Nhóm của tôi và ô tìm vẫn dùng được.
- **Điện thoại (mockup `design/mockups/mobile-shell.html` §12, duyệt 2026-10-19; vỏ theo §5):**
  - *Phòng ban chỉ có ở workspace tổ chức.* Ở workspace cá nhân không có ô Phòng ban, Phòng của tôi, Vai trò, chip lọc Phòng
    ban, dòng Phòng ban và Cùng phòng trên thẻ hồ sơ, và dòng phụ của người chỉ là chức danh (không "· phòng").
  - *Màn đầu:* tiêu đề "Danh bạ" + ⋯ (quản trị viên: "Mời thành viên" → Cài đặt workspace); **Thường liên lạc** — tối đa 8
    avatar 52 cuộn ngang (tên ngắn 12px dưới), lấy từ tin nhắn trực tiếp gần đây, chỉ người còn trong phạm vi được xem, ẩn khi
    chưa nhắn ai; chạm mở thẻ hồ sơ, nhấn giữ mở Nhắn tin · danh mục thành **lưới 2 cột** (ô `--color-raised` radius 16, icon 40
    trên person-wash, tên 15/600 + dòng mờ): Phòng ban · Phòng của tôi (khi người xem thuộc một phòng) · Vai trò · Nhóm của
    tôi; workspace cá nhân: Thành viên · Nhóm của tôi · **Đang trực tuyến · N** (tối đa 5 hàng + "Xem tất cả").
  - *Trong phòng / vai trò / nhóm:* thanh trên ← (lên cấp trên) · tên · kính lúp (ô tìm §6 "Tìm cho mọi tác vụ", phạm vi
    Người, lọc sẵn đúng phòng đang mở) — không ô tìm trong trang; đường dẫn 13px dưới thanh; chip "Đang trực tuyến · N"; hàng
    người 60, avatar 40. Từ 30 người: cột chữ cái A–Z 18px bên phải (theo chữ cái đầu của **tên**, thứ tự tiếng Việt), chạm
    hoặc kéo để nhảy, chữ đang chọn hiện to giữa màn khi kéo. Vuốt trái hàng người lộ **Nhắn tin** (accent) · **Gọi** (success).
  - *Thẻ hồ sơ:* bìa 120 trong vùng cuộn, nút ← và ⋯ nổi; avatar 80 viền 4px đè bìa; tên display 22/700, chức danh, pill trạng
    thái; **4 nút đều** cao 64: Nhắn tin (primary) · Gọi · **Lịch** (mở Lịch với lịch của người đó chồng lên, theo quyền xem lịch
    hiện có) · Email (hiện địa chỉ, chép được); dòng chi tiết theo nhóm iOS (nhãn trên giá trị); Cùng phòng; **Đã cùng làm việc**:
    "N nhóm chung" (tên nhóm, mở danh sách) · "Tệp đã chia sẻ với bạn" (N tệp) — chỉ những gì người xem đọc được. ⋯: Sao chép
    email · Thêm vào nhóm… · Mời vào cuộc họp. Hồ sơ của chính bạn: "Sửa hồ sơ" mở Tôi › Tài khoản và hồ sơ.
  - *Tìm người:* ô tìm §6 với chip phạm vi Người và chip lọc Phòng ban · Vai trò · Đang trực tuyến; dòng "Kết quả trong cả <tổ
    chức> · N người khớp" (hoặc "trong phạm vi bạn được xem"); mỗi hàng có nút Nhắn tin.
- **Quản trị › Danh bạ** (mockup `design/mockups/admin.html` §7b; Claude chốt 2026-10-17 theo uỷ quyền của Zane, khớp
  §1c của mockup đã duyệt): tab thứ tư của Quản trị, chỉ ở workspace tổ chức và chỉ khi workspace đã có dữ liệu phạm vi
  (chưa có thì không hiện tab, không báo lỗi). Câu đầu "Mặc định mọi thành viên thấy toàn bộ danh bạ. Quản trị viên luôn thấy
  tất cả." Hai Panel: **Phòng ẩn khỏi danh bạ · N** (hàng phòng như trên, dòng phụ "đường dẫn cha · Chỉ N người trong phòng
  và quản trị viên thấy", Button sm "Bỏ ẩn"; cuối panel "Ẩn một phòng" mở DepartmentPicker) và **Chỉ thấy phòng của mình · N**
  (ô vai trò 32 hoặc ô phòng 32, "Vai trò X" / "Phòng Y", "N người · thấy người trong phòng của họ", "Bỏ"; cuối panel "Thêm
  vai trò hoặc phòng" mở picker hai nhóm: Vai trò (chỉ vai trò tự tạo) và Phòng ban (Tree)). Ẩn, Bỏ ẩn, Bỏ đều hỏi lại bằng
  Modal nói hệ quả bằng người ("18 người chỉ còn được người trong khối và quản trị viên thấy"; ẩn một phòng là ẩn cả cây con),
  xong thì toast. Trong picker, mục đã có quy tắc bị vô hiệu kèm nhãn ("Đã ẩn", "Ẩn theo <phòng cha>", "Đã giới hạn").
  Dưới 768 picker là bottom sheet.

### Cài đặt: Tôi, cài đặt phân hệ, workspace cá nhân và tổ chức (mockup `design/mockups/mobile-shell.html` §3, §4, §8, duyệt 2026-10-19)

Mỗi tuỳ chọn có **một chỗ ở**; lối vào khác nhau mở cùng một trang. Cài đặt chia hai lớp: **Tài khoản** (theo người, mọi
workspace dùng chung) và **Trong workspace này** (theo workspace). Máy tính giữ trang Cài đặt có tab, đọc cùng dữ liệu.
- **Tôi (dưới 1024; chạm avatar ở thanh trên):** trang toàn màn trượt từ phải, ẩn thanh đáy; thay bốn lối vào cũ (☰ → Cài
  đặt, sheet tài khoản với Cài đặt thông báo / Cài đặt tài khoản / Cài đặt, Thêm → Thông báo của Chat). Từ trên: thẻ hồ sơ
  (avatar 64, tên display 18/700, dòng phụ — tổ chức: "chức danh · phòng"; cá nhân: email — pill trạng thái, pill loại
  workspace) · Tạm dừng thông báo · nhóm **Tài khoản · dùng chung mọi workspace**: Tài khoản và hồ sơ · Thông báo · Giao diện
  và ngôn ngữ · Thiết bị và phiên · nhóm **Trong <workspace>**: (tổ chức) Hồ sơ trong tổ chức · Chat · Lịch · Tài liệu · Phê
  duyệt · Tài sản · Thanh đáy; (cá nhân) Chat · Lịch (chỉ lịch cá nhân) · Tài liệu · Thanh đáy · **Cài đặt workspace**
  (một dòng, mở trang dưới đây) · Giới thiệu (phiên bản `2026.10.18 · 6368e8e`) · Đăng xuất (danger, chỉ ở đây).
- **Danh sách nhóm (iOS/Lark):** nhóm nền `--color-raised` radius 16, cách mép 16; tiêu đề nhóm 13/600 mờ ở ngoài; hàng ≥ 52,
  ô icon 32 radius 9 `--color-sunk`, nhãn 15/500 + dòng mờ 13px, giá trị hiện tại mờ bên phải, chevron `--color-ink-subtle`;
  divider `--color-line` giữa các hàng. Công tắc 44×26 accent khi bật. Chọn giá trị (Sắp xếp, Tuần bắt đầu…): trang con một
  cột radio, chạm là chọn và quay lại. Sửa một trường (Tên hiển thị, Chức danh…): trang con có ô nhập, Lưu ở thanh trên (bật
  khi có thay đổi), quay lại = huỷ.
- **Lưu:** công tắc và radio lưu ngay khi chạm; lưu lỗi thì trả lại giá trị cũ + toast "Chưa lưu được…".
- **Thông báo (Tôi → Thông báo):** Thông báo đẩy trên máy này · Giờ yên lặng (giờ; push bị chặn trừ khi bạn được nhắc tên, kể cả @Tất cả) · Xem trước nội
  dung trên màn khoá · nhóm Theo phân hệ: Chat · Lịch · Phê duyệt · Tài liệu · Tài sản (dòng mờ nói loại việc) — mỗi dòng
  mở mục Thông báo của trang Cài đặt <phân hệ>. Mức thông báo từng nhóm đặt trong cài đặt nhóm đó.
- **Cài đặt <phân hệ>** (⋯ của phân hệ → dòng đầu, hoặc Tôi → dòng phân hệ):
  - *Chat:* Thông báo (Tin nhắn trực tiếp · Được nhắc tên · Thread đang theo dõi · Mức mặc định cho nhóm mới) · Soạn và đọc
    (Enter để gửi · Xem trước liên kết · Cảm xúc khi chạm đúp: 👍 / Tắt) · Danh sách (Mở Chat ở: Tất cả / Mục · Sắp xếp ·
    Quản lý mục).
  - *Lịch:* Hiển thị (Chế độ xem mặc định · Tuần bắt đầu vào · Múi giờ · Lịch đang hiện) · Sự kiện mới (Thời lượng mặc định ·
    Nhắc trước) · Thông báo (Lời mời mới · Thay đổi sự kiện · Nhắc sự kiện).
  - *Tài liệu:* Hiển thị (Xem dạng · Sắp xếp · Thư mục trước tệp) · Dữ liệu di động (Tải ảnh xem trước) · Thông báo (Được
    chia sẻ tài liệu).
  - *Phê duyệt, Tài sản:* cùng khuôn — Thông báo theo loại việc + Hiển thị (tab mở đầu, sắp xếp). Danh bạ không có trang
    cài đặt.
- **Cài đặt workspace (mockup §11):** mọi thứ thuộc về chính workspace, một trang; mở từ nút ⚙ trên khối workspace trong
  Thêm, từ sheet chọn workspace, hoặc Tôi → Cài đặt workspace. Thay Cài đặt → Workspace và ô Quản trị trên điện thoại (ô
  Quản trị bỏ khỏi lưới Thêm; máy tính giữ màn Quản trị). Đầu trang: ô chữ cái workspace 72 radius 20 · tên · "Workspace
  tổ chức · N thành viên" / "Workspace cá nhân · chỉ bạn" · pill vai trò của người xem. Mỗi dòng chỉ hiện khi người xem làm
  được việc đó (kiểm tra quản trị hiện có); server quyết định ở mọi màn.
  - *Tổ chức, quản trị viên:* Thông tin (Tên · Mô tả) · Thành viên và tổ chức (Thành viên · Mời thành viên + "N lời mời
    đang chờ" · Phòng ban · Vai trò và quyền · Phạm vi danh bạ) · Quy định (Ai được tạo nhóm) · Phân hệ (Mẫu phê duyệt ·
    Loại tài sản · Dung lượng Tài liệu "đã dùng / tối đa · N tệp", chạm để đổi hạn mức) · Rời <workspace>. Mỗi dòng mở màn
    Quản trị hiện có với bố cục điện thoại (danh sách, sheet chi tiết thay panel bên phải).
  - *Tổ chức, thành viên:* Thông tin chỉ đọc · Quản trị viên (cụm avatar → danh sách) · Quy định (giá trị + "Do quản trị
    viên đặt") · Hồ sơ trong tổ chức · Rời <workspace> (chủ sở hữu cuối cùng phải chuyển quyền trước).
  - *Cá nhân:* Tên workspace · Dung lượng Tài liệu. Không thành viên, phòng ban, vai trò, quy định hay Rời.
- **Workspace cá nhân và tổ chức:** cùng vỏ, khác nội dung. Cá nhân: không Phê duyệt, Tài sản, Quản trị, phòng ban, Hồ sơ
  trong tổ chức, Rời; không dòng nào bị khoá. Tổ chức: quản trị viên có thể **khoá** một tuỳ chọn cho cả tổ chức — dòng
  khoá mờ 72%, hiện giá trị + dòng "🛡 Quản trị viên <workspace> đã đặt" (không nêu tên người đặt, vì đồ thị không ghi người đặt),
  không chạm được; server là nơi chặn. Đợt này chỉ khoá "Ai được tạo nhóm": khoá = bỏ quyền tạo nhóm của Thành viên và của
  mọi vai trò tự tạo (chủ sở hữu giữ quyền); màn Vai trò và quyền không cho cấp quyền tạo nhóm khi đang khoá; mở khoá trả lại
  quyền cho Thành viên, vai trò tự tạo không tự được trả lại.

### Cài đặt cuộc trò chuyện (mockup `design/mockups/mobile-shell.html` §5, duyệt 2026-10-19)

- **Thanh trên trong cuộc trò chuyện (điện thoại):** quay lại · nút tên (avatar 28 + tên display 17/700 + chevron nhỏ + dòng
  "N thành viên" / chức danh) mở trang cài đặt · Gọi · Thread. "Tìm trong nhóm" và ⓘ rời thanh trên. Máy tính: cùng nội
  dung là panel Thông tin bên phải, mở bằng ⓘ như hiện nay.
- **Cài đặt nhóm:** đầu trang ảnh nhóm 72 (radius 20) · tên display 20/700 · mô tả mờ + "Sửa" (người có quyền) · 4 lối tắt
  (ô `--color-raised` radius 16, icon 20 + nhãn 13px): Tìm · Thông báo · Gắn sao · Thêm người · nhóm Nội dung: Thành viên (cụm
  avatar 28 + số) · Tệp, ảnh và liên kết (số) · Tin đã ghim (số) · nhóm Của bạn: Thông báo (mức) · Mục
  (sheet chọn mục) · Quản lý nhóm (chỉ chủ nhóm và quản trị nhóm) · Rời nhóm (danger, hộp xác nhận; chủ nhóm phải chuyển
  quyền trước).
- **Cài đặt trò chuyện (DM):** avatar 64 · tên · chức danh · phòng (tổ chức) · lối tắt Hồ sơ · Tìm · Thông báo · Gắn sao · Tệp,
  ảnh và liên kết · Tin đã ghim · Thông báo (mức) · Mục · "Tạo nhóm với <tên>" (accent).
- **Thành viên:** danh sách có ô tìm, vai trò trong nhóm (Chủ nhóm / Quản trị nhóm / Thành viên); nhấn giữ một người: Nhắn
  riêng · Đặt làm quản trị nhóm · Xoá khỏi nhóm (theo quyền). Không bao giờ hiện ID.
- **Quản lý nhóm:** ba nhóm radio — Ai được gửi tin · Ai được thêm người · Ai được nhắc @Tất cả — mỗi nhóm "Mọi thành viên" |
  "Chỉ chủ nhóm và quản trị nhóm" (gửi tin: dòng mờ "Thành viên vẫn trả lời trong thread"; bắt đầu bình chọn, công việc hay
  thẻ phê duyệt trong luồng chính cũng tính là gửi tin, còn bỏ phiếu và đóng bình chọn của mình vẫn mở; @Tất cả: ở kênh
  của cả workspace chỉ quản trị viên workspace dùng được, tin nhắn trực tiếp không có); Người quản lý (chủ nhóm, các
  quản trị nhóm, "Thêm quản trị nhóm"); Chuyển quyền chủ nhóm; Giải tán nhóm (danger, chỉ chủ nhóm). Mỗi lựa chọn là quyết định
  của server; bị từ chối thì trả lại lựa chọn cũ + toast. Mất quyền giữa chừng (bị gỡ quản trị): trang và nút tự ẩn khi
  quyền đổi; thao tác đã gửi bị từ chối thì toast "Bạn không còn quyền làm việc này".

### Tìm cho mọi tác vụ (mockup `design/mockups/mobile-shell.html` §6, duyệt 2026-10-19)

Một ô tìm cho mọi phân hệ; hành vi nở ra, Tìm gần đây, gợi ý khi gõ theo "Gợi ý trong ô tìm" ở trên, nay áp cho mọi phân hệ.
- **Phạm vi:** chip đầu là phân hệ đang mở (mặc định), rồi **Tất cả**, rồi các phân hệ khác (workspace cá nhân: không Phê
  duyệt, Tài sản). Chip đang chọn nền `--color-ink` chữ `--color-base`; đổi phạm vi giữ chữ đã gõ. Sau phạm vi là chip lọc
  viền 1px của phân hệ đó: Chat (Người gửi · Trong · Ngày · Có tệp · Có liên kết · Nhắc đến tôi), Tài liệu (Loại · Người sửa
  · Ngày · Thư mục), Phê duyệt (Trạng thái · Loại · Người gửi · Ngày), Lịch (Lịch · Khoảng ngày), Tài sản (Loại · Trạng thái
  · Người giữ); chip đang lọc nền `--color-accent-wash` viền accent + ✕.
- **Chưa gõ:** Tìm gần đây (riêng từng phạm vi) · Gợi ý · **Làm nhanh** (lưới 4: Tạo đề nghị · Sự kiện mới · Nhắn tin · Cài
  đặt <phân hệ>, theo phân hệ và quyền).
- **Tất cả, đang gõ:** nhóm theo loại — Hành động (lệnh khớp chữ, ví dụ "Tạo đề nghị "đối soát"") · Người · Trò chuyện · Tin
  nhắn · Tài liệu · Đề nghị · Sự kiện · Tài sản — mỗi nhóm 1–3 hàng 52 + "Xem tất cả · N" (accent) chuyển sang phạm vi đó;
  chữ khớp đậm `--color-ink`. Mỗi nhóm hiện ngay khi về, nhóm đang chờ là Skeleton.
- **Trong một phân hệ:** kết quả + chip lọc; cuối danh sách thẻ `--color-sunk` "Có N kết quả ở phân hệ khác · Tìm ở mọi nơi".
- Kết quả chỉ gồm những gì người tìm được phép đọc; không bao giờ hiện ID.

### Vuốt (mockup `design/mockups/mobile-shell.html` §7, §9, duyệt 2026-10-19)

App cài trên iPhone không có cử chỉ quay lại của hệ điều hành; app tự làm. Mọi cử chỉ có nút tương đương; không cử chỉ
nào làm việc không hoàn tác được.
- **Quay lại:** vuốt từ mép trái (vùng 20px) ở mọi màn chi tiết, trang cài đặt, thread: màn hiện tại theo ngón tay, màn trước
  lộ dần; thả quá 35% hoặc vận tốc > 0,5 px/ms là quay lại. Mở app từ thông báo đẩy (không có màn trước): về màn gốc của
  phân hệ. Ở trình duyệt (không cài app) dùng nút ←.
- **Vuốt hàng:** trái lộ 2–3 nút rộng 76 (nhãn 12px dưới icon) — Chat: Tắt thông báo · Gắn sao · Thêm (có Chuyển tới mục); Tài liệu:
  Chia sẻ · Gắn sao · Thêm; Thông báo: Xoá (xoá xong có toast Hoàn tác). Phải ở Chat và Thông báo: Đã đọc / Chưa đọc, vuốt hết (> 60%) làm luôn kèm Hoàn
  tác. Phê duyệt (Chờ tôi): chỉ **lộ** Duyệt… (phải) / Từ chối… (trái), chạm mới mở sheet xác nhận có ô ý kiến; vuốt hết
  không làm gì. Một hàng mở tại một thời điểm.
- **Trong cuộc trò chuyện:** vuốt tin sang phải (64px) = trích dẫn trả lời (ô soạn hiện khung trích, bàn phím mở); sang trái
  = mở thread của tin; cùng hướng cho tin của mình và người khác. Nhấn giữ 400ms rồi trượt lên hàng cảm xúc: biểu tượng
  dưới ngón phóng 1.5 và nhô lên, nhấc tay là chọn, trượt ra ngoài là huỷ; nhấc tay trước khi trượt mở sheet như hiện nay.
  Chạm đúp (≤ 300ms) thả 👍 (đổi hoặc tắt ở Cài đặt Chat), chạm đúp lần nữa bỏ. Kéo luồng tin xuống quá 48px khi bàn phím mở
  thì đóng bàn phím. Kéo vạch trên ô soạn lên quá 25% mở soạn toàn màn. Xoá và sửa tin chỉ có trong nhấn giữ.
- **Khác:** vuốt ngang dải tuần / lưới tháng ở Lịch; kéo xuống ở đầu mọi danh sách để tải lại; kéo sheet, trang Tôi, trình
  xem ảnh/tệp xuống để đóng; trình xem ảnh vuốt ngang chuyển ảnh, chụm hoặc chạm đúp để phóng.
- **Quy tắc chung:** khoá trục sau 10px đầu; bắt đầu trong vùng mép 20px luôn là quay lại; không vuốt ngang đổi tab/chip.
  Theo ngón tay 1:1, thả thì về đích 240ms `--ease-out-expo`; qua ngưỡng thì icon phóng 1.15 + đổi màu accent (iOS không
  có rung cho web). Giảm chuyển động: không theo ngón tay, thả quá ngưỡng là làm ngay. Đọc màn hình: mỗi tin có hành động
  "Trích dẫn", "Trả lời trong thread", "Thả cảm xúc".

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
| Trình xem tệp | desktop: như Modal; điện thoại: scrim fade 220ms + `scale(.98)` → 1, 280ms expo | 210ms ease-in |
| Bản xem trước sẵn sàng | fade 160ms | không |
| Đổi tệp trong trình xem | `translateX(±16px)` + fade theo hướng, 160ms | không |
| Khối sự kiện dời giờ (kéo thả / realtime) | FLIP `transform` tới chỗ mới, 280ms expo; giảm chuyển động: nhảy thẳng | không |
| Khối sự kiện của người khác đổi (realtime) | lớp phủ `person-N-wash` 2400ms + nhãn "vừa dời giờ / vừa nhận lời / vừa thêm", như Realtime bên dưới | tan theo mẫu realtime |

**Ngoại lệ: màn đăng nhập** (mockup `design/mockups/auth-motion.html`, duyệt 2026-10-11). Hai bước
đăng nhập là một luồng đi tới, nên được trượt; mọi thứ vẫn dùng token ở trên, chỉ `transform` và `opacity`.

| Chuyển động | Vào | Ra |
|---|---|---|
| Bước 1 → bước 2 (quay lại: ngược hướng) | `translateX(±16px)` + fade, 280ms expo | `translateX(∓12px)` + fade, 210ms ease-in |
| Email bay từ ô nhập sang "Đã gửi tới …" | `translate` + `scale` của chính dòng chữ, 280ms expo, một bản duy nhất | không |
| Form vào lần lượt | từng phần `translateY(8px)` + fade, 220ms ease-out, lệch 40ms, bắt đầu sau 200ms (160ms ở bước sau) | không |
| Logo tự lắp | nút `scale(.6)` → 1 + fade 160ms, nét `scaleX` 0 → 1 160ms, nút đích 280ms expo; lệch 40 đến 60ms, ~840ms; **một lần mỗi phiên trình duyệt** (cờ `sessionStorage`), các lần sau đứng yên | không |
| Hoạt động ở brand pane | hàng vào như "Chèn hàng"; hàng mới nhất phủ màu người 2400ms + nhãn "vừa tải lên" | lớp phủ tan theo mẫu realtime |
| Nền đồ thị thuộc tính ở brand pane | fade 280ms expo; hai lớp trôi tối đa 16px, một lần khoảng **30s** rồi **đứng yên**; dừng khi tab ẩn | không |
| Gõ một số của mã | ô `scale(.92)` → 1, số `translateY(6px)` → 0 + fade, 160ms ease-out | không |
| Mã sai | rung `translateX` ±6px một lần, 280ms ease-out, viền danger, thông báo "Mã chưa đúng. Còn N lần thử." (aria-live) | không |
| Mã đúng | ô phủ success-wash lệch 40ms, gom vào giữa 220ms ease-in, huy hiệu ✓ `scale(.6)` → 1 280ms expo, thông báo "Đã xác minh." (aria-live), giữ ~700ms rồi chuyển route | route fade 160ms như thường |

Giảm chuyển động (hệ điều hành hoặc "Luôn bật" trong Cài đặt): không có `transform`; mỗi dòng trên
thành fade tối đa 120ms hoặc đổi màu; nền đồ thị **không được vẽ**; email không bay; logo hiện bằng
fade 120ms; lớp phủ realtime ở brand pane vẫn hiện. Không chuyển động nào chặn thao tác: ô nhập nhận
phím và focus ngay từ khung đầu tiên.

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
Điện thoại **không phóng to trang** (người dùng chốt 2026-10-17, vì phóng to làm vỡ bố cục): viewport `maximum-scale=1,
user-scalable=no`, `touch-action: manipulation` (bỏ phóng to bằng chạm đúp của trình duyệt; chạm đúp thả cảm xúc của app, §6 "Vuốt", vẫn hoạt động), cử chỉ chụm của iOS bị huỷ; ô nhập 16px trên màn cảm
ứng để không tự phóng khi focus. Đánh đổi có chủ đích với WCAG 1.4.4: chữ đã đủ lớn (chat 16px trên điện thoại), và nội
dung cần phóng (ảnh trong Trình xem tệp) có phóng riêng bằng chụm hai ngón.
Hàng cuộn ngang (tab, chip) phải cho thấy còn nội dung: dùng `ScrollFade` (mép còn nội dung mờ dần), không để một mục bị cắt cụt mà không có dấu hiệu. Popover không bao giờ sát mép màn: luôn cách mép viewport ≥ 16px.

## 10. Enforcement

`frontend/eslint.config.js` khoá ở `error`: cấm hex/rgba, màu palette Tailwind thô, cỡ chữ và
giá trị tuỳ ý (`text-[`, `z-[`, `tracking-[`), `transition-all`, `duration-<số>`, `animate-bounce`,
`<h1-6>`/`<p>` thô ngoài `Heading`/`Text`. Exemption phải có lý do trong comment.
