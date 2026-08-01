# Ép Design System Lên UI — Kế Hoạch Thi Công (Chặng 0 + Chặng 1)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Đưa token và primitive về khớp Stitch, bật ESLint chặn vi phạm design system, rồi di cư trọn bề mặt auth làm phép thử cho toàn bộ migration.

**Architecture:** Sửa nền trước, quét nhà sau. Token và primitive là toàn cục nên làm dứt điểm một đợt (chặng 0); bề mặt auth biệt lập nên dùng làm chặng thí điểm (chặng 1) để chứng minh primitive đúng trước khi áp lên 200+ vi phạm còn lại. Số vi phạm ESLint là thước đo tiến độ.

**Tech Stack:** React 19, Tailwind 4 (CSS-first `@theme`), TypeScript, Vitest + Testing Library, ESLint 10 flat config.

**Nguồn:** `docs/superpowers/specs/2026-08-01-ui-design-system-enforcement-design.md`

---

## Phạm vi của kế hoạch này

Kế hoạch này phủ **chặng 0 và chặng 1** của spec (45 trong 350 vi phạm). Đây là đơn vị hoàn chỉnh và chạy được: token đúng, primitive đúng và có test, lint đã bật, một bề mặt đã di cư trọn vẹn.

**Chặng 2–7 sẽ có kế hoạch riêng, viết sau khi chặng 1 xong.** Đây là quyết định phân rã có chủ đích, không phải chỗ bỏ ngỏ: chặng 1 tồn tại để phát hiện primitive thiết kế sai. Viết sẵn 250 bước cho chặng 2–6 bây giờ là giả định chặng 1 không dạy được gì — trong khi mục đích duy nhất của nó là dạy.

## Thay đổi so với spec

Ba điều được phát hiện lúc lập kế hoạch, đã được người dùng duyệt:

1. **Variant `success` được giữ và hiệu chỉnh, không xoá.** Spec kết luận xoá vì 0 lượt khai báo. Nhưng 4 chỗ tự chế đúng style đó bằng `className` — vì variant có sẵn là dạng nền nhạt trong khi màn hình cần nền đặc. Đúng luận điểm trung tâm của spec. Không rule lint nào bắt được kiểu đè `className` này, nên xoá variant sẽ để lại 4 điểm lệch vô hình.
2. **Số call site vỡ khi gộp variant là 6.** Bản nháp ghi 7 vì đếm cả `AlertBanner` (kiểu `AlertVariant` riêng) và một comment — sửa xuống 5. Nhưng 5 **cũng sai**, phát hiện lúc thi công Task 3: `ConfirmDialog.tsx:112` viết `<Button variant={confirmVariant}>`, giá trị đi qua biến nên grep chuỗi không thấy, chỉ tầng kiểu mới lộ. Prop công khai `confirmVariant` đổi sang `'primary' | 'danger'`, kéo theo 4 caller. Phạm vi Task 3 là **11 file**, không phải 8 — xem spec §5.2.
3. **`--color-on-success` không tồn tại** nhưng `ApprovalDetailPanel.tsx:142` và `BatchActionBar.tsx:39` đều dùng `text-on-success` → hai nút Approve hiện không được áp màu chữ. Bug sống, cùng loại với `--md-primary-rgb`.

4. **Thêm rule lint thứ 5 cho bóng đổ, và ba token bóng nhấn.** Bước tự soát kế hoạch phát hiện rule 2 **không bắt được** `shadow-[0_4px_16px_rgba(0,0,0,0.04)]` — regex đòi cả cụm trong ngoặc phải là `\d+(px|rem)`, nên có `4px` bên trong vẫn lọt. Không vá thì 34 shadow tuỳ tiện và 4 `shadow-xl` vĩnh viễn vô hình với thước đo, và mục 5.6 sẽ báo "0 vi phạm" trong khi chúng còn nguyên. Kèm theo, ba token `--shadow-accent*` được thêm để Task 2 có đích chuyển sang — nếu không, việc sửa `rgba(37,99,235,…)` chỉ đổi vi phạm hardcode màu lấy vi phạm giá trị tuỳ tiện. Tổng vi phạm do đó là 407/57/350, không phải 369/57/312.

## Cấu trúc file

**Tạo mới**

| File | Trách nhiệm |
|---|---|
| `frontend/src/components/primitives/NavRow.tsx` | Dòng điều hướng/danh sách có thể bấm — thứ ~16 chỗ đang giả dạng `<button>` |
| `frontend/src/components/primitives/Button.test.tsx` | Ghim hình học Stitch của `Button` |
| `frontend/src/components/primitives/IconButton.test.tsx` | Ghim variant và tone của `IconButton` |
| `frontend/src/components/primitives/Input.test.tsx` | Ghim dạng mặc định và dạng pill tìm kiếm |
| `frontend/src/components/primitives/NavRow.test.tsx` | Ghim trạng thái active/inactive theo Stitch |
| `frontend/scripts/typecheck-diff.sh` | So lỗi type theo từng file với baseline — dự án có 123 lỗi sẵn nên cổng "0 lỗi" bất khả thi |
| `frontend/typecheck-baseline.txt` | Baseline lỗi type theo file, sinh ở Task 0 |

**Sửa**

| File | Thay đổi |
|---|---|
| `frontend/src/index.css` | Màu primary, `on-primary-fixed-variant`, `on-success`, `shadow-card`, ba token `shadow-accent*`, 3 chỗ `rgba(37,99,235)` |
| `frontend/src/components/primitives/Button.tsx` | 7 variant → 5, thang size theo Stitch |
| `frontend/src/components/primitives/IconButton.tsx` | Thêm `variant` và `tone` |
| `frontend/src/components/primitives/Input.tsx` | Thêm `variant="search"` |
| `frontend/src/components/primitives/Textarea.tsx` | `min-h-[72px]` → giá trị theo thang |
| `frontend/src/components/primitives/index.ts` | Export `NavRow` |
| `frontend/eslint.config.js` | Parser TS + 5 rule design system ở mức `warn` |
| `frontend/package.json` | Thêm `typescript` và `typescript-eslint`; script `typecheck` và `typecheck:diff` |
| 5 call site vỡ do gộp variant | `variant="outline"` → `secondary`, `variant="error"` → `danger` |
| 4 call site tự chế success | Dùng `variant="success"` / `tone="success"` |
| 5 chỗ `rgba(37,99,235)` trong `.tsx` | Chuyển sang `color-mix` trên token |
| 7 file bề mặt auth | Di cư sang primitive |

---

## Task 0: Dựng lại tầng typecheck

Không có task này thì mọi cổng `tsc` trong kế hoạch đều vô nghĩa, và lưới an toàn cho phần gộp variant ở Task 3 không tồn tại.

Trạng thái hiện tại: `typescript` **không có** trong `package.json`. Bản duy nhất trong `node_modules` là **3.9.10**, kéo vào bắc cầu qua `@protobuf-ts/plugin` → `npx tsc` chạy bản 2020, không hiểu `jsx: "react-jsx"` lẫn `noUncheckedIndexedAccess` mà `tsconfig.json` khai, nên chỉ nôn ra lỗi cú pháp. Dự án chưa từng được typecheck.

Chạy TypeScript 5.9 thật cho **123 lỗi có sẵn**. Nên cổng nghiệm thu không thể là "0 lỗi" — phải so theo từng file với baseline.

**Files:**
- Modify: `frontend/package.json`
- Create: `frontend/scripts/typecheck-diff.sh`
- Create: `frontend/typecheck-baseline.txt`

- [ ] **Step 1: Xác nhận chẩn đoán trước khi sửa**

Chạy:
```bash
cd frontend && rtk proxy npx tsc --version && grep -c '"typescript"' package.json
```
Kết quả mong đợi: `Version 3.9.10` và `0`.

**Bắt buộc dùng `rtk proxy` ở lệnh này.** Hook `rtk` toàn cục chặn lệnh `tsc` trần và thay output bằng dòng tóm tắt của nó — `npx tsc --version` trả về `TypeScript: No errors found` thay vì số hiệu bản. Đã kiểm chứng: `npm run typecheck` và `npm run typecheck:diff` **không** bị lọc (chạy qua npm script), nên các cổng nghiệm thu ở những task sau vẫn đáng tin.

Nếu đã là 5.x, dừng lại — chẩn đoán này đã cũ, đọc lại trước khi làm tiếp.

- [ ] **Step 2: Cài TypeScript 5 làm phụ thuộc thật**

Chạy: `cd frontend && npm install -D typescript@5`
Kết quả mong đợi: cài xong; `npx tsc --version` in ra `Version 5.x`.

`typescript-eslint` ở Task 8 cũng cần nó, nên đây là phụ thuộc bắt buộc chứ không phải tiện tay thêm.

- [ ] **Step 3: Thêm hai script vào `package.json`**

Trong khối `"scripts"` của `frontend/package.json`, thêm sau dòng `"lint": "eslint .",`:

```json
    "typecheck": "tsc --noEmit -p tsconfig.json",
    "typecheck:diff": "bash scripts/typecheck-diff.sh",
```

- [ ] **Step 4: Viết script so sánh baseline**

Tạo `frontend/scripts/typecheck-diff.sh`:

```bash
#!/usr/bin/env bash
# So lỗi TypeScript theo TỪNG FILE với baseline đã ghi.
#
# Vì sao không so tổng số: dự án có 123 lỗi type có sẵn (chưa từng được
# typecheck cho tới 2026-08-01), nên cổng "0 lỗi" là bất khả thi và cổng
# "tổng không tăng" thì che mất việc một file sạch bắt đầu hỏng.
#
# Vì sao đếm theo file chứ không so từng dòng lỗi: số dòng xê dịch mỗi khi
# sửa file, sẽ sinh ra khác biệt giả.
set -uo pipefail
cd "$(dirname "$0")/.."

BASELINE=typecheck-baseline.txt
CURRENT=$(mktemp)
trap 'rm -f "$CURRENT"' EXIT

npx tsc --noEmit -p tsconfig.json 2>&1 \
  | grep -oE '^[^(]+\.tsx?' \
  | sort | uniq -c \
  | awk '{print $2"\t"$1}' | sort > "$CURRENT"

if [ ! -f "$BASELINE" ]; then
  echo "Chưa có $BASELINE. Chạy: cp $CURRENT $BASELINE"
  cat "$CURRENT"
  exit 1
fi

fail=0
while IFS=$'\t' read -r file count; do
  [ -z "$file" ] && continue
  base=$(awk -F'\t' -v f="$file" '$1 == f { print $2 }' "$BASELINE")
  base=${base:-0}
  if [ "$count" -gt "$base" ]; then
    echo "LỖI TYPE MỚI  $file: $count (baseline $base)"
    fail=1
  fi
done < "$CURRENT"

if [ "$fail" -eq 0 ]; then
  echo "typecheck: không file nào tăng lỗi so với baseline"
fi
exit "$fail"
```

Cấp quyền chạy: `chmod +x frontend/scripts/typecheck-diff.sh`

- [ ] **Step 5: Ghi baseline**

Chạy:
```bash
cd frontend && npx tsc --noEmit -p tsconfig.json 2>&1 \
  | grep -oE '^[^(]+\.tsx?' | sort | uniq -c \
  | awk '{print $2"\t"$1}' | sort > typecheck-baseline.txt
wc -l < typecheck-baseline.txt
```
Kết quả mong đợi: **52 dòng** (52 file có lỗi).

*Đính chính:* bản nháp ghi "khoảng 30 file" — con số đó lấy từ dòng tóm tắt của `rtk` khi chạy `tsc` **3.9.10**, tức là tóm tắt của một lần chạy hỏng. TypeScript 5 thật cho 52 file / 123 lỗi.

- [ ] **Step 6: Xác nhận tổng khớp con số đã khảo sát**

Chạy: `cd frontend && awk -F'\t' '{s+=$2} END {print s}' typecheck-baseline.txt`
Kết quả mong đợi: `123`

Nếu lệch, TypeScript đã cài không phải bản dùng lúc khảo sát — ghi lại con số thực nhận được và dùng nó làm baseline, không ép về 123.

- [ ] **Step 7: Xác nhận cổng chạy được và đang xanh**

Chạy: `cd frontend && npm run typecheck:diff`
Kết quả mong đợi: in `typecheck: không file nào tăng lỗi so với baseline`, thoát mã 0.

- [ ] **Step 8: Commit**

```bash
git add frontend/package.json frontend/package-lock.json frontend/scripts/typecheck-diff.sh frontend/typecheck-baseline.txt
git commit -m "build: make the project actually typecheck

typescript was never a dependency. The only copy in node_modules is 3.9.10,
pulled in transitively through @protobuf-ts/plugin, and it predates both
jsx: react-jsx and noUncheckedIndexedAccess that tsconfig.json declares — so
npx tsc emitted syntax errors and checked nothing. The project has never been
typechecked.

TypeScript 5 reports 123 pre-existing errors, so a zero-error gate is not
reachable and a total-count gate would hide a clean file starting to break.
typecheck:diff compares per-file counts against a recorded baseline, which is
also stable against the line shifts that editing causes."
```

---

## Task 1: Sửa token màu trong `index.css`

CSS thuần không chạy được qua Vitest (`vitest.config.ts` đặt `css: false`), nên task này verify bằng `grep` và biên dịch, không bằng test. Các task primitive phía sau mới có test đỏ trước.

**Files:**
- Modify: `frontend/src/index.css:30-31` (primary), `:57-60` (success), `:74,87,88` (rgba), `:137-143` (shadow)

- [ ] **Step 1: Đổi màu primary sang giá trị Stitch**

Trong `frontend/src/index.css`, thay hai dòng 30–31:

```css
  --color-primary: #2563EB;
  --color-primary-hover: #1d4ed8;
```

thành:

```css
  --color-primary: #004AC6;
  --color-primary-hover: #003EA8;
```

Không đụng `--color-primary-container: #2563eb` — theo Stitch đó đúng vai của nó.

- [ ] **Step 2: Thêm hai token đang thiếu**

Ngay sau dòng `--color-on-primary-container: #eeefff;`, thêm:

```css
  --color-on-primary-fixed-variant: #003EA8;
```

Ngay sau dòng `--color-success-bg: rgba(22, 163, 74, 0.08);`, thêm:

```css
  --color-on-success: #ffffff;
```

`on-success` là bug sống: `ApprovalDetailPanel.tsx:142` và `BatchActionBar.tsx:39` dùng `text-on-success` mà token chưa từng tồn tại.

- [ ] **Step 3: Thêm token bóng đổ card và ba token bóng nhấn**

Sau dòng `--shadow-raised: 0 2px 8px -2px rgba(0, 0, 0, 0.08);`, thêm:

```css
  --shadow-card: 0 4px 16px rgba(0, 0, 0, 0.04);

  /* Bóng nhuốm màu nhấn. Tồn tại để 5 chỗ đang nướng cứng rgba(37,99,235)
     có đích chuyển sang mà không phải viết giá trị tuỳ tiện. */
  --shadow-accent-sm: 0 2px 12px -2px color-mix(in srgb, var(--color-primary) 20%, transparent);
  --shadow-accent: 0 4px 16px color-mix(in srgb, var(--color-primary) 20%, transparent);
  --shadow-accent-lg: 0 8px 24px color-mix(in srgb, var(--color-primary) 8%, transparent);
```

`--shadow-card` là con số Stitch quy định cho card; thang hiện tại không có giá trị nào khớp.

- [ ] **Step 4: Bỏ mã RGB của primary cũ khỏi 3 token**

Thay ba dòng 74, 87, 88:

```css
  --color-bg-active: rgba(37, 99, 235, 0.08);
  --color-accent-bg: rgba(37, 99, 235, 0.08);
  --color-accent-glow: rgba(37, 99, 235, 0.15);
```

thành:

```css
  --color-bg-active: color-mix(in srgb, var(--color-primary) 8%, transparent);
  --color-accent-bg: color-mix(in srgb, var(--color-primary) 8%, transparent);
  --color-accent-glow: color-mix(in srgb, var(--color-primary) 15%, transparent);
```

Nếu để nguyên, ba token này giữ màu xanh cũ trong khi phần còn lại đã đổi — app sẽ có hai sắc xanh.

- [ ] **Step 5: Kiểm tra `index.css` sạch mã màu cũ**

Chạy: `cd frontend && grep -c '37, 99, 235' src/index.css`
Kết quả mong đợi: `0`

- [ ] **Step 6: Kiểm tra bảy token mới có mặt**

Chạy:
```bash
cd frontend && grep -c -e '--color-primary: #004AC6' -e '--color-on-primary-fixed-variant' -e '--color-on-success' -e '--shadow-card' -e '--shadow-accent-sm' -e '--shadow-accent:' -e '--shadow-accent-lg' src/index.css
```
Kết quả mong đợi: `7`

- [ ] **Step 7: Biên dịch**

Chạy: `cd frontend && npm run build`
Kết quả mong đợi: build thành công, không lỗi.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/index.css
git commit -m "feat(tokens): align colour tokens with the Stitch source

Primary was #2563EB, which Stitch calls primary-container; the brand anchor
is Royal Blue #004AC6, confirmed by .stitch/metadata.json customColor.

Adds two tokens the code already referenced but index.css never defined:
on-primary-fixed-variant (needed by the sidebar active state Stitch specifies)
and on-success, whose absence left two Approve buttons with no text colour.

Three tokens baked rgba(37, 99, 235) inline and would have kept the old blue
after the change; they now derive from the token via color-mix."
```

---

## Task 2: Bỏ mã màu primary cũ khỏi `.tsx`

**Files:**
- Modify: `frontend/src/routes/_auth/workspace-select.tsx:61,170`
- Modify: `frontend/src/components/patterns/MessageItem.tsx:32,35`
- Modify: `frontend/src/components/chat/MessageList.tsx:121`

- [ ] **Step 1: Xác nhận đúng 5 chỗ trong `.tsx`**

Chạy: `cd frontend && grep -rno '37,\s*99,\s*235' --include='*.tsx' src/ | wc -l`
Kết quả mong đợi: `5`

Mỗi chỗ chuyển sang **lớp token**, không phải sang `color-mix` trong ngoặc vuông. Viết `shadow-[0_4px_16px_color-mix(...)]` sẽ hết hardcode màu nhưng vẫn là giá trị tuỳ tiện — đổi một vi phạm lấy một vi phạm.

- [ ] **Step 2: Sửa `workspace-select.tsx`**

Dòng 61, thay `shadow-[0_4px_16px_rgba(37,99,235,0.2)]` bằng `shadow-accent`.

Dòng 170, thay `hover:shadow-[0_8px_24px_rgba(37,99,235,0.08)]` bằng `hover:shadow-accent-lg`.

- [ ] **Step 3: Sửa `MessageItem.tsx`**

Dòng 35 là class thật, dòng 32 là comment mô tả chính class đó — sửa cả hai cho khớp nhau. Thay `shadow-[0_2px_12px_-2px_rgba(37,99,235,0.2)]` bằng `shadow-accent-sm`.

- [ ] **Step 4: Sửa `MessageList.tsx` dòng 121**

Mở file, tìm `rgba(37,99,235` trong className của bong bóng tin nhắn. Chọn token theo hình dạng bóng đang có: offset nhỏ và có giá trị spread âm → `shadow-accent-sm`; offset trung bình → `shadow-accent`; offset lớn và mờ nhạt → `shadow-accent-lg`.

Nếu hình dạng không khớp token nào trong ba, **dừng và báo** — thêm token thứ tư là quyết định thiết kế, không phải việc di cư.

- [ ] **Step 5: Kiểm tra toàn repo sạch**

Chạy: `cd frontend && grep -rn '37,\s*99,\s*235' src/ | wc -l`
Kết quả mong đợi: `0`

Đây là điều kiện nghiệm thu số 3 của spec.

- [ ] **Step 6: Biên dịch**

Chạy: `cd frontend && npm run build`
Kết quả mong đợi: build thành công.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/routes/_auth/workspace-select.tsx frontend/src/components/patterns/MessageItem.tsx frontend/src/components/chat/MessageList.tsx
git commit -m "fix: derive accent shadows from the primary token

Five shadows hardcoded rgba(37, 99, 235), the old primary. They would have
stayed the old blue after the token change, leaving the app with two blues."
```

---

## Task 3: `Button` — hiệu chỉnh theo Stitch

**Files:**
- Create: `frontend/src/components/primitives/Button.test.tsx`
- Modify: `frontend/src/components/primitives/Button.tsx`

- [ ] **Step 1: Viết test đỏ**

Tạo `frontend/src/components/primitives/Button.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { Button } from './Button'

describe('Button — hình học Stitch', () => {
  it('size md dùng px-4 py-2, bo góc 8px, chữ 14/600', () => {
    render(<Button>Lưu</Button>)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('px-4')
    expect(cls).toContain('py-2')
    expect(cls).toContain('rounded-md')
    expect(cls).toContain('text-body-strong')
  })

  it('size sm là bản thu gọn của dự án, không phải của Stitch', () => {
    render(<Button size="sm">Lưu</Button>)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('px-3')
    expect(cls).toContain('py-1.5')
    expect(cls).toContain('text-small-ui')
  })

  it('size cta là nút tràn chiều rộng của Stitch: py-3, bo góc 12px', () => {
    render(<Button size="cta">Tiếp tục</Button>)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('w-full')
    expect(cls).toContain('py-3')
    expect(cls).toContain('rounded-lg')
  })
})

describe('Button — bộ variant sau khi gộp', () => {
  it('primary dùng nền primary đặc', () => {
    render(<Button variant="primary">Lưu</Button>)
    expect(screen.getByRole('button').className).toContain('bg-primary')
  })

  it('secondary là dạng có viền — Stitch gọi nó là Secondary/Outlined', () => {
    render(<Button variant="secondary">Huỷ</Button>)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('border')
    expect(cls).toContain('border-outline-variant')
  })

  it('danger là nền đặc, không phải nền nhạt', () => {
    render(<Button variant="danger">Xoá</Button>)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('bg-error')
    expect(cls).toContain('text-on-error')
  })

  it('success là nền đặc — đúng cái 2 màn hình đang tự chế', () => {
    render(<Button variant="success">Duyệt</Button>)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('bg-success')
    expect(cls).toContain('text-on-success')
  })

  it('ghost trong suốt', () => {
    render(<Button variant="ghost">Bỏ qua</Button>)
    expect(screen.getByRole('button').className).toContain('bg-transparent')
  })
})

describe('Button — hành vi', () => {
  it('loading khoá nút và hiện spinner', () => {
    render(<Button loading>Lưu</Button>)
    const btn = screen.getByRole('button')
    expect(btn).toBeDisabled()
    expect(btn.querySelector('svg')).not.toBeNull()
  })
})
```

- [ ] **Step 2: Chạy test, xác nhận nó đỏ**

Chạy: `cd frontend && npx vitest run src/components/primitives/Button.test.tsx`
Kết quả mong đợi: FAIL. Các test size `md`/`sm`/`cta` và variant `danger`/`success` đều hỏng — hiện `md` là `px-3 py-1`, `cta` chưa tồn tại, `danger` là nền nhạt.

- [ ] **Step 3: Viết lại `Button.tsx`**

Thay toàn bộ nội dung `frontend/src/components/primitives/Button.tsx`:

```tsx
import { type ButtonHTMLAttributes, forwardRef } from 'react'

type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'success'
type ButtonSize = 'sm' | 'md' | 'cta'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  size?: ButtonSize
  loading?: boolean
}

const variantStyles: Record<ButtonVariant, string> = {
  primary: 'bg-primary text-on-primary hover:bg-primary-hover',
  secondary: 'bg-surface-container-lowest text-on-surface border border-outline-variant hover:bg-surface-container-high',
  ghost: 'bg-transparent text-on-surface-variant hover:text-on-surface hover:bg-surface-container',
  danger: 'bg-error text-on-error hover:bg-error/90',
  success: 'bg-success text-on-success hover:bg-success/90',
}

/**
 * Thang size theo `.stitch/DESIGN.md` §Buttons:
 *   md  — Stitch "Primary/Secondary": py-2 px-4, bo 8px, chữ 14/20 w600
 *   cta — Stitch "Full-Width CTA": py-3, bo 12px
 *   sm  — KHÔNG có trong Stitch. Phần mở rộng của dự án cho thanh công cụ và
 *         hàng bảng; 22 chỗ cần tới nó. Xem spec 2026-08-01 §4.2.
 *
 * Bo góc ánh xạ theo pixel Stitch nêu, không theo tên class: thang token của
 * repo là sm 4 / md 8 / lg 12 / xl 16, nên "rounded-lg (8px)" của Stitch là
 * `rounded-md` ở đây. Xem spec §4.1.
 */
const sizeStyles: Record<ButtonSize, string> = {
  sm: 'px-3 py-1.5 rounded-md text-small-ui',
  md: 'px-4 py-2 rounded-md text-body-strong',
  cta: 'w-full py-3 rounded-lg text-body-strong',
}

/** Flat button primitive — Material 3 surface tokens, no gradients or glow effects. */
export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = 'primary', size = 'md', loading, className = '', children, disabled, ...props }, ref) => (
    <button
      ref={ref}
      disabled={disabled || loading}
      className={`inline-flex items-center justify-center gap-2
        transition-colors duration-fast cursor-pointer border-none
        disabled:opacity-40 disabled:cursor-not-allowed focus-ring
        ${variantStyles[variant]} ${sizeStyles[size]} ${className}`}
      {...props}
    >
      {loading && (
        <svg className="animate-spin -ml-1 mr-1 h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
        </svg>
      )}
      {children}
    </button>
  ),
)

Button.displayName = 'Button'
```

- [ ] **Step 4: Chạy test, xác nhận xanh**

Chạy: `cd frontend && npx vitest run src/components/primitives/Button.test.tsx`
Kết quả mong đợi: PASS, 9 test.

- [ ] **Step 5: Để trình biên dịch chỉ ra các call site vỡ**

Chạy:
```bash
cd frontend && npm run typecheck 2>&1 | grep -E '"(outline|error)"' | grep -c 'ButtonVariant'
```
Kết quả mong đợi: `5` — bốn chỗ `variant="outline"` và một chỗ `variant="error"`.

Dùng `typecheck` chứ không phải `typecheck:diff` ở bước này: ta đang **cố ý** tạo lỗi để trình biên dịch chỉ đường. Không đòi sạch lỗi vì dự án có 123 lỗi type có sẵn — xem Task 0.

Nếu số khác 5, dừng lại và đọc kỹ: nghĩa là còn call site chưa được khảo sát.

- [ ] **Step 6: Sửa 4 chỗ `variant="outline"` → `secondary`**

- `frontend/src/routes/_workspace/drive.tsx:206`
- `frontend/src/components/drive/MoveItemDialog.tsx:58`
- `frontend/src/components/composites/ConfirmDialog.tsx:108`
- `frontend/src/components/drive/DrivePreviewDialog.tsx:70`

Ở mỗi chỗ đổi `variant="outline"` thành `variant="secondary"`.

**Không** đụng `AlertBanner variant="error"` ở `ConfirmDialog.tsx:102` — `AlertBanner` có kiểu `AlertVariant` riêng, không liên quan.

- [ ] **Step 7: Sửa 1 chỗ `variant="error"` → `danger`**

`frontend/src/components/approval/ApprovalDetailPanel.tsx:148`: đổi `variant="error"` thành `variant="danger"`.

- [ ] **Step 8: Chuyển 2 nút Approve tự chế sang `variant="success"`**

`frontend/src/components/approval/ApprovalDetailPanel.tsx:142` — bỏ `bg-success text-on-success hover:bg-success/90` khỏi `className`, chỉ giữ `flex-1`, và thêm `variant="success"` vào thẻ `<Button>`.

`frontend/src/components/approval/BatchActionBar.tsx:39` — bỏ `className="bg-success text-on-success hover:bg-success/90"` và thêm `variant="success"`.

- [ ] **Step 9: Biên dịch sạch**

Chạy: `cd frontend && npm run typecheck:diff && npm run build`
Kết quả mong đợi: không file nào tăng lỗi type; build thành công.

- [ ] **Step 10: Chạy toàn bộ test**

Chạy: `cd frontend && npm test`
Kết quả mong đợi: PASS.

- [ ] **Step 11: Commit**

```bash
git add frontend/src/components/primitives/Button.tsx frontend/src/components/primitives/Button.test.tsx frontend/src/routes/_workspace/drive.tsx frontend/src/components/drive/MoveItemDialog.tsx frontend/src/components/composites/ConfirmDialog.tsx frontend/src/components/drive/DrivePreviewDialog.tsx frontend/src/components/approval/ApprovalDetailPanel.tsx frontend/src/components/approval/BatchActionBar.tsx
git commit -m "feat(primitives): calibrate Button to the Stitch geometry

No Button size matched what the screens actually render — md was px-3 py-1
against px-4 py-2.5 on screen — which is why 82% of files hand-rolled their
own buttons instead. The scale now follows Stitch, with radii mapped by the
pixel values Stitch states rather than by its class names, since this repo
shifts the radius scale by one step.

Merges outline into secondary, which Stitch defines as a single
Secondary/Outlined role, and error into danger. Keeps success but makes it
the solid fill two approval screens were already hand-rolling; deleting it
would have left four inconsistencies no lint rule can see."
```

---

## Task 4: `IconButton` — thêm variant và tone

52 nút icon thô đang chờ primitive này. Khảo sát cho thấy chúng cần ba dạng: trong suốt (mặc định), có viền (Stitch quy định), và nền đặc (nút gửi tin nhắn) — cộng với bốn sắc thái ngữ nghĩa.

**Files:**
- Create: `frontend/src/components/primitives/IconButton.test.tsx`
- Modify: `frontend/src/components/primitives/IconButton.tsx`

- [ ] **Step 1: Viết test đỏ**

Tạo `frontend/src/components/primitives/IconButton.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { IconButton } from './IconButton'

describe('IconButton — variant', () => {
  it('mặc định là ghost, nền trong suốt', () => {
    render(<IconButton aria-label="Đóng" />)
    expect(screen.getByRole('button').className).toContain('bg-transparent')
  })

  it('outlined có viền theo Stitch §Buttons', () => {
    render(<IconButton aria-label="Lọc" variant="outlined" />)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('border')
    expect(cls).toContain('border-outline-variant')
  })

  it('filled dùng nền primary đặc', () => {
    render(<IconButton aria-label="Gửi" variant="filled" />)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('bg-primary')
    expect(cls).toContain('text-on-primary')
  })
})

describe('IconButton — tone ngữ nghĩa', () => {
  it('tone success tô chữ màu success', () => {
    render(<IconButton aria-label="Duyệt" tone="success" />)
    expect(screen.getByRole('button').className).toContain('text-success')
  })

  it('tone danger tô chữ màu error', () => {
    render(<IconButton aria-label="Từ chối" tone="danger" />)
    expect(screen.getByRole('button').className).toContain('text-error')
  })

  it('tone mặc định dùng màu outline trung tính', () => {
    render(<IconButton aria-label="Thêm" />)
    expect(screen.getByRole('button').className).toContain('text-outline')
  })
})

describe('IconButton — kích thước', () => {
  it('mặc định là md, ô vuông 8 đơn vị', () => {
    render(<IconButton aria-label="Đóng" />)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('w-8')
    expect(cls).toContain('h-8')
  })
})
```

- [ ] **Step 2: Chạy test, xác nhận nó đỏ**

Chạy: `cd frontend && npx vitest run src/components/primitives/IconButton.test.tsx`
Kết quả mong đợi: FAIL — `variant` và `tone` chưa tồn tại.

- [ ] **Step 3: Viết lại `IconButton.tsx`**

Thay toàn bộ nội dung `frontend/src/components/primitives/IconButton.tsx`:

```tsx
import { type ButtonHTMLAttributes, forwardRef } from 'react'

type IconButtonSize = 'sm' | 'md' | 'lg'
type IconButtonVariant = 'ghost' | 'outlined' | 'filled'
type IconButtonTone = 'default' | 'primary' | 'success' | 'danger'

interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  size?: IconButtonSize
  variant?: IconButtonVariant
  tone?: IconButtonTone
}

const sizeStyles: Record<IconButtonSize, string> = {
  sm: 'w-7 h-7',
  md: 'w-8 h-8',
  lg: 'w-9 h-9',
}

/**
 * `outlined` đến từ Stitch §Buttons ("Icon Button: p-2 rounded-lg border").
 * `filled` là phần mở rộng của dự án cho nút gửi tin nhắn — không có trong Stitch.
 */
const variantStyles: Record<IconButtonVariant, string> = {
  ghost: 'bg-transparent border-none',
  outlined: 'bg-transparent border border-outline-variant',
  filled: 'bg-primary text-on-primary border-none hover:bg-primary-hover',
}

const toneStyles: Record<IconButtonTone, string> = {
  default: 'text-outline hover:text-on-surface hover:bg-surface-container',
  primary: 'text-primary hover:bg-primary/10',
  success: 'text-success hover:bg-success-bg',
  danger: 'text-error hover:bg-error/10',
}

/** Icon-only button with Material 3 surface tokens. */
export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(
  ({ size = 'md', variant = 'ghost', tone = 'default', className = '', ...props }, ref) => (
    <button
      ref={ref}
      className={`inline-flex items-center justify-center rounded-md
        transition-colors duration-fast cursor-pointer focus-ring
        disabled:opacity-40 disabled:cursor-not-allowed
        ${variantStyles[variant]}
        ${variant === 'filled' ? '' : toneStyles[tone]}
        ${sizeStyles[size]} ${className}`}
      {...props}
    />
  ),
)

IconButton.displayName = 'IconButton'
```

`filled` tự mang màu nên bỏ qua `tone` — nếu không, `text-on-primary` sẽ bị `text-outline` của tone mặc định ghi đè.

- [ ] **Step 4: Chạy test, xác nhận xanh**

Chạy: `cd frontend && npx vitest run src/components/primitives/IconButton.test.tsx`
Kết quả mong đợi: PASS, 7 test.

- [ ] **Step 5: Chuyển 2 nút icon tự chế sang `tone`**

`frontend/src/routes/_workspace/approval.tsx:199` — bỏ `className="text-success hover:bg-success/10"`, thêm `tone="success"`.

`frontend/src/components/approval/ApprovalTable.tsx:99` — bỏ `className="text-success hover:bg-success-bg"`, thêm `tone="success"`.

- [ ] **Step 6: Biên dịch và chạy test**

Chạy: `cd frontend && npm run typecheck:diff && npm test`
Kết quả mong đợi: không file nào tăng lỗi type; test xanh.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/components/primitives/IconButton.tsx frontend/src/components/primitives/IconButton.test.tsx frontend/src/routes/_workspace/approval.tsx frontend/src/components/approval/ApprovalTable.tsx
git commit -m "feat(primitives): give IconButton the forms the screens need

It had one appearance — transparent — so anything else was hand-rolled:
Stitch's bordered icon button, the filled send button, and semantic tinting
for approve and reject actions. Adds variant and tone to cover all four,
and moves the two hand-tinted approve buttons onto tone."
```

---

## Task 5: `Input` — thêm dạng pill tìm kiếm

Đây là mảnh từ vựng thiếu gây ra 26 `<input>` thô.

**Files:**
- Create: `frontend/src/components/primitives/Input.test.tsx`
- Modify: `frontend/src/components/primitives/Input.tsx`

- [ ] **Step 1: Viết test đỏ**

Tạo `frontend/src/components/primitives/Input.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { Input } from './Input'

describe('Input — dạng mặc định', () => {
  it('là ô chữ nhật trên nền nổi', () => {
    render(<Input aria-label="Tên" />)
    const cls = screen.getByRole('textbox').className
    expect(cls).toContain('rounded-md')
    expect(cls).toContain('bg-surface-container-lowest')
  })
})

describe('Input — dạng tìm kiếm theo Stitch §Inputs', () => {
  it('là pill trên nền surface-container-low', () => {
    render(<Input aria-label="Tìm" variant="search" />)
    const cls = screen.getByRole('textbox').className
    expect(cls).toContain('rounded-full')
    expect(cls).toContain('bg-surface-container-low')
  })

  it('không mang bo góc chữ nhật của dạng mặc định', () => {
    render(<Input aria-label="Tìm" variant="search" />)
    expect(screen.getByRole('textbox').className).not.toContain('rounded-md')
  })
})

describe('Input — trạng thái lỗi', () => {
  it('hiện thông báo lỗi và tô viền error', () => {
    render(<Input aria-label="Email" error="Email không hợp lệ" />)
    expect(screen.getByText('Email không hợp lệ')).toBeInTheDocument()
    expect(screen.getByRole('textbox').className).toContain('border-danger')
  })

  it('helpText bị lỗi che khi cả hai cùng có', () => {
    render(<Input aria-label="Email" error="Sai định dạng" helpText="Dùng email công ty" />)
    expect(screen.getByText('Sai định dạng')).toBeInTheDocument()
    expect(screen.queryByText('Dùng email công ty')).toBeNull()
  })
})
```

- [ ] **Step 2: Chạy test, xác nhận nó đỏ**

Chạy: `cd frontend && npx vitest run src/components/primitives/Input.test.tsx`
Kết quả mong đợi: FAIL ở hai test dạng tìm kiếm — prop `variant` chưa tồn tại.

- [ ] **Step 3: Viết lại `Input.tsx`**

Thay toàn bộ nội dung `frontend/src/components/primitives/Input.tsx`:

```tsx
import { type InputHTMLAttributes, forwardRef } from 'react'

type InputVariant = 'default' | 'search'

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  error?: string
  helpText?: string
  variant?: InputVariant
}

/** `search` theo Stitch §Inputs: pill trên nền surface-container-low. */
const variantStyles: Record<InputVariant, string> = {
  default: 'rounded-md bg-surface-container-lowest',
  search: 'rounded-full bg-surface-container-low',
}

/** Recessed input — surface-container-lowest background for visual depth on elevated surfaces. */
export const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ className = '', error, helpText, variant = 'default', ...props }, ref) => (
    <div className="flex flex-col gap-1">
      <input
        ref={ref}
        className={`w-full px-3 py-2 text-small border
          text-on-surface placeholder:text-outline
          transition-colors duration-fast focus-ring
          ${variantStyles[variant]}
          ${error
            ? 'border-danger focus:border-danger'
            : 'border-outline-variant focus:border-primary'
          } ${className}`}
        {...props}
      />
      {error && <span className="text-micro text-danger">{error}</span>}
      {helpText && !error && <span className="text-micro text-outline">{helpText}</span>}
    </div>
  ),
)

Input.displayName = 'Input'
```

- [ ] **Step 4: Chạy test, xác nhận xanh**

Chạy: `cd frontend && npx vitest run src/components/primitives/Input.test.tsx`
Kết quả mong đợi: PASS, 5 test.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/primitives/Input.tsx frontend/src/components/primitives/Input.test.tsx
git commit -m "feat(primitives): add the search pill Input Stitch specifies

Stitch defines search fields as pills on surface-container-low; Input only
had the rectangular form, which is what the 26 raw <input> elements exist to
work around."
```

---

## Task 6: `Textarea` — bỏ giá trị pixel tuỳ tiện

Đây là vi phạm duy nhất của chặng 0 theo bảng chặng trong spec.

**Files:**
- Modify: `frontend/src/components/primitives/Textarea.tsx:15`

- [ ] **Step 1: Xác nhận vi phạm tồn tại**

Chạy: `cd frontend && grep -n 'min-h-\[72px\]' src/components/primitives/Textarea.tsx`
Kết quả mong đợi: khớp ở dòng 15.

- [ ] **Step 2: Thay bằng giá trị theo thang Tailwind**

Trong `frontend/src/components/primitives/Textarea.tsx`, thay `min-h-[72px]` bằng `min-h-18`.

Thang spacing của Tailwind là bội số 4px, nên `min-h-18` = 72px — cùng chiều cao, không còn giá trị tuỳ tiện.

- [ ] **Step 3: Xác nhận primitive sạch**

Chạy: `cd frontend && grep -rno '\[[0-9]\+px\]' src/components/primitives/ | wc -l`
Kết quả mong đợi: `0`

- [ ] **Step 4: Biên dịch**

Chạy: `cd frontend && npm run build`
Kết quả mong đợi: build thành công.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/primitives/Textarea.tsx
git commit -m "refactor(primitives): put Textarea's min height on the spacing scale

min-h-[72px] is the same 72px as min-h-18 on Tailwind's 4px scale, but as an
arbitrary value it would trip the lint rule the primitives are meant to model."
```

---

## Task 7: `NavRow` — primitive mới cho dòng điều hướng

~16 chỗ đang dùng `<button>` để render dòng sidebar và node cây. Chúng không phải nút bấm theo nghĩa design system, nên ép vào `Button` là sai cả ngữ nghĩa lẫn hình dạng.

**Files:**
- Create: `frontend/src/components/primitives/NavRow.tsx`
- Create: `frontend/src/components/primitives/NavRow.test.tsx`
- Modify: `frontend/src/components/primitives/index.ts`

- [ ] **Step 1: Viết test đỏ**

Tạo `frontend/src/components/primitives/NavRow.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi } from 'vitest'
import { NavRow } from './NavRow'

describe('NavRow — trạng thái theo Stitch §Navigation Sidebar', () => {
  it('sub-item đang active dùng nền primary-fixed và chữ on-primary-fixed-variant', () => {
    render(<NavRow kind="subItem" active>Kỹ thuật</NavRow>)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('bg-primary-fixed')
    expect(cls).toContain('text-on-primary-fixed-variant')
  })

  it('group header đang active dùng nền surface-container và chữ primary', () => {
    render(<NavRow kind="groupHeader" active>Phòng ban</NavRow>)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('bg-surface-container')
    expect(cls).toContain('text-primary')
  })

  it('trạng thái không active dùng chữ on-surface-variant', () => {
    render(<NavRow kind="subItem">Kinh doanh</NavRow>)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('text-on-surface-variant')
    expect(cls).not.toContain('bg-primary-fixed')
  })

  it('sub-item thụt lề pl-9 pr-2 đúng Stitch', () => {
    render(<NavRow kind="subItem">Kỹ thuật</NavRow>)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('pl-9')
    expect(cls).toContain('pr-2')
  })

  it('group header không thụt lề như sub-item', () => {
    render(<NavRow kind="groupHeader">Phòng ban</NavRow>)
    expect(screen.getByRole('button').className).not.toContain('pl-9')
  })
})

describe('NavRow — hành vi', () => {
  it('bấm thì gọi onClick', async () => {
    const onClick = vi.fn()
    render(<NavRow kind="subItem" onClick={onClick}>Kỹ thuật</NavRow>)
    await userEvent.click(screen.getByRole('button'))
    expect(onClick).toHaveBeenCalledOnce()
  })

  it('active được phơi ra cho trình đọc màn hình', () => {
    render(<NavRow kind="subItem" active>Kỹ thuật</NavRow>)
    expect(screen.getByRole('button')).toHaveAttribute('aria-current', 'page')
  })

  it('không active thì không đặt aria-current', () => {
    render(<NavRow kind="subItem">Kỹ thuật</NavRow>)
    expect(screen.getByRole('button')).not.toHaveAttribute('aria-current')
  })
})
```

- [ ] **Step 2: Chạy test, xác nhận nó đỏ**

Chạy: `cd frontend && npx vitest run src/components/primitives/NavRow.test.tsx`
Kết quả mong đợi: FAIL với lỗi không phân giải được `./NavRow`.

- [ ] **Step 3: Tạo `NavRow.tsx`**

Tạo `frontend/src/components/primitives/NavRow.tsx`:

```tsx
import { type ButtonHTMLAttributes, forwardRef } from 'react'

type NavRowKind = 'groupHeader' | 'subItem'

interface NavRowProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  kind: NavRowKind
  active?: boolean
}

/**
 * Dòng điều hướng bấm được — sidebar item và node cây.
 * Không phải `Button`: đây là phần tử điều hướng, không phải nút hành động.
 * Mọi trạng thái lấy từ `.stitch/DESIGN.md` §Navigation Sidebar.
 */
const layoutStyles: Record<NavRowKind, string> = {
  groupHeader: 'px-3 py-2 text-body-ui',
  subItem: 'pl-9 pr-2 py-1.5 text-small-ui',
}

const activeStyles: Record<NavRowKind, string> = {
  groupHeader: 'bg-surface-container text-primary font-medium',
  subItem: 'bg-primary-fixed text-on-primary-fixed-variant font-medium',
}

const inactiveStyles = 'text-on-surface-variant hover:bg-surface-container-low'

export const NavRow = forwardRef<HTMLButtonElement, NavRowProps>(
  ({ kind, active = false, className = '', children, ...props }, ref) => (
    <button
      ref={ref}
      aria-current={active ? 'page' : undefined}
      className={`w-full flex items-center gap-2 rounded-md text-left
        transition-colors duration-fast cursor-pointer border-none focus-ring
        ${layoutStyles[kind]}
        ${active ? activeStyles[kind] : inactiveStyles}
        ${className}`}
      {...props}
    >
      {children}
    </button>
  ),
)

NavRow.displayName = 'NavRow'
```

- [ ] **Step 4: Chạy test, xác nhận xanh**

Chạy: `cd frontend && npx vitest run src/components/primitives/NavRow.test.tsx`
Kết quả mong đợi: PASS, 8 test.

- [ ] **Step 5: Export từ barrel**

Trong `frontend/src/components/primitives/index.ts`, thêm sau dòng `export { IconButton } from './IconButton'`:

```ts
export { NavRow } from './NavRow'
```

- [ ] **Step 6: Biên dịch và chạy toàn bộ test**

Chạy: `cd frontend && npm run typecheck:diff && npm test`
Kết quả mong đợi: không file nào tăng lỗi type; toàn bộ test xanh.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/components/primitives/NavRow.tsx frontend/src/components/primitives/NavRow.test.tsx frontend/src/components/primitives/index.ts
git commit -m "feat(primitives): add NavRow for navigation rows

Around 16 sidebar rows and tree nodes are written as raw <button>. They are
navigation, not actions, so forcing them into Button would be wrong in both
semantics and shape. Every state comes from Stitch's sidebar section,
including the active sub-item colour that needed the on-primary-fixed-variant
token added in the token commit."
```

---

## Task 8: Bật ESLint cho TypeScript với 4 rule design system

**Files:**
- Modify: `frontend/package.json`
- Modify: `frontend/eslint.config.js`

- [ ] **Step 1: Kiểm tra bản `typescript-eslint` tương thích ESLint 10**

Chạy: `cd frontend && npm info typescript-eslint peerDependencies`
Kết quả mong đợi: in ra ràng buộc peer với `eslint`.

Nếu ràng buộc **không** bao gồm `^10`, dừng lại và dùng phương án dự phòng ở spec §6.2 — cài riêng `@typescript-eslint/parser` thay cho gói meta. Bốn rule đều nằm trong ESLint lõi nên chỉ cần parser.

- [ ] **Step 2: Cài đặt**

Chạy: `cd frontend && npm install -D typescript-eslint`
Kết quả mong đợi: cài xong, không lỗi peer dependency.

- [ ] **Step 3: Viết lại `eslint.config.js`**

Thay toàn bộ nội dung `frontend/eslint.config.js`:

```js
import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import tseslint from 'typescript-eslint'
import { defineConfig, globalIgnores } from 'eslint/config'

/**
 * Rule design system. Xem docs/superpowers/specs/2026-08-01-ui-design-system-enforcement-design.md
 *
 * Chủ đích chỉ dùng parser của typescript-eslint, KHÔNG dùng bộ rule recommended:
 * bộ đó đổ ra hàng trăm lỗi unused-vars/any/import-order không liên quan và sẽ
 * chôn vùi tín hiệu design system. Siết TS hygiene là việc riêng. Xem spec §4.3.
 */
const designSystemRules = [
  {
    // Rule 1 — thẻ tương tác thô. Override cho phép trong chính primitives/.
    selector: "JSXOpeningElement[name.name=/^(button|input|textarea|select)$/]",
    message:
      'Dùng primitive (Button/IconButton/Input/Select/Textarea/NavRow) thay thẻ thô. Chỉ components/primitives/ được render thẻ HTML trực tiếp.',
  },
  {
    // Rule 2 — chỉ chặn số đo pixel/rem, KHÔNG chặn [...] nói chung:
    // DESIGN.md quy định lưới bảng bằng grid-cols-[auto_1.5fr_1fr_1.5fr_1fr].
    selector: "JSXAttribute[name.name='className'] Literal[value=/\\[\\d+(px|rem)\\]/]",
    message: 'Giá trị pixel tuỳ tiện. Dùng thang spacing/radius trong index.css.',
  },
  {
    selector: "JSXAttribute[name.name='className'] TemplateElement[value.raw=/\\[\\d+(px|rem)\\]/]",
    message: 'Giá trị pixel tuỳ tiện. Dùng thang spacing/radius trong index.css.',
  },
  {
    // Rule 3 — mọi string literal, không chỉ className: phần lớn hex thật nằm ở
    // prop color= và trong object màu ở .ts.
    selector: 'Literal[value=/#[0-9a-fA-F]{6}/]',
    message: 'Hex nướng cứng. Dùng token màu trong index.css.',
  },
  {
    // Rule 4 — bảng màu Tailwind thô; token M3 đã thay thế.
    selector:
      "JSXAttribute[name.name='className'] Literal[value=/\\b(bg|text|border)-(slate|gray|zinc|neutral|red|orange|green|blue|indigo)-\\d{2,3}\\b/]",
    message: 'Bảng màu Tailwind thô. Dùng token M3 hoặc --color-status-*.',
  },
  {
    // Rule 5 — bóng đổ ngoài thang. Rule 2 KHÔNG bắt được nhóm này: nó đòi cả
    // cụm trong ngoặc là \d+(px|rem), nên shadow-[0_4px_16px_rgba(...)] lọt qua
    // dù bên trong có 4px. Thiếu rule này thì 34 shadow tuỳ tiện và 4 shadow-xl
    // vô hình với thước đo tiến độ. Xem spec §4.3.
    selector: "JSXAttribute[name.name='className'] Literal[value=/shadow-(\\[|xl\\b)/]",
    message: 'Bóng đổ ngoài thang. Dùng shadow-sm/md/lg/card/accent* trong index.css.',
  },
  {
    selector: "JSXAttribute[name.name='className'] TemplateElement[value.raw=/shadow-(\\[|xl\\b)/]",
    message: 'Bóng đổ ngoài thang. Dùng shadow-sm/md/lg/card/accent* trong index.css.',
  },
]

export default defineConfig([
  globalIgnores(['dist', 'src/routeTree.gen.ts', 'src/generated']),
  {
    files: ['vite.config.js', 'vitest.config.ts', 'eslint.config.js'],
    languageOptions: { globals: globals.node },
  },
  {
    files: ['**/*.{js,jsx}'],
    extends: [js.configs.recommended, reactHooks.configs.flat.recommended, reactRefresh.configs.vite],
    languageOptions: {
      globals: globals.browser,
      parserOptions: { ecmaFeatures: { jsx: true } },
    },
  },
  {
    files: ['**/*.{ts,tsx}'],
    languageOptions: {
      parser: tseslint.parser,
      globals: globals.browser,
      parserOptions: { ecmaFeatures: { jsx: true } },
    },
    rules: {
      // Mức warn trong suốt quá trình di cư; khoá thành error ở chặng cuối,
      // nếu không repo sẽ đỏ suốt và không phân biệt được lỗi mới với nợ cũ.
      'no-restricted-syntax': ['warn', ...designSystemRules],
    },
  },
  {
    // Nhóm A của spec §1.3 — primitive phải render thẻ HTML thật.
    files: ['src/components/primitives/**/*.tsx'],
    rules: {
      'no-restricted-syntax': [
        'warn',
        ...designSystemRules.filter(r => !r.selector.startsWith('JSXOpeningElement')),
      ],
    },
  },
  {
    // Nhóm B của spec §1.3 — màu là bản sắc nội dung, không phải màu giao diện.
    // fileIcons.ts gán màu cho loại tệp; login.tsx chứa logo thương hiệu Google.
    files: ['src/lib/fileIcons.ts', 'src/routes/_auth/login.tsx'],
    rules: {
      'no-restricted-syntax': [
        'warn',
        ...designSystemRules.filter(r => r.selector !== 'Literal[value=/#[0-9a-fA-F]{6}/]'),
      ],
    },
  },
])
```

- [ ] **Step 4: Chạy lint và ghi lại số nền**

Chạy: `cd frontend && npm run lint 2>&1 | tail -5`
Kết quả mong đợi: lint chạy xong, báo khoảng **340** cảnh báo. Con số chính xác là số nền để đối chiếu ở mọi chặng sau.

Nếu ra 0 cảnh báo, config chưa ăn — kiểm tra lại khối `files: ['**/*.{ts,tsx}']`.

- [ ] **Step 5: Ghi số nền vào plan**

Chạy: `cd frontend && npm run lint 2>&1 | grep -c 'no-restricted-syntax'`

Ghi con số nhận được vào Task 17 Step 2 làm mốc đối chiếu.

- [ ] **Step 6: Xác nhận miễn trừ hoạt động đúng**

Chạy:
```bash
cd frontend && npm run lint 2>&1 | grep -E 'fileIcons|primitives/(Button|Input|Select|Textarea|IconButton|NavRow)\.tsx' | grep -c 'Hex nướng cứng\|thẻ thô'
```
Kết quả mong đợi: `0` — không cảnh báo nào thuộc nhóm được miễn trừ.

- [ ] **Step 7: Commit**

```bash
git add frontend/eslint.config.js frontend/package.json frontend/package-lock.json
git commit -m "build(lint): lint TypeScript and encode the design system as rules

eslint.config.js declared files: ['**/*.{js,jsx}'] while every one of the 107
UI files is .tsx, so npm run lint passed without reading a line of the app.
That is why 113 raw buttons could accumulate unchallenged.

Takes only the parser, not the recommended rule set: the recommended rules
would bury the design-system signal under unused-vars and any. Four
no-restricted-syntax selectors from core ESLint cover raw interactive
elements, arbitrary pixel values, hardcoded hex, and the raw Tailwind
palette. They start at warn so the violation count can serve as the progress
meter, and get locked to error once it reaches zero.

Two exemptions are scoped rather than blanket: primitives may render real
HTML elements, and file-type icon colours plus the Google logo stay hex
because they are content identity, not UI chrome."
```

---

## Task 9: Chụp ảnh nền chặng 0

Chặng 0 đổi diện mạo toàn app mà không file `.tsx` nào nằm trong diff. Review bằng đọc diff là vô hiệu — bắt buộc nhìn.

**Files:** không sửa file nào; task này sinh bằng chứng.

- [ ] **Step 1: Khởi động ứng dụng**

Chạy: `cd /home/zane/Desktop/projects/ngac && make dev`
Kết quả mong đợi: hạ tầng và 8 service Go khởi động, frontend chạy ở `http://localhost:5173`. PID ở `.dev-pids`, log ở `.dev-logs/`.

- [ ] **Step 2: Chụp ba màn hình ở ba bề rộng**

Dùng skill `/browse` để mở và chụp `http://localhost:5173` ở 375px, 768px, 1280px. Chụp tối thiểu: màn đăng nhập, một màn hình có sidebar, một màn hình có bảng dữ liệu.

Lưu vào `docs/superpowers/plans/screenshots/2026-08-01-stage0/`.

- [ ] **Step 3: Đối chiếu bằng mắt**

Xác nhận bốn điều:
1. Nút primary chuyển sang xanh đậm `#004AC6`, không còn chỗ nào xanh sáng cũ
2. Nút dùng `<Button>` sẵn có nay rộng hơn (`px-4 py-2` thay cho `px-3 py-1`)
3. Focus ring vẫn thấy rõ khi tab qua các nút
4. Không có vùng nào mất màu chữ hoặc mất viền

- [ ] **Step 4: Dừng ứng dụng**

Chạy: `cd /home/zane/Desktop/projects/ngac && make dev-stop`

- [ ] **Step 5: Commit ảnh**

```bash
git add docs/superpowers/plans/screenshots/2026-08-01-stage0/
git commit -m "docs: capture the appearance change stage 0 makes invisible in diff

Stage 0 touched CSS and primitives only, yet it restyled 42 existing Button
call sites and repainted every primary surface. Reviewing the diff would show
a few CSS lines and read as a small change, so the evidence has to be visual."
```

---

## Task 10: Chặng 1 — `welcome.tsx`

Từ đây là chặng 1: di cư bề mặt auth. Bắt đầu từ file ít vi phạm nhất để kiểm chứng quy trình.

**Files:**
- Modify: `frontend/src/routes/_auth/welcome.tsx`

- [ ] **Step 1: Xem vi phạm của file**

Chạy: `cd frontend && npx eslint src/routes/_auth/welcome.tsx`
Kết quả mong đợi: 4 cảnh báo — 2 giá trị pixel tuỳ tiện, 2 bóng đổ ngoài thang.

- [ ] **Step 2: Xem chính xác chúng nằm ở đâu**

Chạy: `cd frontend && grep -n '\[[0-9]\+px\]' src/routes/_auth/welcome.tsx`

- [ ] **Step 3: Thay bằng giá trị theo thang**

Với mỗi giá trị `[Npx]`, quy về lớp Tailwind gần nhất trên thang 4px (`N/4` đơn vị). Ví dụ `[16px]` → `4`, `[24px]` → `6`, `[12px]` → `3`.

Nếu giá trị không chia hết cho 4 và việc làm tròn sẽ đổi bố cục thấy được, **dừng lại và báo** — theo spec §6.1, quyết định bố cục không có trong `DESIGN.md` thì không tự đoán.

- [ ] **Step 4: Xác nhận file sạch**

Chạy: `cd frontend && npx eslint src/routes/_auth/welcome.tsx`
Kết quả mong đợi: 0 vấn đề.

- [ ] **Step 5: Biên dịch**

Chạy: `cd frontend && npm run build`
Kết quả mong đợi: build thành công.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/routes/_auth/welcome.tsx
git commit -m "refactor(auth): put welcome screen spacing on the scale"
```

---

## Task 11: Chặng 1 — `AuthLayout.tsx`

**Files:**
- Modify: `frontend/src/components/layouts/AuthLayout.tsx`

- [ ] **Step 1: Xem vi phạm**

Chạy: `cd frontend && npx eslint src/components/layouts/AuthLayout.tsx && grep -n '\[[0-9]\+px\]' src/components/layouts/AuthLayout.tsx`
Kết quả mong đợi: 1 cảnh báo giá trị pixel tuỳ tiện.

- [ ] **Step 2: Thay bằng giá trị theo thang**

Áp cùng quy tắc như Task 10 Step 3. Nếu làm tròn đổi bố cục thấy được, dừng và báo.

- [ ] **Step 3: Xác nhận sạch và biên dịch**

Chạy: `cd frontend && npx eslint src/components/layouts/AuthLayout.tsx && npm run build`
Kết quả mong đợi: 0 vấn đề, build thành công.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/layouts/AuthLayout.tsx
git commit -m "refactor(auth): put AuthLayout spacing on the scale"
```

---

## Task 12: Chặng 1 — `OtpInput.tsx`

**Files:**
- Modify: `frontend/src/components/auth/OtpInput.tsx`

- [ ] **Step 1: Xem vi phạm và ngữ cảnh**

Chạy: `cd frontend && npx eslint src/components/auth/OtpInput.tsx && grep -n '<input' src/components/auth/OtpInput.tsx`
Kết quả mong đợi: 1 cảnh báo thẻ thô.

- [ ] **Step 2: Đọc toàn bộ file trước khi sửa**

Ô OTP thường cần `ref` theo từng ký tự, `maxLength={1}`, và điều hướng bằng phím. `Input` chuyển tiếp `ref` qua `forwardRef` và trải mọi prop còn lại xuống thẻ `<input>`, nên các nhu cầu này giữ nguyên được.

- [ ] **Step 3: Thay `<input>` bằng `<Input>`**

Thêm import:

```tsx
import { Input } from '../primitives'
```

Thay thẻ `<input .../>` bằng `<Input ... />`, giữ nguyên mọi prop.

Lưu ý: `Input` bọc thẻ trong `<div className="flex flex-col gap-1">`. Nếu bố cục OTP dựa vào `<input>` là con trực tiếp của một flex container, hãy chuyển lớp bố cục sang thẻ bọc bằng prop `className` — hoặc nếu không đạt được, **dừng và báo** thay vì bẻ bố cục.

- [ ] **Step 4: Xác nhận sạch**

Chạy: `cd frontend && npx eslint src/components/auth/OtpInput.tsx`
Kết quả mong đợi: 0 vấn đề.

- [ ] **Step 5: Biên dịch và chạy test**

Chạy: `cd frontend && npm run typecheck:diff && npm test`
Kết quả mong đợi: không file nào tăng lỗi type; test xanh.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/auth/OtpInput.tsx
git commit -m "refactor(auth): move the OTP field onto the Input primitive"
```

---

## Task 13: Chặng 1 — `_auth.tsx`

**Files:**
- Modify: `frontend/src/routes/_auth.tsx`

- [ ] **Step 1: Xem vi phạm**

Chạy: `cd frontend && npx eslint src/routes/_auth.tsx`
Kết quả mong đợi: 7 cảnh báo — 2 nút thô, 2 giá trị pixel, 2 hex, 1 bóng đổ ngoài thang.

- [ ] **Step 2: Sửa 2 hex trong gradient nền**

Chạy: `cd frontend && grep -n '#[0-9a-fA-F]\{6\}' src/routes/_auth.tsx`

Dòng 18 chứa `bg-[radial-gradient(...,#dee8ff_0%,...)]` với hai mã hex. Thay mỗi mã bằng biến token tương ứng — `#dee8ff` là giá trị của `--color-surface-container-high`, nên viết `var(--color-surface-container-high)`. Tra mã còn lại trong `src/index.css` và dùng đúng token có cùng giá trị.

Nếu một mã hex không khớp token nào đang có, **dừng và báo** — thêm token mới là quyết định thiết kế, không phải việc di cư.

- [ ] **Step 3: Thay 2 nút thô bằng primitive**

Chạy: `cd frontend && grep -n -A3 '<button' src/routes/_auth.tsx`

Với mỗi nút, chọn primitive theo hình dạng: chỉ có icon → `IconButton`; có chữ và là hành động → `Button`; là dòng điều hướng → `NavRow`. Bỏ các lớp padding/bo góc/cỡ chữ mà primitive đã lo, giữ lại lớp bố cục (`flex-1`, `mt-4`, …) qua `className`.

- [ ] **Step 4: Sửa 2 giá trị pixel**

Áp quy tắc Task 10 Step 3.

- [ ] **Step 5: Xác nhận sạch**

Chạy: `cd frontend && npx eslint src/routes/_auth.tsx`
Kết quả mong đợi: 0 vấn đề.

- [ ] **Step 6: Biên dịch**

Chạy: `cd frontend && npm run typecheck:diff && npm run build`
Kết quả mong đợi: không file nào tăng lỗi type; build thành công.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/routes/_auth.tsx
git commit -m "refactor(auth): move the auth shell onto primitives and tokens"
```

---

## Task 14: Chặng 1 — `workspace-select.tsx`

**Files:**
- Modify: `frontend/src/routes/_auth/workspace-select.tsx`

- [ ] **Step 1: Xem vi phạm**

Chạy: `cd frontend && npx eslint src/routes/_auth/workspace-select.tsx`
Kết quả mong đợi: 9 cảnh báo — 3 nút thô, 4 giá trị pixel, 2 bóng đổ ngoài thang.

Hai shadow ở dòng 61 và 170 đã được xử lý ở Task 2 nên không còn trong danh sách.

- [ ] **Step 2: Thay 3 nút thô**

Chạy: `cd frontend && grep -n -A4 '<button' src/routes/_auth/workspace-select.tsx`

Màn hình này là danh sách workspace bấm được. Thẻ workspace là **hàng chọn**, không phải nút hành động — nếu nó render như một hàng danh sách thì dùng `NavRow kind="subItem"`; nếu là nút hành động thật (Tạo workspace, Đăng xuất) thì dùng `Button`.

- [ ] **Step 3: Sửa 4 giá trị pixel**

Áp quy tắc Task 10 Step 3.

- [ ] **Step 4: Xác nhận sạch và biên dịch**

Chạy: `cd frontend && npx eslint src/routes/_auth/workspace-select.tsx && npm run typecheck:diff && npm run build`
Kết quả mong đợi: 0 vấn đề lint; không file nào tăng lỗi type; build thành công.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/routes/_auth/workspace-select.tsx
git commit -m "refactor(auth): move workspace selection onto primitives"
```

---

## Task 15: Chặng 1 — `onboarding.tsx`

**Files:**
- Modify: `frontend/src/routes/_auth/onboarding.tsx`

- [ ] **Step 1: Xem vi phạm**

Chạy: `cd frontend && npx eslint src/routes/_auth/onboarding.tsx`
Kết quả mong đợi: 11 cảnh báo — 6 nút thô, 3 input thô, 2 bóng đổ ngoài thang.

- [ ] **Step 2: Thay 3 input thô bằng `Input`**

Chạy: `cd frontend && grep -n -A3 '<input' src/routes/_auth/onboarding.tsx`

Thêm `import { Button, Input } from '../../components/primitives'` — file nằm ở `src/routes/_auth/`, nên lùi hai cấp về `src/` rồi vào `components/primitives`.

Ô nào là ô tìm kiếm thì dùng `variant="search"`; còn lại để mặc định.

- [ ] **Step 3: Thay 6 nút thô**

Onboarding là luồng nhiều bước, nên nút "Tiếp tục"/"Hoàn tất" thường tràn chiều rộng — đó chính là `size="cta"` mà Stitch định nghĩa. Nút "Quay lại" dùng `variant="secondary"`. Nút bỏ qua dùng `variant="ghost"`.

- [ ] **Step 4: Xác nhận sạch và biên dịch**

Chạy: `cd frontend && npx eslint src/routes/_auth/onboarding.tsx && npm run typecheck:diff && npm run build`
Kết quả mong đợi: 0 vấn đề lint; không file nào tăng lỗi type; build thành công.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/routes/_auth/onboarding.tsx
git commit -m "refactor(auth): move onboarding onto primitives

The step actions are full-width, which is exactly the CTA size Stitch defines
and the primitive did not have until this branch."
```

---

## Task 16: Chặng 1 — `login.tsx`

File cuối của chặng 1, và là file duy nhất có miễn trừ vĩnh viễn.

**Files:**
- Modify: `frontend/src/routes/_auth/login.tsx`

- [ ] **Step 1: Xem vi phạm**

Chạy: `cd frontend && npx eslint src/routes/_auth/login.tsx`
Kết quả mong đợi: 6 cảnh báo — 5 nút thô, 1 input thô.

**Không** có cảnh báo hex: 4 mã màu logo Google đã được miễn trừ ở config Task 8. Nếu chúng vẫn xuất hiện, override chưa ăn — quay lại Task 8 Step 6.

- [ ] **Step 2: Thay 1 input thô**

Chạy: `cd frontend && grep -n -A3 '<input' src/routes/_auth/login.tsx`

Dùng `Input`, giữ nguyên `type`, `value`, `onChange`, `autoComplete`.

- [ ] **Step 3: Thay 5 nút thô**

Chạy: `cd frontend && grep -n -A5 '<button' src/routes/_auth/login.tsx`

Ánh xạ theo vai trò:
- Nút "Đăng nhập" tràn chiều rộng → `<Button size="cta">`
- Nút "Sign in with Google" → `<Button variant="secondary" size="cta">`, **giữ nguyên khối `<svg>` logo bên trong không sửa gì**
- Liên kết kiểu "Quên mật khẩu" / "Đăng ký" → `<Button variant="ghost" size="sm">`

- [ ] **Step 4: Xác nhận logo Google còn nguyên**

Chạy: `cd frontend && grep -c '#4285F4\|#EA4335\|#FBBC05\|#34A853' src/routes/_auth/login.tsx`
Kết quả mong đợi: `4`

Bốn mã này **phải** còn — chúng là màu thương hiệu, token hoá là làm sai logo. Xem spec §1.3-B.

- [ ] **Step 5: Xác nhận sạch và biên dịch**

Chạy: `cd frontend && npx eslint src/routes/_auth/login.tsx && npm run typecheck:diff && npm run build`
Kết quả mong đợi: 0 vấn đề lint; không file nào tăng lỗi type; build thành công.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/routes/_auth/login.tsx
git commit -m "refactor(auth): move login onto primitives

Leaves the four Google brand hexes in the logo SVG untouched; they are brand
identity, not UI chrome, and the lint config exempts this file for that
reason."
```

---

## Task 17: Nghiệm thu chặng 1

**Files:** không sửa file nào; task này chứng minh chặng 1 đã xong.

- [ ] **Step 1: Bề mặt auth sạch hoàn toàn**

Chạy:
```bash
cd frontend && npx eslint src/routes/_auth.tsx src/routes/_auth/ src/components/auth/ src/components/layouts/
```
Kết quả mong đợi: 0 vấn đề.

- [ ] **Step 2: Số vi phạm toàn repo giảm đúng 32, không tăng ở đâu khác**

Chạy: `cd frontend && npm run lint 2>&1 | grep -c 'no-restricted-syntax'`

So với số nền ghi ở Task 8 Step 5. Hiệu số phải đúng bằng **39**.

Nếu giảm ít hơn 39, còn sót vi phạm. Nếu giảm nhiều hơn 39, có nghĩa đã sửa sang bề mặt khác — vượt phạm vi chặng, cần tách ra commit riêng.

- [ ] **Step 3: Biên dịch và test**

Chạy: `cd frontend && npm run typecheck:diff && npm run build && npm test`
Kết quả mong đợi: không file nào tăng lỗi type; build thành công; toàn bộ test xanh, gồm 29 test primitive mới (9 Button + 7 IconButton + 5 Input + 8 NavRow).

- [ ] **Step 4: Chụp ảnh sau di cư**

Chạy `make dev`, dùng skill `/browse` chụp màn đăng nhập, đăng ký, onboarding, chọn workspace ở 375 / 768 / 1280px. Lưu vào `docs/superpowers/plans/screenshots/2026-08-01-stage1/`.

Đối chiếu với ảnh chặng 0: hình dạng nút phải nhất quán trên cả bốn màn hình, và không màn hình nào vỡ bố cục ở 375px.

Chạy `make dev-stop` khi xong.

- [ ] **Step 5: Ghi lại điều chặng 1 dạy được**

Đây là mục đích tồn tại của chặng thí điểm. Bổ sung một mục vào cuối spec `docs/superpowers/specs/2026-08-01-ui-design-system-enforcement-design.md` trả lời ba câu:

1. Primitive nào thiếu hoặc sai hình dạng khi đụng màn hình thật?
2. Quy tắc "quy giá trị pixel về thang 4px" có chỗ nào không áp được?
3. Có chỗ nào phải dừng và hỏi không, và vì sao?

Câu trả lời quyết định hình dạng kế hoạch cho chặng 2–7.

- [ ] **Step 6: Commit**

```bash
git add docs/superpowers/plans/screenshots/2026-08-01-stage1/ docs/superpowers/specs/2026-08-01-ui-design-system-enforcement-design.md
git commit -m "docs: record what the pilot surface taught

Stage 1 exists to find out where the recalibrated primitives fail against
real screens before the pattern is applied to the remaining 279 violations.
Captures the answers so the stage 2-7 plan is written from evidence."
```

---

## Sau kế hoạch này

Chặng 1 xong thì viết kế hoạch cho chặng 2–7 bằng `superpowers:writing-plans`, dựa trên phần ghi chép ở Task 17 Step 5. Thứ tự và số vi phạm đã có sẵn ở spec §4.4:

| Chặng | Phạm vi | Vi phạm |
|---|---|---:|
| 2 | `components/composites` → `components/patterns` — gồm bug `--md-primary-rgb` ở `ImagePreviewCard.tsx:72` | 82 |
| 3 | `routes/_workspace/` + `routes/_workspace.tsx` | 73 |
| 4 | `components/drive` — giữ số đo pixel kèm chú thích miễn trừ (D6) | 70 |
| 5 | `components/chat` | 36 |
| 6 | `approval` + `assets` + `components/` gốc + `lib/constants.ts` | 44 |
| 7 | Khoá lint `warn` → `error`, cập nhật 3 capability spec | 0 |

`--md-primary-rgb` là biến không tồn tại trong `index.css`, khiến shadow ở `ImagePreviewCard.tsx:72` hiện không render. Nó là bug sống nhưng nằm trong file của chặng 2, nên sửa ở đó thay vì kéo vào chặng 0 — khác với 5 shadow nhuốm màu primary cũ, vốn buộc phải đi cùng đợt đổi token vì chúng vỡ khi token đổi.
