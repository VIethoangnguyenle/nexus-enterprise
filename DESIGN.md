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
  **Tin nhắn · Tài liệu · Phê duyệt · Tài sản · Danh bạ**, đáy: **Quản trị** (khi có quyền),
  **Cài đặt**, người dùng hiện tại. Thu gọn thành rail 64px (icon + tooltip). Tài sản nằm trong
  shell này, không có layout riêng.
- **List panel** chỉ ở Tin nhắn và Tài liệu. **Detail panel** mở bằng chọn mục, đóng bằng Esc.
- **Breakpoints:** ≥ 1280 đủ 4 cột · 1024–1279 detail panel thành overlay bên phải ·
  768–1023 sidebar thành rail, list panel thành cột thay thế content · < 768 tab bar đáy 5 mục,
  mỗi màn một cột, detail panel thành sheet từ dưới lên.

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

**Realtime (bản sắc của hệ):**
- Thay đổi do **người khác** gây ra: avatar tác giả loé vòng `--color-person-N-wash` 900ms; hàng
  được phủ `--color-person-N-wash`, giữ 40% thời gian rồi tan, tổng **2400ms**; nhãn nhỏ "vừa
  gửi / vừa đổi tên / vừa duyệt" màu `--color-person-N` biến mất cùng lớp phủ.
- Thay đổi của **chính mình**: không phủ màu, chỉ animation chèn hàng.
- **Gộp:** ≥ 3 thay đổi trong cửa sổ 2000ms ở cùng một danh sách → một dòng "Vinh và 2 người khác
  vừa cập nhật", tối đa một lớp phủ mỗi hàng.
- **Presence:** cụm avatar người đang xem ở header (tối đa 3 + "+N"), vào/ra fade 220ms.
- **Kết nối:** bình thường không hiện gì. Mất kết nối > 3s → dải `warning-wash` dưới header
  "Đang kết nối lại…"; nối lại → "Đã cập nhật" 2s rồi ẩn.

**Giảm chuyển động** (`prefers-reduced-motion: reduce`): bỏ mọi `transform`; giữ fade ≤ 120ms;
lớp phủ realtime vẫn hiện (là thông tin) nhưng không loé avatar; layout animation tắt.

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
