# Thiết kế: Ép design system lên toàn bộ UI

Ngày: 2026-08-01
Trạng thái: đã duyệt thiết kế, chờ lập kế hoạch thi công

## 1. Vấn đề

Design system đã tồn tại đầy đủ — token M3 trong `frontend/src/index.css`, ba tầng component
(`primitives` → `composites` → `patterns`), và Stitch làm nguồn thiết kế (`.stitch/DESIGN.md`,
`.stitch/metadata.json`). Vấn đề là **phần lớn màn hình không dùng nó**.

Đo trên `frontend/src` ngày 2026-08-01:

| Chỉ số | Tổng | Hợp lệ | Cần xử lý |
|---|---:|---:|---:|
| File `.tsx` (không tính test) | 107 | — | — |
| File có import `components/primitives` | 19 | — | — |
| `<button>` thô | 113 | 2 | 111 |
| `<input>` thô | 26 | 1 | 25 |
| `<select>` thô | 8 | 1 | 7 |
| `<textarea>` thô | 2 | 1 | 1 |
| Giá trị Tailwind tuỳ tiện `[Npx]` | 148 | 0 | 148 |
| Hex nướng cứng trong `.tsx` | 10 | 4 | 6 |
| Hex nướng cứng trong `.ts` | 57 | 48 | 9 |
| Class bảng màu Tailwind thô | 5 | 0 | 5 |
| `shadow-[…]` tuỳ tiện | 34 | 0 | 34 |
| `shadow-xl` (ngoài thang) | 4 | 0 | 4 |
| **Tổng** | **407** | **57** | **350** |

Cột "Hợp lệ" gồm hai nhóm khác hẳn nhau, xem 1.3.

Mọi con số ở đây đếm **lượt xuất hiện**, không đếm dòng. Phân biệt này không thừa: `routes/_auth.tsx`
có 2 hex trên cùng một dòng, và bản nháp đếm theo dòng đã ra 9 thay vì 10.

### 1.1 Nguyên nhân gốc, không phải sự cẩu thả

Hai phát hiện bác bỏ giả thuyết "lập trình viên lười":

**Các nút thô dùng token đúng.** Chúng viết `bg-primary`, `text-on-surface-variant` — chuẩn M3.
Thứ chúng tự chế là *hình học*: `py-2.5 px-4`, `py-3 px-4`, `py-[7px]`, `p-1.5`, `rounded-lg`,
`rounded-md`, `text-[13px]`. Vấn đề nằm ở kích thước, không ở màu.

**Primitive không khớp thực tế nên bị bỏ qua.**

```
Button size md  →  px-3 py-1  rounded-md
Button size lg  →  px-4 py-2  rounded-md
Màn hình thật   →  px-4 py-2.5 / py-3   rounded-lg
```

Không size nào của `Button` khớp cái các màn hình đang dùng. Thay thẳng `<button>` bằng `<Button>`
sẽ làm mọi nút co lại và đổi bo góc.

**Primitive thiếu từ vựng.** Phân loại 102 thẻ `<button>` bắt được `className`:

| Nhóm | Số lượng | Thuộc về |
|---|---:|---|
| Icon toggle (`p-1.5 rounded-md`, không chữ) | ~52 | `IconButton` |
| Dòng điều hướng / danh sách (`w-full flex items-center gap-2`) | ~16 | **chưa có primitive** |
| Nút hành động thật | ~15 | `Button` |

`IconButton` chỉ có dạng trong suốt — không có dạng viền mà Stitch quy định.
`Input` chỉ có dạng chữ nhật — không có dạng pill tìm kiếm mà Stitch quy định, đó là nguyên nhân
trực tiếp của 26 `<input>` thô.

Kết luận: **dọn mà không mở rộng từ vựng primitive thì nợ sẽ mọc lại.**

### 1.2 Không có gì ngăn nợ quay lại

`frontend/eslint.config.js` khai báo `files: ['**/*.{js,jsx}']`. Toàn bộ 107 file `.tsx`
**không được lint**. `npm run lint` chạy xanh mà không soi một dòng nào của ứng dụng.

Frontend có đúng **2 file test** (`components/CreateChannelModal.test.tsx`, `api/messaging.test.ts`).

### 1.3 Hai nhóm "vi phạm" thực ra hợp lệ

Không phải mọi con số đếm được đều là nợ. 57 lượt phải được miễn trừ vĩnh viễn, vì ép chúng vào
design system là sai chứ không phải đúng:

**Nhóm A — thẻ thô bên trong `components/primitives/` (5 lượt).** Primitive *phải* render thẻ HTML
thật. Đây là nơi định nghĩa, không phải nơi vi phạm. Rule lint có override cho đúng thư mục này.
Lưu ý: miễn trừ chỉ áp cho *thẻ thô*, không áp cho giá trị tuỳ tiện — `primitives/Textarea.tsx` có
1 giá trị `[Npx]` vẫn phải sửa.

**Nhóm B — màu là bản sắc nội dung, không phải màu giao diện (52 lượt).**

- `routes/_auth/login.tsx` — 4 hex `#4285F4 #EA4335 #FBBC05 #34A853` là **màu thương hiệu Google**
  trong logo SVG của nút "Sign in with Google". Logo có màu cố định; token hoá là làm sai logo.
- `lib/fileIcons.ts` — 48 hex gán màu cho **loại tệp** (PDF đỏ, Excel xanh lá, JS vàng). Đây là từ
  vựng nhận dạng nội dung, không phải chrome giao diện. Bảng M3 không có "xanh bảng tính", và ép
  chúng vào token surface sẽ phá mất chức năng phân biệt loại tệp bằng màu.

Ngược lại, `lib/constants.ts` (9 hex trong `ASSET_STATE_COLORS` và `REQUEST_STATUS_COLORS`)
**là vi phạm thật**: đó là màu *trạng thái*, mà `index.css` đã có sẵn `--color-success`,
`--color-warning`, `--color-danger`, `--color-info` cho đúng mục đích đó.

## 2. Phạm vi

**Trong phạm vi:** đưa token về khớp Stitch; hiệu chỉnh và mở rộng từ vựng primitive; bật ESLint
cho `.ts` và `.tsx` với rule ép design system; xử lý 350 vi phạm theo từng chặng; cập nhật ba
capability spec bị chạm tới.

**Ngoài phạm vi, có chủ đích:**

- Chuẩn hoá backend — đã đo và thấy gần như đã chuẩn: `pkg/httputil.MapDomainError` dùng ở 8/9 file
  REST; 4 chỗ trả error thô đều là deprecation stub cố ý trong `services/document`;
  `services/workspace/internal/rest/admin_handler.go` là file duy nhất chưa dùng `httputil`.
- Gỡ rối tầng data frontend (`useMessaging.ts` 15.5K, `websocket.store.ts` 16.9K, route 400–473 dòng).
- Thiết kế lại bố cục màn hình.
- Bật bộ rule TypeScript tổng quát (`tseslint.configs.recommended`) — xem 5.1.
- Sửa các sai lệch có sẵn giữa ba capability spec và code (sidebar 280px không có nút thu gọn,
  Drive thiếu breadcrumb) — xem 7.2.
- Dark mode: `metadata.json` khai `"colorMode": "LIGHT"`, code có 0 lượt dùng `dark:`. Không làm.

## 3. Quyết định đã chốt

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Mục tiêu là ép design system lên UI, không phải ba việc song song | Backend đã chuẩn ~90%; nợ đo được tập trung ở UI |
| D2 | Stitch là chuẩn; màn hình được phép đổi diện mạo | Nhất quán với `CLAUDE.md` §3 và `.stitch/WORKFLOW.md` |
| D3 | `--color-primary` đổi `#2563EB` → `#004AC6` | `metadata.json` khai `"customColor": "#004ac6"` |
| D4 | Bật ESLint cho `.ts`/`.tsx` kèm rule chặn, vi phạm = lỗi | Không có ràng buộc thì nợ mọc lại |
| D5 | Dùng `DESIGN.md` + `metadata.json` làm nguồn, không cần HTML gốc | Xem 6.1 |
| D6 | Giữ nguyên số đo pixel trong `components/drive`, kèm chú thích miễn trừ | Xem 4.4 |
| D7 | Ánh xạ bo góc theo **pixel** Stitch nêu, không theo tên class | Xem 4.1 |
| D8 | Giữ `focus-ring` thay vì `focus:ring-2 ring-primary/10` của Stitch | Accessibility; sai lệch có chủ đích, ghi rõ |
| D9 | Màu trạng thái dùng token `--color-status-*`, không dùng Tailwind thô như Stitch ghi | 8/11 chỗ đã dùng token; WORKFLOW.md tự cấm class tuỳ tiện. Xem 4.1 |
| D10 | Miễn trừ vĩnh viễn cho màu bản sắc nội dung (logo Google, màu loại tệp) | Token hoá chúng là làm sai chức năng. Xem 1.3-B |

## 4. Thiết kế

### 4.1 Tầng token

Toàn cục, không chia lát được, làm dứt điểm một đợt.

**Màu:**

```
--color-primary:        #2563EB → #004AC6   (Royal Blue, brand anchor)
--color-primary-hover:  #1d4ed8 → #003EA8   (Deep Navy)
--color-primary-container: giữ #2563eb      (đúng vai theo Stitch)
+ --color-on-primary-fixed-variant: #003EA8 (Stitch quy định, index.css đang thiếu)
```

**Bo góc — giữ nguyên thang token, sửa chỗ dùng.** Thang hiện tại là `sm 4 / md 8 / lg 12 / xl 16`,
lệch một nấc so với thang mặc định của Tailwind mà Stitch viết theo. Stitch nêu ý định bằng pixel
("gently rounded (8px)", "generous 12px corners"), nên ta tôn trọng con số:

| Stitch viết | Ý định | Class đúng trong repo |
|---|---|---|
| nút `rounded-lg` | 8px | `rounded-md` |
| card `rounded-xl` | 12px | `rounded-lg` |
| CTA `rounded-xl` | 12px | `rounded-lg` |
| tìm kiếm `rounded-full` | pill | `rounded-full` |

Đổi thang token thay vì đổi chỗ dùng sẽ làm xê dịch mọi góc bo trong app mà không ai yêu cầu.

**Bóng đổ:** thêm `--shadow-card: 0 4px 16px rgba(0,0,0,0.04)` — con số Stitch quy định cho card,
thang hiện tại không có (`shadow-sm` là `0 1px 3px/0.08`, `shadow-md` là `0 4px 12px/0.1`).

Thêm ba token bóng nhấn, để 5 chỗ đang nướng cứng mã RGB của primary cũ có đích để chuyển sang mà
không phải viết giá trị tuỳ tiện:

```css
--shadow-accent-sm: 0 2px 12px -2px color-mix(in srgb, var(--color-primary) 20%, transparent);
--shadow-accent:    0 4px 16px      color-mix(in srgb, var(--color-primary) 20%, transparent);
--shadow-accent-lg: 0 8px 24px      color-mix(in srgb, var(--color-primary) 8%,  transparent);
```

Không có ba token này, việc sửa `rgba(37,99,235,…)` chỉ đổi một vi phạm lấy một vi phạm khác:
`shadow-[0_4px_16px_color-mix(…)]` hết hardcode màu nhưng vẫn là giá trị tuỳ tiện.

**Ba khoản nợ ẩn phải dọn cùng đợt, nếu không đổi màu sẽ vỡ:**

1. **8 chỗ hardcode `rgba(37, 99, 235, …)`** — mã RGB của primary cũ. Đổi `--color-primary` sẽ
   không động tới chúng → app có hai sắc xanh. Thay bằng `color-mix()` trên token.
2. **`shadow-[0_0_0_2px_rgba(var(--md-primary-rgb),0.15)]`** tại
   `components/patterns/ImagePreviewCard.tsx:72` tham chiếu `--md-primary-rgb` **không tồn tại**
   trong `index.css` → shadow này hiện không render. Bug sẵn có, sửa ở chặng 2 cùng file.
3. **34 giá trị `shadow-[…]` tuỳ tiện** trải trên 14 file, cộng `shadow-xl` (4 lượt, không có trong
   thang → rơi về mặc định Tailwind). Quy về thang token, đi theo từng chặng sở hữu file.

   *Đính chính so với bản nháp:* ba khoản này ban đầu được xếp "phải dọn cùng lúc ở chặng 0". Sai —
   chỉ 5 shadow có nhuốm màu primary cũ là gấp, vì chúng vỡ khi đổi token. 29 shadow còn lại là bóng
   đen trung tính, không liên quan gì tới việc đổi màu, nên đi theo chặng của file chứa chúng.

*Ghi chú:* `on-primary-fixed-variant` ban đầu bị nghi là bug đang chạy. Kiểm tra kỹ: nó chỉ xuất
hiện trong **comment** của `components/patterns/ContactsSidebar.tsx` (dòng 22 và 129), không phải
class thật. Không phải lỗi runtime — là tài liệu lệch code, và là dấu hiệu token này lẽ ra phải có.

**Màu trạng thái: một mâu thuẫn trong chính Stitch, giải quyết nghiêng về token.**

`.stitch/DESIGN.md` §2 ghi "Status Colors (Semantic — **NOT** from design tokens)" và kê màu
Tailwind thô (`bg-green-50 text-green-700`, `bg-slate-100`, `bg-orange-50`). Nhưng `index.css` đã
định nghĩa `--color-status-online / -offline / -meeting` kèm biến thể `-bg` và `-border`, và **8 chỗ
trong code đã dùng token đó**. Chỉ còn 3 chỗ dùng màu thô (`components/patterns/ChatListItem.tsx:54`,
`components/chat/ChannelInfoPanel.tsx:125,197`).

Quyết định: **dùng token, sửa 3 chỗ còn lại**. Lý do — codebase đã chọn token ở đa số (8 trên 11),
`.stitch/WORKFLOW.md` §Anti-Patterns tự cấm "Using arbitrary Tailwind classes", và dòng "NOT from
design tokens" là dấu vết của quy trình sinh màn hình bên Stitch chứ không phải một quyết định
thiết kế. Đây là sai lệch có chủ đích thứ hai khỏi văn bản Stitch, cùng loại với D8.

### 4.2 Từ vựng primitive

**`Button` — 7 variant còn 4:**

| Variant | Lượt dùng | Quyết định | Căn cứ |
|---|---:|---|---|
| `primary` | 12 | giữ | Stitch §Buttons |
| `secondary` | 6 | giữ, **gộp `outline` (4) vào** | Stitch định nghĩa "Secondary/**Outlined**" là một |
| `ghost` | 27 | giữ | mở rộng ngoài Stitch, dùng nhiều nhất |
| `danger` | 3 | giữ, **gộp `error` (3) vào** | hai variant cùng chức năng, Stitch chỉ có một Signal Red |
| `success` | 0 | **xoá** | không ai dùng, không có trong Stitch |

**Size** (đếm riêng cho `<Button>` bằng parser nhận biết thẻ nhiều dòng: 41 mặc định + 1 `md`,
22 `sm`, 0 `lg`):

```
md   px-4 py-2   rounded-md  text-body-strong    ← Stitch: py-2 px-4, 8px, 14/20 w600
sm   px-3 py-1.5 rounded-md  text-small-ui       ← mở rộng của dự án, KHÔNG có trong Stitch
cta  w-full py-3 rounded-lg  text-body-strong    ← Stitch: Full-Width CTA, 12px
lg   xoá                                          ← 0 lượt dùng trên Button
```

`sm` được giữ vì 22 chỗ đang cần thật, và được ghi rõ là phần mở rộng của dự án — không giả vờ nó
đến từ Stitch.

**`IconButton`:** thêm `variant: 'ghost' | 'outlined'`, mặc định `ghost` để 18 chỗ hiện tại không
đổi. `outlined` = `p-2 rounded-md border border-outline-variant` theo Stitch §Buttons.

**`Input`:** thêm `variant: 'default' | 'search'`. `search` = `rounded-full bg-surface-container-low`
theo Stitch §Inputs — mảnh từ vựng thiếu gây ra 26 `<input>` thô.

Về focus: Stitch quy định `focus:border-primary focus:ring-2 focus:ring-primary/10` (vòng sáng 10%
alpha). Code đang dùng utility `focus-ring` (2px nền + 4px primary đặc), **dễ nhìn hơn hẳn và tốt
hơn cho accessibility**. Giữ `focus-ring` — sai lệch có chủ đích khỏi Stitch, lý do là a11y (D8).

**`NavRow` — primitive mới**, cho ~16 dòng điều hướng đang giả dạng nút. Stitch §Navigation Sidebar
quy định đủ trạng thái:

```
group header active  bg-surface-container text-primary font-medium
sub-item active      bg-primary-fixed text-on-primary-fixed-variant
inactive             text-on-surface-variant hover:bg-surface-container-low
thụt lề sub-item     pl-9 pr-2
```

Đây là lý do token `on-primary-fixed-variant` được thêm ở 4.1 — không có nó thì trạng thái active
của sub-item không diễn đạt được.

### 4.3 Tầng ràng buộc

**Chỉ thêm parser, không thêm bộ rule tổng.** `typescript-eslint` chưa được cài. Bốn rule cần dùng
đều thuần cú pháp JSX, không cần type-checking. Bật `tseslint.configs.recommended` sẽ đổ ra hàng
trăm lỗi unused vars / `any` / import order — chôn vùi tín hiệu cần thiết. Giữ như thiết kế thì
**mọi lỗi lint xuất hiện đều đúng là vi phạm design system**.

Bốn rule dùng `no-restricted-syntax` (esquery selector, có sẵn trong ESLint lõi, không cần plugin):

```js
// 1. Thẻ tương tác thô — override cho phép trong components/primitives/**
{ selector: "JSXOpeningElement[name.name=/^(button|input|textarea|select)$/]" }

// 2. Giá trị pixel tuỳ tiện — cả literal lẫn template string
{ selector: "JSXAttribute[name.name='className'] Literal[value=/\\[\\d+(px|rem)\\]/]" }
{ selector: "JSXAttribute[name.name='className'] TemplateElement[value.raw=/\\[\\d+(px|rem)\\]/]" }

// 3. Hex nướng cứng — MỌI string literal, không chỉ trong className (xem ghi chú dưới)
{ selector: "Literal[value=/#[0-9a-fA-F]{6}/]" }

// 4. Bảng màu Tailwind thô — đã có token M3 thay thế
{ selector: "JSXAttribute[name.name='className'] Literal[value=/\\b(bg|text|border)-(slate|gray|zinc|neutral|red|orange|green|blue|indigo)-\\d{2,3}\\b/]" }

// 5. Bóng đổ ngoài thang token
{ selector: "JSXAttribute[name.name='className'] Literal[value=/shadow-(\\[|xl\\b)/]" }
{ selector: "JSXAttribute[name.name='className'] TemplateElement[value.raw=/shadow-(\\[|xl\\b)/]" }
```

**Vì sao phải có rule 5 — một lỗ hổng do bước tự soát kế hoạch phát hiện.** Rule 2 đòi toàn bộ cụm
trong ngoặc phải là `\d+(px|rem)`, nên `shadow-[0_4px_16px_rgba(0,0,0,0.04)]` **lọt qua**, dù bên
trong có `4px`. Kiểm chứng:

```
shadow-[0_4px_16px_rgba(0,0,0,0.04)]   → lọt
px-[7px]                                → bị chặn
grid-cols-[auto_1.5fr]                  → lọt   (đúng ý đồ, xem dưới)
```

Không có rule 5 thì 34 shadow tuỳ tiện + 4 `shadow-xl` **không bao giờ xuất hiện trên thước đo**, và
mục 5.6 sẽ báo "0 vi phạm" trong khi chúng còn nguyên vẹn. Một thước đo không nhìn thấy thứ nó phải
đo thì tệ hơn không có thước đo, vì nó tạo ra niềm tin sai.

**Phạm vi file:** `**/*.{ts,tsx}`, không chỉ `.tsx`. Bản nháp đầu của tài liệu này giới hạn rule
hex trong `className` và chỉ quét `.tsx` — cả hai đều sai, và tự soát lại mới phát hiện:

- 4 trong 6 hex thật ở `.tsx` nằm ngoài `className` — ở prop `color="#3b82f6"` / `color="#f59e0b"`
  và trong biểu thức `ASSET_STATE_COLORS[...] || '#6b7280'` → rule giới hạn theo `className` chỉ
  bắt được 2 chỗ trong `routes/_auth.tsx`
- 57 hex nằm trong `.ts` (`lib/constants.ts`, `lib/fileIcons.ts`) → giới hạn ở `.tsx` bỏ sót toàn bộ

Nên rule 3 quét **mọi string literal** trong `.ts` và `.tsx`, kèm hai override cho nhóm hợp lệ ở
1.3: `lib/fileIcons.ts` (màu loại tệp) và khối SVG logo Google trong `routes/_auth/login.tsx`.
Cả hai override phải có chú thích nêu lý do.

Rule 2 cố ý **chỉ chặn số đo pixel/rem**, không chặn `[...]` nói chung — vì chính `DESIGN.md` quy
định lưới bảng bằng `grid-cols-[auto_1.5fr_1fr_1.5fr_1fr]`. Chặn cả cụm sẽ biến source of truth
thành vi phạm.

Rule 1 bao cả `select` và `textarea` — repo có 8 `<select>` và 2 `<textarea>` thô, và primitive
`Select`/`Textarea` đã tồn tại sẵn nhưng không ai dùng.

**Số vi phạm lint là thước đo tiến độ.** Bật ở mức `warn` trước khi di cư, khoá `error` sau khi về
0. Con số giảm dần theo từng chặng, chạy một lệnh ra ngay, không phụ thuộc vào lời khai của người
thực hiện. Đây là thứ thay thế cho lưới an toàn mà 2 test không cung cấp được.

### 4.4 Trình tự di cư

Xếp theo phụ thuộc, không theo kích thước.

| Chặng | Phạm vi | Vi phạm | Lý do vị trí |
|---|---|---:|---|
| 0 | token + primitive + lint mức `warn` | 6 | Nền móng (4.1–4.3). Gồm 1 `[Npx]` trong `primitives/Textarea.tsx` và 5 shadow nhuốm màu primary cũ, nằm rải ở file của chặng 1/2/5 nhưng thuộc về đợt đổi token |
| 1 | `routes/_auth/` + `routes/_auth.tsx` + `components/auth` + `components/layouts` | 39 | **Thí điểm.** Biệt lập, không màn hình nào phụ thuộc. Primitive sai thì lộ ở đây với chi phí thấp nhất |
| 2 | `components/composites` → `components/patterns` | 82 | Bộ khung, bán kính ảnh hưởng lớn nhất. Chỉ làm sau khi chặng 1 chứng minh primitive đúng. Gồm bug `--md-primary-rgb` |
| 3 | `routes/_workspace/` + `routes/_workspace.tsx` | 73 | Route vỏ, phụ thuộc `patterns` |
| 4 | `components/drive` | 70 | Xem cảnh báo bên dưới |
| 5 | `components/chat` | 36 | Độc lập với drive |
| 6 | `components/approval` + `routes/assets/` + `routes/assets.tsx` + `components/` gốc + `lib/constants.ts` | 44 | Phần đuôi. `lib/constants.ts` đi cùng vì `assets` là nơi tiêu thụ nó |
| 7 | khoá lint `warn` → `error` | 0 | Chỉ khả thi khi đã về 0 |

Đối chiếu: 6 + 39 + 82 + 73 + 70 + 36 + 44 = **350** = tổng 407 trừ 57 hợp lệ (1.3).

Vi phạm về màu (15 hex thật + 5 class màu thô) không tách thành chặng riêng — chúng đi theo file
chứa chúng, nên đã nằm sẵn trong các con số trên.

**Cảnh báo chặng 4.** `components/drive` chứa 45 giá trị pixel tuỳ tiện — tập trung nhất toàn repo,
nằm trong hàng bảng dữ liệu. Đó là chiều cao hàng và bề rộng cột đã căn chỉnh có chủ ý cho mật độ.
`DESIGN.md` chỉ quy định lưới bảng cho màn hình contacts, không có drive; HTML gốc của
`drive-my-files` không tải được (6.1). `docs/specs/lark-data-table-layout/spec.md` đã được đọc và
**không giải quyết được** — nó tả hành vi (không đổ bóng khi hover, có breadcrumb, peek panel bên
phải), không tả số đo.

Theo D6: **giữ nguyên các số đo đó**, kèm chú thích `eslint-disable-next-line` có nêu lý do và dẫn
chiếu mục này. Không đoán, không xoá.

Nhân tiện: `lark-data-table-layout` cấm hàng bảng đổ bóng khi hover, mà repo có 34 shadow tuỳ tiện
trải trên 14 file. Ở chặng 4 sẽ **đối chiếu và báo cáo** nếu có vi phạm — không tự tiện sửa, vì đó
ngoài phạm vi.

**Nghiệm thu mỗi chặng** (một commit, phải thoả cả bốn):

1. `npm run lint` — số vi phạm giảm **đúng bằng** phần của chặng, **không tăng ở bề mặt khác**
2. `npm run build` xanh
3. `npm test` xanh
4. Xem mắt ở 375 / 768 / 1280px theo `.stitch/WORKFLOW.md` Bước 6

Điều kiện 1 bác bỏ được kiểu "dọn chỗ này làm bẩn chỗ kia".

## 5. Nghiệm thu tổng

### 5.1 TDD cho primitive

`CLAUDE.md` §4 quy định "All code: `superpowers:test-driven-development` — failing test first".
Sửa primitive là viết code, nên test đỏ trước.

Primitive là chỗ đáng test nhất và dễ test nhất frontend: vào là props, ra là class, không phụ
thuộc server hay router. Test ghim đúng hình học Stitch quy định:

```
Button variant=primary size=md  → có 'px-4 py-2' 'rounded-md' 'text-body-strong'
Button size=cta                 → có 'w-full py-3 rounded-lg'
IconButton variant=outlined     → có 'border border-outline-variant'
Input variant=search            → có 'rounded-full bg-surface-container-low'
NavRow active kind=subItem      → có 'bg-primary-fixed text-on-primary-fixed-variant'
```

Đây là lần đầu hình học Stitch được ghim bằng thứ chạy được thay vì văn xuôi.

### 5.2 TypeScript bắt hộ phần gộp variant — nhưng phải dựng lại trình biên dịch trước

Xoá một nhánh khỏi union type biến mọi call site cũ thành **lỗi biên dịch**:

```
gộp outline → secondary  → 4 chỗ
gộp error   → danger     → 1 chỗ literal + 1 chỗ lan qua prop = 2
đổi success sang nền đặc → 4 chỗ tự chế bằng className (TypeScript KHÔNG bắt được)
xoá size lg              → 0 chỗ
```

Trình biên dịch liệt kê **6** vị trí.

Hai lần đính chính, và lần thứ hai mới là bài học:

- Bản nháp đầu ghi 7, vì đếm cả `AlertBanner variant="error"` — `AlertBanner` có kiểu `AlertVariant`
  riêng, không liên quan `Button`. Sửa xuống 5.
- Con số 5 **cũng sai**, phát hiện lúc thi công. `components/composites/ConfirmDialog.tsx:112` viết
  `<Button variant={confirmVariant} …>`, trong đó `confirmVariant?: 'primary' | 'error'` là prop
  công khai của chính `ConfirmDialog`, và có **4 nơi bên ngoài** truyền `confirmVariant="error"`
  (`routes/_workspace/drive.tsx`, `components/drive/ShareDialog.tsx`,
  `components/drive/DeleteConfirmDialog.tsx`, `components/chat/ChannelInfoPanel.tsx`).

**Vì sao khảo sát bỏ sót:** nó grep chuỗi literal `variant="error"`. Một giá trị đi qua **biến** thì
vô hình với grep — nó chỉ lộ ra ở tầng kiểu. Đây chính là lý do mục này tồn tại: trình biên dịch
tìm được thứ mà khảo sát chuỗi không thể. Nhưng nó chỉ tìm được **nếu có trình biên dịch chạy được**,
mà điều đó lại phải dựng riêng — xem cảnh báo bên dưới.

**Xử lý:** `ConfirmDialog` đổi luôn prop công khai sang `'primary' | 'danger'` thay vì ánh xạ nội bộ
`error → danger`. Ánh xạ nội bộ giữ lại hai tên gọi cho một khái niệm trong cùng codebase — đúng loại
trôi dạt mà tài liệu này tồn tại để xoá — và biểu thức ba ngôi sẽ âm thầm quy mọi variant tương lai
về `primary`. Giá phải trả là 3 file ngoài danh sách ban đầu, mỗi file đổi đúng một từ, do trình biên
dịch chỉ đích danh.

**Cảnh báo phải nêu, vì nó làm mục này gần như sai:** dự án **chưa từng được typecheck**.
`typescript` không có trong `package.json`; bản duy nhất trong `node_modules` là **3.9.10**, kéo vào
bắc cầu qua `@protobuf-ts/plugin`. Bản 2020 đó không hiểu `jsx: "react-jsx"` lẫn
`noUncheckedIndexedAccess` mà `tsconfig.json` khai, nên `npx tsc` chỉ nôn ra lỗi cú pháp và không
kiểm được gì.

Chạy TypeScript 5.9 thật lên codebase cho **123 lỗi có sẵn**. Nên lưới an toàn ở mục này **không tồn
tại cho tới khi được dựng**, và khi dựng xong nó cũng không thể là cổng "0 lỗi" — 123 lỗi cũ sẽ chôn
5 lỗi cần tìm, đúng cái bẫy mà 4.3 đã tránh khi từ chối bộ rule TS tổng.

Cách dùng đúng: cài `typescript@5` làm devDependency thật, ghi lại baseline 123, và mỗi task so theo
**từng file** thay vì theo tổng số. Trong số file mà kế hoạch chạm, chỉ 7 file có lỗi sẵn (11 lỗi):
`routes/_workspace/drive.tsx` 3, `routes/_workspace/approval.tsx` 2,
`routes/_auth/workspace-select.tsx` 2, và bốn file khác mỗi file 1. Phần còn lại sạch, nên lỗi mới
xuất hiện ở chúng là tín hiệu thật.

### 5.3 Cạm bẫy: chặng 0 đổi giao diện mà diff không thể hiện

Chặng 0 chỉ sửa token và primitive. Nhưng **42 chỗ đang dùng `<Button>`** sẽ đổi hình dạng
(`px-3 py-1` → `px-4 py-2`) và **toàn app đổi màu xanh** — trong khi *không file `.tsx` nào nằm
trong diff*. Người review diff sẽ thấy vài dòng CSS và một file primitive rồi kết luận "thay đổi
nhỏ". Thực tế đây là cú đổi diện mạo lớn nhất của cả dự án.

Chặng 0 **bắt buộc** có ảnh chụp trước/sau ở 375 / 768 / 1280px. Verify bằng đọc diff là vô hiệu.

Phép kiểm tra dứt khoát cho màu: sau chặng 0, `grep -rn '37,\s*99,\s*235' frontend/src/` phải
**bằng 0**. Còn sót là app có hai sắc xanh.

### 5.4 Cập nhật capability spec

Migration chạm ba capability đã có spec:

| Spec | Chặng | Trạng thái hiện tại |
|---|---|---|
| `lark-sidebar-layout` | 2 | *Diverges* — 280px, không có nút thu gọn |
| `lark-data-table-layout` | 4 | *Partially implemented* — Drive thiếu breadcrumb |
| `lark-messaging-layout` | 5 | Một phần không kiểm chứng được bằng đọc code |

`docs/specs/README.md` quy định: "*silently editing a spec to match the code destroys the only
record that a decision was ever made*". Nên ta **ghi thêm** thay đổi do refactor này vào phần
`## Status`, **không xoá** ghi chú sai lệch có sẵn. Việc sửa các sai lệch đó là công việc khác (7.2).

### 5.5 Điều không kiểm chứng được

Không có visual regression baseline.

- "Không có gì hỏng" — kiểm chứng được: lint + build + test + mắt người ở 3 breakpoint.
- "Trông đẹp hơn" — **không** kiểm chứng được. Đó là phán đoán thẩm mỹ của người dùng, không phải
  kết quả test. Sẽ không tuyên bố.

Vì giao diện **được phép đổi** theo D2, ảnh trước/sau là để *nhìn thấy cái gì đã đổi*, không phải
để chứng minh không đổi.

### 5.6 Định nghĩa "xong"

1. `npm run lint` = 0 vi phạm, rule ở mức `error`. Miễn trừ duy nhất được chấp nhận là ba nhóm đã
   nêu tên: thẻ thô trong `components/primitives/` (1.3-A), màu bản sắc nội dung (1.3-B), và số đo
   pixel trong `components/drive` (D6). Mỗi miễn trừ phải có chú thích nêu lý do và dẫn chiếu mục
   tương ứng của tài liệu này
2. `npm run build` xanh; `npm test` xanh, bao gồm test primitive mới
3. `grep -rn '37,\s*99,\s*235' frontend/src/` = 0
4. Ảnh 3 breakpoint cho mỗi chặng
5. Ba capability spec đã cập nhật `## Status`
6. Không còn `<button>`/`<input>`/`<textarea>`/`<select>` thô ngoài `components/primitives/`
7. `lib/constants.ts` không còn hex — màu trạng thái đọc từ token
8. 3 chỗ dùng màu trạng thái Tailwind thô đã chuyển sang token `--color-status-*`

## 6. Giả định

### 6.1 `DESIGN.md` + `metadata.json` là nguồn đủ dùng

`.stitch/WORKFLOW.md` Bước 2 yêu cầu tải HTML gốc từng màn hình về `.stitch/designs/` qua
`mcp_stitch_get_screen`. **Không thực hiện được:**

- `.stitch/designs/` không tồn tại — HTML gốc chưa từng được tải
- Stitch MCP không được cấu hình: `.mcp.json` chỉ có `serena` và `codegraph`
- `.stitch/scripts/fetch-stitch.sh` chỉ là wrapper `curl`; nó cần `downloadUrl` mà chỉ MCP sinh ra
  được, và loại URL đó hết hạn

**Đánh giá:** không chặn ở phạm vi này. Ta chuẩn hoá token + primitive, không dựng lại bố cục. Mọi
thứ primitive cần — màu, bo góc, padding nút, radius card, kiểu chữ, hình dạng ô tìm kiếm — đều
được `DESIGN.md` và `metadata.json` quy định bằng con số. `metadata.json` còn xác nhận độc lập hai
quyết định quan trọng nhất bằng dữ liệu máy đọc được:

```json
"customColor": "#004ac6",    // xác nhận D3
"roundness": "ROUND_EIGHT",  // xác nhận D7 (nút 8px)
"font": "MANROPE",
"colorMode": "LIGHT"         // xác nhận không cần dark mode
```

**Quy tắc khi thi công:** chỗ nào di cư mà đụng quyết định bố cục không có trong hai nguồn trên →
**dừng và báo**, không đoán. Đúng tinh thần "không viết UI từ trí nhớ" của WORKFLOW.md.

### 6.2 `typescript-eslint` có bản tương thích ESLint 10

Repo dùng ESLint `^10.2.1`. Phải xác nhận thật lúc thi công, không giả định. Nếu chưa có bản tương
thích, phương án dự phòng là dùng thẳng `@typescript-eslint/parser` không qua gói meta — bốn rule
đều nằm trong ESLint lõi nên chỉ cần parser.

## 7. Rủi ro

### 7.1 Giai đoạn nửa vời giữa các chặng

Sau chặng 0, màn hình đã di cư và chưa di cư sẽ trông hơi lệch nhau cho tới chặng 6. Đây là cái giá
có ý thức của việc chia nhỏ để review được, thay vì một codemod khổng lồ không ai kiểm được.

### 7.2 Cám dỗ mở rộng phạm vi

Ba capability spec đều đang ghi nhận sai lệch với code. Khi sửa `AppSidebar` ở chặng 2 sẽ rất dễ
"tiện tay" thêm nút thu gọn cho đúng `lark-sidebar-layout`. **Không làm.** Đó là thay đổi hành vi,
cần spec và test riêng theo Enforcement Rules của `CLAUDE.md`, và nó không nằm trong cái đã chọn.

### 7.5 `className` truyền vào primitive KHÔNG ghi đè được class cùng thuộc tính

Đây là phát hiện đắt giá nhất của chặng thí điểm, và nó áp cho mọi lần di cư còn lại.

Khi thử đưa ô OTP (`OtpInput.tsx`) vào `Input`, bo góc **âm thầm** đổi từ 12px xuống 8px:
`rounded-md` nằm sẵn trong `variantStyles.default` của `Input`, `rounded-lg` truyền qua `className`.
Hai class **hoà nhau về độ đặc hiệu**, nên Tailwind phân giải theo **thứ tự trong stylesheet sinh ra**,
không theo thứ tự trong chuỗi JSX. Class viết sau không thắng.

Điều nguy hiểm hơn kết quả: chiều rộng (`w-12` với `w-full`) và màu focus **sống sót** trong cùng
phép thử — nhưng sống sót do may, cùng cơ chế đã giết bo góc. Một lần thử thấy "trông ổn" không
chứng minh được gì về lần sau.

**Không có tín hiệu nào ở thời điểm biên dịch.** TypeScript không thấy, ESLint không thấy, test
khẳng định class có mặt trong chuỗi thì vẫn xanh — vì class *có* mặt thật, nó chỉ không thắng.
Ảnh chụp so sánh cũng nhiều khả năng cho qua: 12px với 8px trên một ô 56px gần như không phân biệt
được bằng mắt. Thứ duy nhất bắt được là **đo computed style trên trình duyệt**.

**Quy tắc cho các chặng sau:** trước khi thay thẻ thô bằng primitive, phân loại nó đã:

- **Trường thông thường viết tay** → dùng primitive, đúng ý đồ.
- **Control có hình học cố định, thẻ bọc riêng, hoặc xử lý focus/thị giác riêng nướng trong
  `className`** → ép qua primitive sẽ đánh nhau trên nhiều trục, và có thể mất âm thầm một thuộc
  tính vào tay thứ tự cascade. Nếu nó không có logic nghiệp vụ, cách rẻ và trung thực hơn là
  **chuyển nó vào `primitives/`**, chứ không phải bọc nó lại.

`OtpInput` đi theo hướng thứ hai: rename thuần, không đổi một byte markup nào. Nó vốn đã đúng, chỉ
nằm sai thư mục — `components/auth/` là nơi nó tình cờ được cần đến lần đầu, không phải nơi nó thuộc về.

**Cùng cơ chế đó đã có sẵn trong `Button`, và test của chính dự án này không thấy.** Chuỗi base của
`Button` chứa `border-none`, trong khi variant `secondary` khai `border border-outline-variant`. Hai
class hoà nhau, `.border-none` được sinh ra sau `.border` trong stylesheet nên luôn thắng — variant
mà Stitch gọi là "Secondary/Outlined" **render không có viền ở mọi chỗ dùng**, đo được
`borderStyle: none`, `borderWidth: 0px`.

Test viết ở Task 3 khẳng định `expect(cls).toContain('border')` và **xanh suốt**, vì class có mặt
thật. Đó là giới hạn cấu trúc của kiểu test này: `vitest` đặt `css: false` nên nó chỉ thấy chuỗi,
không thấy hiệu lực.

Hai hệ quả cho các chặng sau:

1. Mỗi variant phải tự khai tình trạng viền của mình; không đặt thuộc tính có thể chọi ở chuỗi base.
2. Khi một phép kiểm chỉ khẳng định **sự hiện diện** của class, hãy thêm phép kiểm khẳng định
   **sự vắng mặt** của class chọi với nó. Đó là thứ duy nhất bắt được xung đột ở tầng chuỗi.

### 7.4 Bóng đổ: xấp xỉ là chấp nhận được, đã đo

Chặng thí điểm nêu nghi vấn rằng thang shadow thiếu một nấc cho loại "offset lớn, rất nhạt"
(`0 16px 32px -12px / .08` ở `welcome.tsx`), và lo rằng sẽ phải xấp xỉ lặp lại nhiều lần.

Kiểm kê 27 shadow tuỳ tiện còn lại bác bỏ điều đó — hình dạng ấy chỉ còn **đúng một lượt nữa**.
Bức tranh thật là **trôi dạt alpha quanh token đã có**, không phải thang thiếu nấc:

```
0 1px 2px / 1px 3px, alpha .04–.08   12 lượt  → sát --shadow-sm
0 2px 8px -2px / .05                  5 lượt  → cùng hình học --shadow-raised, chỉ khác alpha
0 4px 16px / .04                      2 lượt  → TRÙNG KHÍT --shadow-card
0 8px 30px / ...                      3 lượt  → sát --shadow-lg
```

Dựng hai bóng cạnh nhau ở đúng kích thước dùng thật (khối tròn 192px) rồi nhìn: chênh lệch nhỏ tới
mức gần như không phân biệt được. **Quyết định: dùng token gần nhất, không thêm nấc mới.** Thêm
token để phục vụ trôi dạt là làm từ vựng phình ra vì lý do ngược.

Hai lượt `0 4px 16px / .04` trùng khít `--shadow-card` là phần thưởng sẵn có ở chặng sau — thay
thẳng, không xấp xỉ gì cả.

### 7.3 Chú thích miễn trừ ở drive có thể bị lạm dụng

D6 mở một lối thoát khỏi rule lint. Ràng buộc: mỗi `eslint-disable` phải nêu lý do và dẫn chiếu
mục 4.4 của tài liệu này. Miễn trừ không lý do là vi phạm.
