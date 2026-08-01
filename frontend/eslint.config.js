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
    //
    // Không neo vào className, cùng lý do với rule 3: nội dung class có thể được
    // soạn ở nơi khác rồi nội suy vào, và selector neo theo JSXAttribute sẽ không
    // thấy. Đã có hai chỗ như vậy thật — một object variant-map (Modal.tsx) và một
    // hằng template ở cấp module (MessageItem.tsx). Đo thực tế khi bỏ neo: đúng +2
    // cảnh báo, không dương tính giả nào.
    selector: 'Literal[value=/\\[\\d+(px|rem)\\]/]',
    message: 'Giá trị pixel tuỳ tiện. Dùng thang spacing/radius trong index.css.',
  },
  {
    selector: 'TemplateElement[value.raw=/\\[\\d+(px|rem)\\]/]',
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
      "JSXAttribute[name.name='className'] Literal[value=/\\b(bg|text|border)-(slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-\\d{2,3}\\b/]",
    message: 'Bảng màu Tailwind thô. Dùng token M3 hoặc --color-status-*.',
  },
  {
    // Rule 5 — bóng đổ ngoài thang. Rule 2 KHÔNG bắt được nhóm này: nó đòi cả
    // cụm trong ngoặc là \d+(px|rem), nên shadow-[0_4px_16px_rgba(...)] lọt qua
    // dù bên trong có 4px. Thiếu rule này thì 34 shadow tuỳ tiện và 4 shadow-xl
    // vô hình với thước đo tiến độ. Xem spec §4.3.
    // Không neo vào className — xem lý do ở rule 2.
    selector: 'Literal[value=/shadow-(\\[|xl\\b)/]',
    message: 'Bóng đổ ngoài thang. Dùng shadow-sm/md/lg/card/accent* trong index.css.',
  },
  {
    selector: 'TemplateElement[value.raw=/shadow-(\\[|xl\\b)/]',
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
    plugins: { 'react-hooks': reactHooks },
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
