# Ép Design System Lên UI — Chặng 2–7

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Đưa 271 vi phạm design system còn lại về 0 và khoá rule từ `warn` thành `error`, theo đúng quy trình mà chặng thí điểm đã chứng minh.

**Architecture:** Chặng 0 dựng nền (token, primitive, tầng lint). Chặng 1 là thí điểm trên bề mặt auth và đã tìm ra bốn lần cùng một lỗi ẩn, trong đó một lỗi nằm trong chính primitive của dự án. Chặng 2–6 lặp lại quy trình đó theo từng bề mặt, xếp theo phụ thuộc; chặng 7 khoá lint và cập nhật ba capability spec. Thước đo là số vi phạm lint, giảm đúng lượng ở mỗi chặng và không tăng ở đâu khác.

**Tech Stack:** React 19, Vite, Tailwind 4 (CSS-first `@theme`), TypeScript 5.9, Vitest + Testing Library, ESLint 10 flat config.

**Nguồn:** `docs/superpowers/specs/2026-08-01-ui-design-system-enforcement-design.md`, đặc biệt **§8 Biên bản chặng thí điểm** — kế hoạch này viết từ đó.

---

## Trạng thái khởi điểm

| Chỉ số | Giá trị |
|---|---|
| Vi phạm design system | **271** |
| Cảnh báo `Unused eslint-disable` (không thuộc 5 rule) | 1 → ESLint in ra **272** |
| Test | 42 xanh |
| Lỗi type baseline | 123 trên 52 file (`frontend/typecheck-baseline.txt`) |
| Nhánh | `refactor/ui-design-system` |

Phân bố theo chặng, đo bằng `npx eslint <thư mục>` trên từng phạm vi:

| Chặng | Phạm vi | Vi phạm |
|---|---|---:|
| 2 | `components/composites` + `components/patterns` | 66 |
| 3 | `routes/_workspace.tsx` + `routes/_workspace/` | 66 |
| 4 | `components/drive` | 66 |
| 5 | `components/chat` | 30 |
| 6 | `approval` + `assets` + `components/` gốc + `lib/` | 43 |
| 7 | khoá lint, cập nhật spec | 0 |

---

## Quy trình di cư — đọc một lần, áp cho mọi task

Mọi task từ Task 1 trở đi đều theo đúng sáu bước này. Chúng không được nhắc lại trong từng task.

**B1. Đọc lint, không tin số trong kế hoạch.**

```bash
cd frontend && npx eslint <đường-dẫn> > /tmp/t.txt 2>&1; cat /tmp/t.txt
```

Số trong kế hoạch này lấy từ một lần chạy lint ngày 2026-08-01. Nếu số thực khác, **số thực đúng** — ghi lại chênh lệch trong báo cáo. Ở chặng 1, kế hoạch dự đoán 39 còn thực tế là 31, vì `grep` của kế hoạch đếm cả các lượt nằm trong comment mà ESLint loại đúng.

**B2. Phân loại từng thẻ thô trước khi thay.**

- **(a) Control thông thường viết tay** — nút hành động, ô nhập bình thường, không có hình học cố định hay xử lý focus riêng → dùng primitive.
- **(b) Control có hình học cố định, phụ thuộc thẻ bọc, hoặc hover/focus riêng nướng trong `className`** → ép vào primitive sẽ đánh nhau nhiều trục và **có thể mất âm thầm một thuộc tính**. Nếu nó không có logic nghiệp vụ, chuyển file vào `components/primitives/` (rename thuần, không đổi markup). Nếu là trường hợp cá biệt, giữ nguyên kèm `eslint-disable-next-line` **có nêu lý do thật**.

**B3. Đo computed style trên trình duyệt — đo TRƯỚC khi quyết, không phải sau.**

Đây là quy tắc đắt nhất mà chặng 1 mua được. Bốn lần lỗi "class truyền qua `className` thua class nướng sẵn trong primitive" đều **chỉ** lộ ra khi đo; đọc code, chạy test, và nhìn ảnh chụp đều cho qua.

```bash
B="$HOME/.claude/skills/gstack/browse/dist/browse"
$B js "const s=getComputedStyle(document.querySelector('<selector>')); JSON.stringify({r:s.borderRadius,p:s.padding,w:s.width,b:s.borderStyle,f:s.fontSize})"
```

Tailwind phân giải hai class cùng thuộc tính theo **thứ tự trong stylesheet sinh ra**, không theo thứ tự trong chuỗi JSX. Class viết sau không thắng.

**B4. Không thêm hành vi chưa từng có.**

`Button size="cta"` kéo theo `w-full`. Nút gốc không full-width mà dùng `cta` là đẻ ra hành vi mới trên mobile. Chặng 1 đã suýt mắc ở `onboarding.tsx` và tránh được.

**B5. Ánh xạ giá trị.**

| Loại | Cách xử lý |
|---|---|
| `[Npx]` khoảng cách/kích thước | `N/4` trên thang 4px. **Xác minh từ CSS đã build**: `grep -o 'max-w-110{[^}]*}' dist/assets/*.css` phải cho `calc(var(--spacing) * 110)` với `--spacing: .25rem` |
| `shadow-[…]` | Token gần nhất trong thang, **không thêm token mới**. Xem §7.4 của spec — đã đo và chốt |
| `shadow-[0_4px_16px_rgba(0,0,0,0.04)]` | **Trùng khít `shadow-card`** — thay thẳng, không xấp xỉ |
| `shadow-xl` | Ngoài thang → token gần nhất |
| hex trong `className` / prop `color=` | Token màu. Tra giá trị trong `index.css` trước, có thể đã tồn tại |
| `bg-green-500` v.v. | Token `--color-status-*` hoặc token M3 |
| `blur-[Npx]` | Thang blur của Tailwind dừng ở `blur-3xl` = 64px. Trên mức đó **không có giá trị** → miễn trừ có lý do |

**B6. Nghiệm thu mỗi task.**

```bash
cd frontend && npx eslint <đường-dẫn> > /tmp/t.txt 2>&1; cat /tmp/t.txt     # 0 problems
cd frontend && npm run lint 2>&1 | tail -3                                  # giảm đúng lượng
cd frontend && npm run typecheck:diff && npm run build && npm test
```

Nêu rõ phép trừ. Nếu số của bạn và số của công cụ lệch nhau, **dừng và báo** thay vì chỉnh cho khớp.

**Ràng buộc chung cho mọi task:**

- Chỉ sửa file thuộc task. Không đụng primitive, `index.css`, `eslint.config.js` trừ khi task nói rõ.
- Không sửa 123 lỗi type có sẵn.
- Mọi `eslint-disable` phải nêu lý do thật, không được để trống.
- Không tạo dữ liệu trong DB dev.
- Không `git add -A`. Không stage `.gitignore` và ba file untracked có sẵn (`agent-setup-prompt.txt`, `docs/approve-flow.md`, `verify.py`).
- Hook `rtk` tóm tắt output một số lệnh — `npx eslint <file>` in ra bản rút gọn, nên redirect ra file rồi `cat` khi cần output thật. `npm run …` không bị ảnh hưởng.
- Dev server: **không** đụng cổng 5173 (của người dùng). Tự dựng instance riêng khi cần: `cd frontend && VITE_DEV_MODE=true npx vite --port 5180 --strictPort`. Thiếu `VITE_DEV_MODE=true` thì proxy trỏ sang Traefik không chạy và mọi lệnh gọi `/api/*` trả 404.

---

## Task 1: `Input` nhận prop `radius`

Việc treo duy nhất mà biên bản chặng thí điểm để lại cho khâu lập kế hoạch. Làm trước mọi chặng vì nó gỡ nguyên nhân của 3 trong 8 miễn trừ đã có và sẽ còn gặp lại.

**Bằng chứng.** Trong các `<input>` thô còn lại trên toàn repo: **5 dùng `rounded-lg` (12px), 2 dùng `rounded-md` (8px)**, 4 dùng `rounded-full` (ô tìm kiếm, đã có `variant="search"`). Primitive nướng cứng `rounded-md`. `.stitch/DESIGN.md` §Inputs **chỉ** quy định ô tìm kiếm và kiểu focus — im lặng về bo góc ô thường.

**Vì sao thêm prop chứ không đổi mặc định.** Khác `Button`, nơi *không* size nào khớp thực tế nên đổi primitive là đúng. Ở đây bằng chứng nghiêng 5–2 nhưng Stitch không phân xử, và đổi mặc định sẽ đổi diện mạo 19 chỗ đang dùng `<Input>` mà không ai yêu cầu. Prop `radius` biến một xung đột CSS thầm lặng thành một lựa chọn tường minh có kiểu.

**Files:**
- Modify: `frontend/src/components/primitives/Input.tsx`
- Modify: `frontend/src/components/primitives/Input.test.tsx`

- [ ] **Step 1: Viết test đỏ**

Thêm vào cuối `frontend/src/components/primitives/Input.test.tsx`:

```tsx
describe('Input — bo góc chọn được', () => {
  it('mặc định là md (8px), giữ nguyên cho 19 chỗ đang dùng', () => {
    render(<Input aria-label="Tên" />)
    expect(screen.getByRole('textbox').className).toContain('rounded-md')
  })

  it('radius="lg" cho 12px — 5 ô nhập thật cần giá trị này', () => {
    render(<Input aria-label="Tên" radius="lg" />)
    expect(screen.getByRole('textbox').className).toContain('rounded-lg')
  })

  it('radius="lg" KHÔNG kèm rounded-md — nếu kèm thì bo góc sụp về 8px mà test vẫn xanh', () => {
    render(<Input aria-label="Tên" radius="lg" />)
    expect(screen.getByRole('textbox').className).not.toContain('rounded-md')
  })

  it('variant="search" giữ pill bất kể radius', () => {
    render(<Input aria-label="Tìm" variant="search" radius="lg" />)
    const cls = screen.getByRole('textbox').className
    expect(cls).toContain('rounded-full')
    expect(cls).not.toContain('rounded-lg')
  })
})
```

Test thứ ba và thứ tư là loại phép kiểm mà §7.5 của spec bắt buộc: khẳng định class chọi **vắng mặt**, vì `vitest` chạy `css: false` nên chỉ thấy chuỗi chứ không thấy hiệu lực.

- [ ] **Step 2: Chạy test, xác nhận đỏ**

Run: `cd frontend && npx vitest run src/components/primitives/Input.test.tsx`
Expected: FAIL — prop `radius` chưa tồn tại.

- [ ] **Step 3: Sửa `Input.tsx`**

Thay khối type và `variantStyles` bằng:

```tsx
type InputVariant = 'default' | 'search'
type InputRadius = 'md' | 'lg'

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  error?: string
  helpText?: string
  variant?: InputVariant
  radius?: InputRadius
}

/**
 * Bo góc đi qua prop chứ không qua `className`: `rounded-md` nướng trong chuỗi
 * sẽ hoà độ đặc hiệu với `rounded-lg` truyền vào và thắng theo thứ tự stylesheet,
 * làm ô nhập sụp từ 12px về 8px mà không gì báo. Đã xảy ra 3 lần ở chặng 1.
 * Xem spec 2026-08-01 §7.5.
 *
 * `search` là pill nên bỏ qua `radius` — Stitch §Inputs quy định như vậy.
 */
const radiusStyles: Record<InputRadius, string> = {
  md: 'rounded-md',
  lg: 'rounded-lg',
}

const variantStyles: Record<InputVariant, string> = {
  default: 'bg-surface-container-lowest',
  search: 'rounded-full bg-surface-container-low',
}
```

Trong phần render, thay dòng `${variantStyles[variant]}` bằng:

```tsx
          ${variant === 'search' ? '' : radiusStyles[radius]}
          ${variantStyles[variant]}
```

và thêm `radius = 'md'` vào danh sách destructure props.

- [ ] **Step 4: Chạy test, xác nhận xanh**

Run: `cd frontend && npx vitest run src/components/primitives/Input.test.tsx`
Expected: PASS, 9 test (5 cũ + 4 mới).

- [ ] **Step 5: Xác minh trên trình duyệt rằng bo góc thật sự đổi**

Test chỉ thấy chuỗi. Dựng harness với CSS đã build và đo:

```bash
cd frontend
npm run build
CSS=$(ls dist/assets/*.css | head -1)
BASE="w-full px-3 py-2 text-small border text-on-surface transition-colors duration-fast focus-ring bg-surface-container-lowest border-outline-variant"
{
  echo "<link rel=\"stylesheet\" href=\"file://$PWD/$CSS\">"
  echo "<div style=\"padding:40px;display:flex;gap:16px\">"
  echo "  <input id=\"md\" class=\"$BASE rounded-md\">"
  echo "  <input id=\"lg\" class=\"$BASE rounded-lg\">"
  echo "</div>"
} > /tmp/input-radius.html
B="$HOME/.claude/skills/gstack/browse/dist/browse"
$B goto "file:///tmp/input-radius.html"
$B js "JSON.stringify(['md','lg'].map(i=>i+': '+getComputedStyle(document.getElementById(i)).borderRadius))"
```

Expected: `["md: 8px","lg: 12px"]`. Nếu cả hai ra cùng một giá trị, prop không có tác dụng — **dừng và báo**.

- [ ] **Step 6: Chạy toàn bộ test và build**

Run: `cd frontend && npm run typecheck:diff && npm run build && npm test`
Expected: 46 test xanh (42 + 4), không file nào tăng lỗi type.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/components/primitives/Input.tsx frontend/src/components/primitives/Input.test.tsx
git commit -m "feat(primitives): let Input callers choose their radius

Five of the seven non-search inputs still to migrate want 12px; the primitive
bakes 8px and Stitch says nothing about the default field's radius, so neither
value is wrong. Passing rounded-lg through className does not work — it ties
with the baked rounded-md and loses on stylesheet order, which silently
collapsed three inputs during the pilot.

A prop makes the choice explicit and type-checked instead of a cascade race.
The default stays md so the nineteen existing call sites are untouched."
```

---

## Chặng 2 — `components/composites` + `components/patterns` (66)

Bộ khung của ứng dụng: sidebar, thanh trên, danh sách chat, cây thư mục. Mọi bề mặt khác render bên trong chúng, nên bán kính ảnh hưởng lớn nhất. Làm sau khi chặng 1 đã chứng minh primitive.

**`NavRow` được dùng thật lần đầu ở đây.** Nó được tạo ở chặng 0 cho ~16 dòng điều hướng đang giả dạng `<button>`, và cho tới giờ **chưa có chỗ dùng nào**. `AppSidebar.tsx`, `ContactsSidebar.tsx`, `LarkRail.tsx`, `TreeView.tsx`, `MobileNav.tsx` là nơi kiểm chứng nó. Nếu `NavRow` không vừa với chúng, đó là phát hiện đáng báo — đừng bẻ cong.

### Task 2: `AppSidebar.tsx` (9 — thô 7, px 2)

**Files:** Modify `frontend/src/components/patterns/AppSidebar.tsx`

- [ ] Theo quy trình B1–B6. Đây là file chính để kiểm chứng `NavRow`: các mục Chat / Drive / Approvals / Contacts / Workplace / Admin là dòng điều hướng có trạng thái active, đúng thứ `NavRow kind="subItem"` được thiết kế cho.
- [ ] Đo trạng thái active bằng computed style trước và sau, xác nhận `bg-primary-fixed` và `text-on-primary-fixed-variant` thật sự render — `on-primary-fixed-variant` là token mới nhất trong `index.css` và đây là chỗ tiêu thụ thật đầu tiên ngoài test.
- [ ] Commit với message nêu rõ `NavRow` có vừa hay không.

### Task 3: `ChatInput.tsx` (7 — thô 6, px 1)

**Files:** Modify `frontend/src/components/patterns/ChatInput.tsx`

- [ ] Theo quy trình B1–B6.
- [ ] Nút gửi tin nhắn thường là `bg-primary … rounded-full` — đó chính là `IconButton variant="filled"`, được thêm ở chặng 0 và **chưa có chỗ dùng nào**. Kiểm chứng nó ở đây.
- [ ] `filled` cố ý bỏ qua `tone` (nếu không `text-on-primary` sẽ bị `text-outline` của tone mặc định đè). Xác nhận bằng computed style rằng chữ trong nút vẫn trắng.

### Task 4: `ChatList.tsx` (7 — thô 4, px 3)

**Files:** Modify `frontend/src/components/patterns/ChatList.tsx`

- [ ] Theo quy trình B1–B6.
- [ ] File 352 dòng. Các hàng hội thoại là ứng viên `NavRow`, nhưng chúng có avatar + nhiều dòng text + badge — nhiều khả năng là loại (b). Đo trước khi quyết.

### Task 5: `ImagePreviewCard.tsx` (5 — px 4, shadow 1) — **có bug sống**

**Files:** Modify `frontend/src/components/patterns/ImagePreviewCard.tsx`

- [ ] Dòng 72 chứa `shadow-[0_0_0_2px_rgba(var(--md-primary-rgb),0.15)]`. Biến `--md-primary-rgb` **không tồn tại** trong `index.css` → shadow này hiện **không render gì cả**. Đây là bug có sẵn, không phải do di cư.
- [ ] Xác nhận trước bằng: `grep -c 'md-primary-rgb' src/index.css` → phải là `0`.
- [ ] Thay bằng token phù hợp. Ý đồ là vòng sáng 2px màu primary ở 15% — `--shadow-accent-sm` là bóng chứ không phải ring, nên nhiều khả năng đúng nhất là dùng `focus-ring` utility hoặc `ring-2 ring-primary/15`. **Đo kết quả**, và nếu không token nào diễn đạt được ý đồ thì báo cáo thay vì chế token mới.
- [ ] Theo quy trình B1–B6 cho 4 giá trị pixel còn lại.

### Task 6: `MessageItem.tsx` (5 — thô 4, shadow 1)

**Files:** Modify `frontend/src/components/patterns/MessageItem.tsx`

- [ ] Theo quy trình B1–B6.
- [ ] File này có `BUBBLE_INCOMING` / `BUBBLE_SELF` là **hằng template ở cấp module**, không phải `className` nội tuyến. Rule lint bắt được chúng vì selector đã được bỏ neo khỏi `className` — nhưng khi sửa, nhớ rằng chuỗi nằm ngoài JSX nên các phép kiểm dựa trên thẻ sẽ không thấy.

### Task 7: `ChatListItem.tsx` (4 — thô 1, shadow 1, màu 1, px 1)

**Files:** Modify `frontend/src/components/patterns/ChatListItem.tsx`

- [ ] Theo quy trình B1–B6.
- [ ] Vi phạm "màu" ở dòng 54 là `bg-green-500` cho chấm trạng thái online. Theo **D9** của spec: dùng token `--color-status-online`, không dùng bảng màu Tailwind thô. `.stitch/DESIGN.md` §Status Colors ghi ngược lại, nhưng đó là dấu vết của quy trình sinh màn hình bên Stitch — 8/11 chỗ trong code đã dùng token, và `WORKFLOW.md` §Anti-Patterns tự cấm class tuỳ tiện.

### Task 8: `ContactProfilePanel.tsx` (4 — thô 3, shadow 1)

**Files:** Modify `frontend/src/components/patterns/ContactProfilePanel.tsx`

- [ ] Theo quy trình B1–B6.
- [ ] `.stitch/DESIGN.md` §Avatars quy định panel avatar là `w-24 h-24 rounded-full border-4 border-white shadow-md` và §Layout quy định panel là `w-96 fixed right-0 top-14 bottom-0 shadow-xl`. Đối chiếu trước khi đổi.

### Task 9: `TopBar.tsx` (4 — thô 4)

**Files:** Modify `frontend/src/components/patterns/TopBar.tsx`

- [ ] Theo quy trình B1–B6.
- [ ] Ô tìm kiếm toàn cục ở đây là ứng viên rõ ràng cho `Input variant="search"` (pill trên `surface-container-low`), thứ được thêm ở chặng 0 và **chưa có chỗ dùng nào**. Kiểm chứng nó.

### Task 10: Nhóm sidebar còn lại — `ContactsFilterBar` (3), `ContactsSidebar` (3), `LarkRail` (3)

**Files:** Modify cả ba file trong `frontend/src/components/patterns/`

- [ ] Theo quy trình B1–B6, một commit chung vì cả ba đều là dòng điều hướng và cùng kiểm chứng `NavRow`.
- [ ] `ContactsSidebar.tsx` dòng 22 và 129 có **comment nhắc tới `bg-primary-fixed text-on-primary-fixed-variant`** mà code không dùng. Nếu chuyển sang `NavRow`, comment thành đúng — cập nhật cho khớp thay vì để lệch.

### Task 11: Nhóm nhỏ chặng 2 — `Breadcrumbs` (2), `ChannelDrivePanel` (2), `ContactCard` (2), `ContactsTable` (2), `Modal` (1), `Tabs` (1), `MobileNav` (1), `TreeView` (1)

**Files:** Modify 8 file trong `frontend/src/components/composites/` và `frontend/src/components/patterns/`

- [ ] Theo quy trình B1–B6, một commit chung.
- [ ] `composites/Modal.tsx` chứa `max-w-[640px]` trong **object variant-map** (`Record<ModalSize,string>`), không phải `className` nội tuyến — một trong hai chỗ khiến rule phải bỏ neo khỏi `className`. Sửa như bình thường; chỉ lưu ý nó không nằm trong JSX.
- [ ] `MobileNav.tsx` và `TreeView.tsx` là ứng viên `NavRow`.

### Task 12: Nghiệm thu chặng 2

- [ ] `npx eslint src/components/composites src/components/patterns` → 0 (chạy hai lệnh riêng; ESLint không nhận nhiều đường dẫn trong một chuỗi).
- [ ] `npm run lint` → **205** (271 − 66).
- [ ] `npm run typecheck:diff && npm run build && npm test`.
- [ ] Chụp ảnh 375 / 768 / 1280 cho một màn hình có sidebar và một màn hình có chat. Lưu vào `docs/superpowers/plans/screenshots/2026-08-01-stage2/`.
- [ ] Ghi vào spec §8 một mục ngắn: `NavRow`, `IconButton variant="filled"`, `Input variant="search"` — mỗi thứ có vừa với chỗ dùng thật đầu tiên của nó không. Cả ba đều được tạo ở chặng 0 mà chưa từng được dùng; đây là lần đầu chúng bị thực tế kiểm tra.
- [ ] Commit.

---

## Chặng 3 — `routes/_workspace` (66)

Route vỏ, phụ thuộc `patterns` vừa xong.

### Task 13: `drive.tsx` (18 — thô 6, px 6, shadow 6)

**Files:** Modify `frontend/src/routes/_workspace/drive.tsx`

- [ ] File nặng nhất của chặng. Theo quy trình B1–B6.
- [ ] 6 shadow tuỳ tiện — tra bảng ánh xạ ở B5 trước, có thể có chỗ trùng khít `shadow-card`.

### Task 14: `channels.$channelId.tsx` (10 — thô 6, px 4)

**Files:** Modify `frontend/src/routes/_workspace/channels.$channelId.tsx`

- [ ] Theo quy trình B1–B6.

### Task 15: `admin/roles.tsx` (9 — px 8, thô 1)

**Files:** Modify `frontend/src/routes/_workspace/admin/roles.tsx`

- [ ] Theo quy trình B1–B6. Gần như toàn bộ là giá trị pixel — công việc cơ học, nhưng vẫn phải xác minh từng giá trị từ CSS đã build.

### Task 16: `admin/users.tsx` (8 — thô 5, px 3)

**Files:** Modify `frontend/src/routes/_workspace/admin/users.tsx`

- [ ] Theo quy trình B1–B6.

### Task 17: `_workspace.tsx` (6 — thô 6) và `contacts.tsx` (6 — thô 6)

**Files:** Modify `frontend/src/routes/_workspace.tsx`, `frontend/src/routes/_workspace/contacts.tsx`

- [ ] Theo quy trình B1–B6, một commit chung — cả hai chỉ có thẻ thô.

### Task 18: Nhóm nhỏ chặng 3 — `admin/index.tsx` (4), `documents.tsx` (4), `admin.tsx` (1)

**Files:** Modify ba file trong `frontend/src/routes/_workspace/`

- [ ] Theo quy trình B1–B6, một commit chung.
- [ ] `documents.tsx` có 1 hex: `color="#3b82f6"` trên icon của `EmptyState`. Đây là hex nằm ở **prop `color=`**, không phải `className` — lý do rule 3 phải quét mọi string literal. Thay bằng token.

### Task 19: Nghiệm thu chặng 3

- [ ] `npx eslint src/routes/_workspace.tsx` và `npx eslint src/routes/_workspace` → 0.
- [ ] `npm run lint` → **139** (205 − 66).
- [ ] `npm run typecheck:diff && npm run build && npm test`.
- [ ] Ảnh 3 breakpoint, lưu `…/screenshots/2026-08-01-stage3/`.
- [ ] Commit.

---

## Chặng 4 — `components/drive` (66)

**Chặng có ngoại lệ được duyệt trước.** Spec **D6** cho phép giữ nguyên số đo pixel trong thư mục này kèm chú thích miễn trừ, vì chúng là chiều cao hàng và bề rộng cột đã căn chỉnh có chủ ý cho mật độ, và không nguồn nào quy định chúng: `DESIGN.md` chỉ có lưới bảng cho màn hình contacts, HTML gốc của `drive-my-files` không tải được (§6.1), và `docs/specs/lark-data-table-layout/spec.md` tả hành vi chứ không tả số đo.

**Nhưng miễn trừ không phải mặc định.** Quy tắc cho chặng này: với **mỗi** giá trị pixel, thử quy về thang trước; chỉ giữ nguyên khi việc quy đổi làm đổi mật độ thấy được, và khi đó chú thích phải nêu **số đo cụ thể** chứ không nói chung chung. Spec §7.3 đã cảnh báo đây là lối thoát dễ bị lạm dụng.

### Task 20: `DriveSidebar.tsx` (22 — px 13, thô 6, shadow 3)

**Files:** Modify `frontend/src/components/drive/DriveSidebar.tsx`

- [ ] File nhiều vi phạm nhất toàn repo. Theo quy trình B1–B6, cộng quy tắc miễn trừ của chặng 4.
- [ ] Các mục All Files / Recent / Shared / Starred / Trash là dòng điều hướng — ứng viên `NavRow`.

### Task 21: `DriveContextPanel.tsx` (13 — px 10, thô 3)

**Files:** Modify `frontend/src/components/drive/DriveContextPanel.tsx`

- [ ] Theo quy trình B1–B6 + quy tắc miễn trừ chặng 4.
- [ ] Có capability spec riêng: `docs/specs/drive-context-panel/spec.md`. Đọc trước, và nếu di cư làm đổi hành vi được mô tả ở đó thì **dừng và báo** — đổi hành vi cần spec và test riêng theo Enforcement Rules của `CLAUDE.md`.

### Task 22: `DriveFileRow.tsx` (13 — px 10, shadow 2, thô 1)

**Files:** Modify `frontend/src/components/drive/DriveFileRow.tsx`

- [ ] Đây là nơi lập luận mật độ mạnh nhất — hàng bảng dữ liệu. Theo quy tắc miễn trừ chặng 4.
- [ ] `docs/specs/lark-data-table-layout/spec.md` quy định hàng bảng **không được đổ bóng khi hover**, chỉ đổi nền phẳng. File này có 2 shadow — **đối chiếu và báo cáo** nếu vi phạm, không tự sửa hành vi.

### Task 23: Nhóm còn lại chặng 4 — `DriveFilterPills` (4), `ShareDialog` (4), `DriveFileList` (3), `DriveTreePanel` (3), `DrivePreviewDialog` (2), `FolderTreeSelect` (2)

**Files:** Modify 6 file trong `frontend/src/components/drive/`

- [ ] Theo quy trình B1–B6 + quy tắc miễn trừ chặng 4, một commit chung.
- [ ] `DriveTreePanel` và `FolderTreeSelect` là ứng viên `NavRow`.

### Task 24: Nghiệm thu chặng 4

- [ ] `npx eslint src/components/drive` → 0, **hoặc** chỉ còn các dòng có `eslint-disable` kèm lý do nêu số đo cụ thể.
- [ ] `npm run lint` → **73** (139 − 66).
- [ ] Đếm và liệt kê mọi miễn trừ đã thêm: `grep -rn 'eslint-disable-next-line' src/components/drive/`. Mỗi cái phải có lý do. Báo cáo tổng số — nếu vượt quá một phần ba số vi phạm ban đầu của chặng, **dừng và báo**, vì đó là dấu hiệu miễn trừ đang được dùng thay cho di cư.
- [ ] `npm run typecheck:diff && npm run build && npm test`.
- [ ] Ảnh 3 breakpoint, lưu `…/screenshots/2026-08-01-stage4/`.
- [ ] Commit.

---

## Chặng 5 — `components/chat` (30)

### Task 25: `ChannelInfoPanel.tsx` (13 — px 5, thô 5, màu 3)

**Files:** Modify `frontend/src/components/chat/ChannelInfoPanel.tsx`

- [ ] Theo quy trình B1–B6.
- [ ] 3 vi phạm "màu" là chấm trạng thái online/offline dùng bảng Tailwind thô. Theo **D9**: chuyển sang token `--color-status-*`.
- [ ] File này cũng có `IconButton` tự tô màu `hover:text-error` — đó là `tone="danger"` đã thêm ở chặng 0.

### Task 26: Nhóm chat còn lại — `MessageList` (4), `MentionDropdown` (3), `ChatEditor` (2), `MessageContent` (2), `ReactionBar` (2), `EditorToolbar` (1), `HoverActionBar` (1), `ThreadPanel` (1)

**Files:** Modify 8 file trong `frontend/src/components/chat/`

- [ ] Theo quy trình B1–B6, một commit chung.
- [ ] `ChatEditor.tsx` và `ThreadPanel.tsx` có nút gửi tự chế `bg-primary text-on-primary … rounded-full` — `IconButton variant="filled"`.
- [ ] `MessageContent.tsx` có 1 vi phạm màu: `text-green-600` ở trạng thái "Copied". Dùng token `--color-success`.

### Task 27: `EmojiPicker.tsx` (1 shadow) và dọn chỉ thị disable thừa

**Files:** Modify `frontend/src/components/chat/EmojiPicker.tsx`

- [ ] Sửa 1 shadow theo B5.
- [ ] File này còn có chỉ thị `// eslint-disable-next-line react-hooks/exhaustive-deps` ở dòng 51 mà ESLint báo là **thừa** (`Unused eslint-disable directive`) — đó là cảnh báo thứ 272, không thuộc 5 rule design system.
- [ ] Nguyên nhân: `eslint-plugin-react-hooks` được khai cho `**/*.{js,jsx}`, mà `src/` có **0 file `.js`/`.jsx`** trên 113 file `.tsx`. Rules of Hooks **chưa từng kiểm một dòng nào** của ứng dụng. Khối `.tsx` chỉ đăng ký plugin để chỉ thị phân giải được, không bật rule nào.
- [ ] **Không xoá chỉ thị** — chú thích bên cạnh nó ghi *"Mount-only, matching the upstream wrapper's behaviour"*, tức là tác giả đã cân nhắc. Giữ nguyên và **báo cáo** rằng Rules of Hooks đang không được kiểm, để chủ dự án quyết riêng. Đó là việc ngoài phạm vi kế hoạch này.

### Task 28: Nghiệm thu chặng 5

- [ ] `npx eslint src/components/chat` → chỉ còn 1 cảnh báo `Unused eslint-disable directive`, 0 vi phạm design system.
- [ ] `npm run lint` → **43** (73 − 30) vi phạm design system, ESLint in ra 44.
- [ ] `npm run typecheck:diff && npm run build && npm test`.
- [ ] Ảnh 3 breakpoint, lưu `…/screenshots/2026-08-01-stage5/`.
- [ ] Commit.

---

## Chặng 6 — `approval` + `assets` + `components/` gốc + `lib/` (43)

### Task 29: `lib/constants.ts` (9 hex) — và các chỗ tiêu thụ nó

**Files:**
- Modify: `frontend/src/lib/constants.ts`
- Modify: `frontend/src/routes/assets/list.tsx`, `frontend/src/routes/assets/$assetId.tsx`

- [ ] `ASSET_STATE_COLORS` và `REQUEST_STATUS_COLORS` chứa 9 hex. Đây là **màu trạng thái**, khác hẳn `lib/fileIcons.ts` (48 hex, miễn trừ vĩnh viễn vì là bản sắc loại tệp — xem spec §1.3-B).
- [ ] `index.css` đã có `--color-success`, `--color-warning`, `--color-danger`, `--color-info` cho đúng mục đích này.
- [ ] Vấn đề: các object này trả về **chuỗi màu** dùng cho `style={{ color }}` chứ không phải class Tailwind. Nên không thể thay bằng tên class trực tiếp. Hai lối:
  - đổi giá trị sang `var(--color-success)` v.v. — hoạt động trong `style` inline vì CSS variable phân giải được ở runtime;
  - hoặc đổi cả cấu trúc sang trả về tên class rồi dùng `className`.
  Lối thứ nhất nhỏ hơn và không đụng nơi tiêu thụ. **Đo bằng computed style** rằng màu render ra đúng như trước khi đổi.
- [ ] `routes/assets/list.tsx` và `$assetId.tsx` mỗi file có 1 hex `'#6b7280'` làm giá trị mặc định khi tra bảng không thấy — thay bằng cùng token với `--color-outline`.

### Task 30: `routes/assets/list.tsx` (8 — px 6, thô 1, hex 1) và `routes/assets.tsx` (6 — px 3, thô 3)

**Files:** Modify cả hai file

- [ ] Theo quy trình B1–B6, một commit chung. Hex của `list.tsx` đã xử ở Task 29 nếu làm trước.

### Task 31: `NotificationBell.tsx` (5 — px 4, thô 1) và `InviteMemberForm.tsx` (1 px)

**Files:** Modify `frontend/src/components/NotificationBell.tsx`, `frontend/src/components/InviteMemberForm.tsx`

- [ ] Theo quy trình B1–B6, một commit chung.

### Task 32: Nhóm approval — `ApprovalTable` (2), `TemplateDetailPanel` (2), `ApprovalDetailPanel` (1), `BatchActionBar` (1), `DynamicFormRenderer` (1), `FormFieldBuilder` (1)

**Files:** Modify 6 file trong `frontend/src/components/approval/`

- [ ] Theo quy trình B1–B6, một commit chung.

### Task 33: Nhóm cuối — `routes/assets/request/new.tsx` (4), `routes/assets/dashboard.tsx` (1), `components/ErrorState.tsx` (1 hex)

**Files:** Modify ba file

- [ ] Theo quy trình B1–B6, một commit chung.
- [ ] `ErrorState.tsx` có `color="#f59e0b"` trên icon — hex ở prop, thay bằng `var(--color-warning)`.

### Task 34: Nghiệm thu chặng 6

- [ ] `npm run lint` → **0** vi phạm design system; ESLint in ra 1 (cảnh báo `Unused eslint-disable directive` ở `EmojiPicker.tsx`).
- [ ] `npm run typecheck:diff && npm run build && npm test`.
- [ ] Ảnh 3 breakpoint, lưu `…/screenshots/2026-08-01-stage6/`.
- [ ] Commit.

---

## Chặng 7 — Khoá lint và cập nhật spec

### Task 35: Khoá rule từ `warn` thành `error`

**Files:** Modify `frontend/eslint.config.js`

- [ ] **Điều kiện tiên quyết:** `npm run lint` phải cho 0 vi phạm design system. Nếu chưa, dừng — chặng này không được bắt đầu sớm.

- [ ] **Step 1: Đổi mức trong cả ba khối cấu hình**

Trong `frontend/eslint.config.js`, đổi `'warn'` thành `'error'` ở cả ba chỗ khai `no-restricted-syntax`: khối `**/*.{ts,tsx}`, khối miễn trừ `src/components/primitives/**/*.tsx`, và khối miễn trừ `src/lib/fileIcons.ts` + `src/routes/_auth/login.tsx`.

- [ ] **Step 2: Xác nhận lint vẫn xanh**

Run: `cd frontend && npm run lint; echo "exit=$?"`
Expected: exit 0. Chỉ còn cảnh báo `Unused eslint-disable directive`, không có error.

- [ ] **Step 3: Chứng minh cổng biết fail**

Rule ở mức `error` mà không chặn được gì thì vô nghĩa. Chèn tạm một vi phạm rồi kiểm:

```bash
cd frontend && printf '\nexport const __probe = () => <button className="p-[7px]">x</button>\n' >> src/routes/index.tsx
npm run lint; echo "exit=$?"
```
Expected: exit **1**, báo đúng hai lỗi (thẻ thô + giá trị pixel) ở `src/routes/index.tsx`.

Sau đó hoàn tác: `cd frontend && git checkout src/routes/index.tsx` và chạy lại `npm run lint` để xác nhận về xanh. **Không commit khi chưa hoàn tác** — `git status` phải sạch ngoài `eslint.config.js`.

- [ ] **Step 4: Commit**

```bash
git add frontend/eslint.config.js
git commit -m "build(lint): lock the design-system rules to error

The violation count reached zero, so the rules stop being a progress meter
and start being a gate. Verified the gate fails: a probe violation makes the
lint run exit non-zero and names both rules it breaks."
```

### Task 36: Cập nhật ba capability spec

**Files:**
- Modify: `docs/specs/lark-sidebar-layout/spec.md`
- Modify: `docs/specs/lark-data-table-layout/spec.md`
- Modify: `docs/specs/lark-messaging-layout/spec.md`

- [ ] `docs/specs/README.md` quy định: *"silently editing a spec to match the code destroys the only record that a decision was ever made"*. Nên **ghi thêm** vào phần `## Status`, **không xoá** ghi chú sai lệch có sẵn.
- [ ] Mỗi spec ghi một mục ngắn: đợt ép design system đã chạm những file nào của capability đó, và những sai lệch đã ghi nhận trước đây (`lark-sidebar-layout` "Diverges — 280px, không có nút thu gọn"; `lark-data-table-layout` "Partially implemented — Drive thiếu breadcrumb") **vẫn còn nguyên**, vì sửa chúng là đổi hành vi và nằm ngoài phạm vi.
- [ ] Nếu chặng 4 phát hiện hàng bảng đổ bóng khi hover (vi phạm `lark-data-table-layout`), ghi vào `## Status` của spec đó.
- [ ] Commit.

### Task 37: Nghiệm thu toàn bộ

- [ ] `npm run lint` → 0 error, chỉ còn cảnh báo `Unused eslint-disable directive`; exit 0.
- [ ] `npm run typecheck:diff` → không file nào tăng lỗi so với baseline 123.
- [ ] `npm run build` xanh, `npm test` xanh.
- [ ] `grep -rn '37,\s*99,\s*235' frontend/src/` → 0.
- [ ] Không còn `<button>`/`<input>`/`<select>`/`<textarea>` thô ngoài `components/primitives/`, trừ các chỗ có `eslint-disable` kèm lý do. Đếm và liệt kê toàn bộ miễn trừ của cả dự án: `grep -rn 'eslint-disable-next-line no-restricted-syntax' frontend/src/`.
- [ ] Ghi mục tổng kết vào spec §8: tổng số miễn trừ, phân theo lý do, và đánh giá xem có cái nào nên được giải quyết bằng cách mở rộng từ vựng primitive thay vì miễn trừ.
- [ ] Commit.

---

## Điều kế hoạch này cố ý KHÔNG làm

- **Không sửa các sai lệch có sẵn giữa capability spec và code** (sidebar 280px không có nút thu gọn, Drive thiếu breadcrumb). Đó là đổi hành vi, cần spec và test riêng theo Enforcement Rules của `CLAUDE.md`.
- **Không sửa lỗi định tuyến** khiến `/workspace-select` không tới được khi đã đăng nhập (`_auth.tsx` trả `<Navigate to="/documents"/>` ngay khi `isAuth`, trong khi `login.tsx` đặt token rồi mới điều hướng sang đó).
- **Không sửa `max-w-2xl` của `onboarding`** vốn không bao giờ có hiệu lực vì nằm lồng trong khung `max-w-110` của `_auth.tsx`.
- **Không bật Rules of Hooks** dù nó đang không kiểm một dòng nào — xem Task 27.
- **Không bật `tseslint.configs.recommended`.** 123 lỗi type có sẵn sẽ chôn vùi tín hiệu design system, đúng cái bẫy mà §4.3 của spec đã tránh.
- **Không xoá 4 workspace "Second Org"** thừa trong DB dev: `CLAUDE.md` bắt mọi ghi vào `ngac_*` phải đi qua đường EPP invalidation, mà không có REST endpoint xoá workspace.

Mỗi mục trên đều là lỗi hoặc nợ **có thật**, đã được xác minh, và đáng làm — nhưng làm chúng ở đây sẽ biến một đợt chuẩn hoá thị giác thành một đợt đổi hành vi, và không còn review được nữa.
